package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
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
			It("upgrades with --no-verify", func() {

				if s.GetArch() == "aarch64" {
					By("Upgrading aarch64 system")
					s.GreenRepo = "quay.io/costoolkit/releases-green-arm64"
				}

				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				out, err = s.Command(fmt.Sprintf("cos-upgrade --no-verify --docker-image %s:cos-system-%s", s.GreenRepo, s.TestVersion))
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from cos-upgrade: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))
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

			// NOTE: once https://github.com/rancher-sandbox/cOS-toolkit/pull/959 is merged,
			// the image used in the test below here is not relevant anymore and this test
			// can be run aside of the upgrade-images-signed (which upgrades against master)
			// What we are checking here is that the grub menu file is getting updated after
			// running a cos-upgrade from the content of the target image.
			It("changes grub menu entry during upgrades", func() {

				if s.GetArch() == "aarch64" {
					Skip("test valid only on amd64, until https://github.com/rancher-sandbox/cOS-toolkit/pull/959 is merged")
				}

				grubEntry, err := s.Command("grub2-editenv /run/initramfs/cos-state/grub_oem_env list | grep default_menu_entry= | sed 's/default_menu_entry=//'")
				Expect(err).ToNot(HaveOccurred())

				out, err := s.Command("cos-upgrade --no-cosign --no-verify --docker-image quay.io/costoolkit/test-images:grub_menu")
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from cos-upgrade: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))

				By("checking grub menu entry changes", func() {
					newGrubEntry, err := s.Command("grub2-editenv /run/initramfs/cos-state/grub_oem_env list | grep default_menu_entry= | sed 's/default_menu_entry=//'")
					Expect(err).ToNot(HaveOccurred())
					Expect(grubEntry).ToNot(Equal(newGrubEntry))
				})

				By("rollbacking state")
				s.Reset()
			})
		})
	})
})
