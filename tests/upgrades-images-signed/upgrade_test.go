package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Upgrade tests - Images signed", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	AfterEach(func() {
		// Try to gather mtree logs on failure
		if CurrentGinkgoTestDescription().Failed {
			s.GatherAllLogs()
		}
		if CurrentGinkgoTestDescription().Failed == false {
			s.Reset()
		}
	})
	Context("After install", func() {
		When("upgrading", func() {
			It("upgrades to latest available (master) and reset", func() {
				out, err := s.Command("elemental --debug upgrade")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade completed"))
				Expect(out).Should(ContainSubstring("Upgrading active partition"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))
			})

			It("upgrades to a specific image and reset back to the installed version", func() {

				if s.GetArch() == "aarch64" {
					By("Upgrading aarch64 system")
					s.GreenRepo = "quay.io/costoolkit/releases-green-arm64"
				}

				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				By(fmt.Sprintf("upgrading to an old image: %s:cos-system-%s", s.GreenRepo, s.TestVersion))
				out, err = s.Command(fmt.Sprintf("elemental --debug upgrade --docker-image %s:cos-system-%s", s.GreenRepo, s.TestVersion))
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(
					And(
						ContainSubstring("Upgrade completed"),
						ContainSubstring("Upgrading active partition"),
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
