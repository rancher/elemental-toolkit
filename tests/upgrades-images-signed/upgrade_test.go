package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

var _ = Describe("cOS Upgrade tests - Images signed", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
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
	Context("After install", func() {
		When("upgrading", func() {
			It("upgrades to latest available (master) and reset", func() {
				out, err := s.Command(s.ElementalCmd("upgrade"))
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade completed"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))
			})

			It("upgrades to a specific image and reset back to the installed version", func() {

				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				By(fmt.Sprintf("upgrading to an old image: %s:cos-system-%s", s.GetArtifactsRepo(), s.TestVersion))
				out, err = s.Command(s.ElementalCmd("upgrade", "--verify", "--system.uri", fmt.Sprintf("docker:%s:cos-system-%s", s.GetArtifactsRepo(), s.TestVersion)))
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(
					And(
						ContainSubstring("Upgrade completed"),
					),
				)

				By("rebooting and checking out the version")
				s.Reboot()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal(fmt.Sprintf("%s\n", s.TestVersion)))
			})
		})
	})
})
