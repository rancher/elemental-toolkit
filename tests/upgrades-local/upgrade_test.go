package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

var _ = Describe("cOS Upgrade tests - local upgrades", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			s.GatherAllLogs()
		}
	})

	Context("After install can upgrade and reset", func() {
		When("specifying a directory to upgrade from", func() {
			It("upgrades from the specified path", func() {

				if s.GetArch() == "aarch64" {
					By("Upgrading aarch64 system")
					s.ArtifactsRepo = "quay.io/costoolkit/releases-teal-arm64"
				}

				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out

				out, err = s.Command(fmt.Sprintf("mkdir /run/update && elemental pull-image %s:cos-system-%s /run/update", s.ArtifactsRepo, s.TestVersion))
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from elemental pull-image: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				out, err = s.Command("elemental upgrade --directory /run/update")
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from elemental upgrade: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade completed"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal(fmt.Sprintf("%s\n", s.TestVersion)))

				By("rollbacking state")
				s.Reset()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(fmt.Sprintf("%s\n", s.TestVersion)))
				Expect(out).To(Equal(version))
			})
		})
	})
})
