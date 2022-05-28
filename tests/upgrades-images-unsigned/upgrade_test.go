package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

var _ = Describe("cOS Upgrade tests - Images unsigned", func() {
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
	Context("After install", func() {
		When("images are not signed", func() {
			It("upgrades", func() {

				if s.GetArch() == "aarch64" {
					By("Upgrading aarch64 system")
					s.ArtifactsRepo = "quay.io/costoolkit/releases-teal-arm64"
				}

				grubEntry, err := s.Command("grub2-editenv /run/initramfs/cos-state/grub_oem_env list | grep default_menu_entry= | sed 's/default_menu_entry=//'")
				Expect(err).ToNot(HaveOccurred())

				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				out, err = s.Command(fmt.Sprintf("elemental upgrade --system.uri docker:%s:cos-system-%s", s.ArtifactsRepo, s.TestVersion))
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).Should(ContainSubstring("Upgrade completed"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal(fmt.Sprintf("%s\n", s.TestVersion)))

				By("checking grub menu entry changes", func() {
					newGrubEntry, err := s.Command("grub2-editenv /run/initramfs/cos-state/grub_oem_env list | grep default_menu_entry= | sed 's/default_menu_entry=//'")
					Expect(err).ToNot(HaveOccurred())
					Expect(grubEntry).ToNot(Equal(newGrubEntry))
				})

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
