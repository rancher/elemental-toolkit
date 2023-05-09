package cos_test

import (
	"time"

	sut "github.com/rancher-sandbox/ele-testhelpers/vm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS booting fallback tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})
	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
		if !CurrentSpecReport().Failed() {
			s.Reset()
		}
	})

	Context("COS_PERSISTENT partition is corrupted", func() {
		It("boots in active when the persistent partition is damaged, and can be repaired with fsck", func() {

			// Just to make sure we can match against the same output of blkid later on
			// and that the starting condition is the one we expect
			Eventually(func() string {
				out, _ := s.Command("sudo blkid")
				return out
			}, 1*time.Minute, 10*time.Second).Should(ContainSubstring(`LABEL="COS_PERSISTENT"`))

			persistent, err := s.Command("blkid -L COS_PERSISTENT")
			Expect(err).ToNot(HaveOccurred())

			// This breaks the partition so it can be fixed with fsck
			_, err = s.Command("dd if=/dev/zero count=1 bs=4096 seek=0 of=" + persistent)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				out, _ := s.Command("sudo blkid")
				return out
			}, 5*time.Minute, 10*time.Second).ShouldNot(ContainSubstring(`LABEL="COS_PERSISTENT"`))

			s.Reboot()
			s.EventuallyConnects(700)

			Expect(s.BootFrom()).To(Equal(sut.Active))

			// We should see traces of fsck in the journal.
			// Note, this is a bit ugly because the only messages
			// we have from systemd-fsck is just failed attempts to run.
			// But this is enough for us to assess if it actually kicked in.
			out, err := s.Command("sudo journalctl")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("e2fsck"))
			Expect(out).To(ContainSubstring("Checking inodes"))
			Expect(out).Should(MatchRegexp("COS_PERSISTENT: .* files"))
		})
	})
})
