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
				grubEntry, err := s.Command("grub2-editenv /run/boot/grub_oem_env list | grep default_menu_entry= | sed 's/default_menu_entry=//'")
				Expect(err).ToNot(HaveOccurred())

				out, err := s.Command("cos-upgrade")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))
				By("checking grub menu entry changes", func() {
					newGrubEntry, err := s.Command("grub2-editenv /run/boot/grub_oem_env list | grep default_menu_entry= | sed 's/default_menu_entry=//'")
					Expect(err).ToNot(HaveOccurred())
					Expect(grubEntry).ToNot(Equal(newGrubEntry))
				})
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
				out, err = s.Command(fmt.Sprintf("cos-upgrade --docker-image %s:cos-system-%s", s.GreenRepo, s.TestVersion))
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("to /usr/local/.cos-upgrade/tmp/rootfs"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))

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
