package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
	comm "github.com/rancher/elemental-toolkit/tests/common"
)

var _ = Describe("Elemental Feature tests", func() {
	var s *sut.SUT
	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
		Expect(s.BootFrom()).To(Equal(sut.Active))
	})

	Context("After install", func() {
		It("upgrades to a signed image including upgrade and reset hooks", func() {
			By("setting /oem/chroot_hooks.yaml")
			err := s.SendFile("../assets/chroot_hooks.yaml", "/oem/chroot_hooks.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())
			originalVersion := s.GetOSRelease("TIMESTAMP")

			By(fmt.Sprintf("and upgrading the to %s", comm.UpgradeImage()))
			out, err := s.Command(s.ElementalCmd("upgrade", "--verify", "--system.uri", comm.UpgradeImage()))
			Expect(err).To(HaveOccurred())
			out, err = s.Command(s.ElementalCmd("upgrade", "--system.uri", comm.UpgradeImage()))
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade completed"))

			s.Reboot()
			Expect(s.BootFrom()).To(Equal(sut.Active))
			currentVersion := s.GetOSRelease("TIMESTAMP")
			Expect(currentVersion).NotTo(Equal(originalVersion))

			_, err = s.Command("cat /after-upgrade-chroot")
			Expect(err).ToNot(HaveOccurred())

			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).To(HaveOccurred())

			s.Reset()
			currentVersion = s.GetOSRelease("TIMESTAMP")
			Expect(currentVersion).To(Equal(originalVersion))
			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
