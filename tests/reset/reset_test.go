package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
	comm "github.com/rancher/elemental-toolkit/tests/common"
)

var _ = Describe("Elemental reset tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	AfterEach(func() {
		// Try to gather mtree logs on failure
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
		if !CurrentSpecReport().Failed() {
			s.Reset()
		}
	})

	Context("From recovery", func() {
		When("deploying again", func() {
			It("deploys the system", func() {

				err := s.ChangeBootOnce(sut.Recovery)
				Expect(err).ToNot(HaveOccurred())
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Recovery))

				out := s.GetOSRelease("TIMESTAMP")
				Expect(out).ToNot(Equal(""))
				By(fmt.Sprintf("Starting from version %s", out))

				version := out
				cmd := s.ElementalCmd("reset", "--system.uri", comm.UpgradeImage())
				By(fmt.Sprintf("Runnning %s", cmd))
				_, err = s.Command(cmd)
				Expect(err).NotTo(HaveOccurred())
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))

				out = s.GetOSRelease("TIMESTAMP")
				Expect(out).ToNot(Equal(""))

				Expect(out).ToNot(Equal(version))
			})
		})
	})
})
