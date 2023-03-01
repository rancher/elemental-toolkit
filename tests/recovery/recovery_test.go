package cos_test

import (
	"fmt"

	sut "github.com/rancher-sandbox/ele-testhelpers/vm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS Recovery upgrade tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
	})

	Context("upgrading COS_ACTIVE from the recovery partition", func() {
		AfterEach(func() {
			if !CurrentSpecReport().Failed() {
				// Get the label filter
				a, _ := GinkgoConfiguration()
				// if no label was set, we are running the whole suite, so do the reset
				// Otherwise we are only running one test, no need to reset the vm afterwards, saves time
				if a.LabelFilter == "" {
					s.Reset()
				}
			}
		})

		It("upgrades to a specific image", Label("second-test"), func() {
			err := s.ChangeBoot(sut.Active)
			Expect(err).ToNot(HaveOccurred())

			s.Reboot()
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
			currentVersion := s.GetOSRelease("VERSION")

			By("booting into recovery to check the OS version")
			err = s.ChangeBoot(sut.Recovery)
			Expect(err).ToNot(HaveOccurred())

			s.Reboot()
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Recovery))
			By(fmt.Sprintf("upgrading to %s", s.GetSystemURIDocker()))
			cmd := s.ElementalCmd("upgrade", "--system.uri", s.GetSystemURIDocker())
			By(fmt.Sprintf("running %s", cmd))
			out, err := s.Command(cmd)
			_, _ = fmt.Fprintln(GinkgoWriter, out)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade completed"))
			fmt.Fprint(GinkgoWriter, out)
			err = s.ChangeBoot(sut.Active)
			Expect(err).ToNot(HaveOccurred())

			s.Reboot()
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))

			upgradedVersion := s.GetOSRelease("VERSION")
			Expect(upgradedVersion).ToNot(Equal(currentVersion))
			Expect(upgradedVersion).To(Equal(s.TestVersion))
		})
	})

	// After this test, the VM is no longer in its initial state!!
	Context("upgrading recovery", func() {
		When("using specific images", func() {
			It("upgrades to a specific image and reset back to the installed version", Label("third-test"), func() {

				version := s.GetOSRelease("VERSION")
				By(fmt.Sprintf("upgrading to %s", s.GetRecoveryURIDocker()))
				cmd := s.ElementalCmd("upgrade", "--recovery", "--recovery-system.uri", s.GetRecoveryURIDocker(), "--squash-no-compression")
				By(fmt.Sprintf("running %s", cmd))
				out, err := s.Command(cmd)
				_, _ = fmt.Fprintln(GinkgoWriter, out)
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade completed"))

				By("booting into recovery to check the OS version")
				err = s.ChangeBootOnce(sut.Recovery)
				Expect(err).ToNot(HaveOccurred())

				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Recovery))

				out = s.GetOSRelease("VERSION")
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal(s.TestVersion))

				By("rebooting back to active")
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
			})
		})
	})
})
