package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS Upgrade tests", func() {
	var s *SUT
	BeforeEach(func() {
		s = NewSUT("", "", "")
		s.EventuallyConnects()
	})

	Context("After install", func() {

		When("images are not signed", func() {
			It("fails to upgrade to a version which is not signed", func() {
				out, err := s.Command("cos-upgrade --docker-image raccos/releases-opensuse:cos-system-0.4.31")
				Expect(err).To(HaveOccurred())
				Expect(out).Should(ContainSubstring("No valid trust data"))
			})

			It("fails to upgrade if verify is enabled on an unsigned upgrade channel", func() {
				out, err := s.Command("sed -i 's|raccos/releases-.*|raccos/releases-amd64\"|g' /etc/luet/luet.yaml && cos-upgrade")
				Expect(out).Should(ContainSubstring("does not have trust data"))
				Expect(err).To(HaveOccurred())
				s.Reset()
			})

			It("upgrades if verify is disabled on an unsigned upgrade channel", func() {
				out, err := s.Command("sed -i 's|raccos/releases-.*|raccos/releases-amd64\"|g' /etc/luet/luet.yaml && sed -i 's/verify: true/verify: false/g' /etc/luet/luet.yaml && cos-upgrade")
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(err).ToNot(HaveOccurred())
				s.Reset()

				// That version is very old and incompatible. It ships oem files inside /oem, that overrides configuration now shipped in
				// /system/cos. Mainly, they override the /etc/cos-upgrade-image file to an incompatible format
				out, err = s.Command("rm -rfv /oem/*_*.yaml")
				Expect(out).Should(ContainSubstring("removed"))
				Expect(err).ToNot(HaveOccurred())
				s.Reboot()
			})

			It("upgrades to an unsigned image with --no-verify and can reset back to the installed state", func() {
				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out

				out, err = s.Command("cos-upgrade --no-verify --docker-image raccos/releases-opensuse:cos-system-0.4.31")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("to /usr/local/tmp/rootfs"))
				Expect(out).Should(ContainSubstring("Booting from: active.img"))

				s.Reboot()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal("0.4.31\n"))

				s.Reset()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal("0.4.31\n"))
				Expect(out).To(Equal(version))
			})
		})

		When("images are signed", func() {
			It("upgrades to latest available (master) and reset", func() {
				out, err := s.Command("cos-upgrade")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Booting from: active.img"))

				s.Reboot()

				s.Reset()
			})

			It("upgrades to a specific image and reset back to the installed version", func() {
				// out, err := s.Command("source /etc/os-release && echo $VERSION", false)
				// Expect(err).ToNot(HaveOccurred())
				// Expect(out).ToNot(Equal(""))

				//	version := out

				out, err := s.Command("cos-upgrade --docker-image raccos/releases-opensuse:cos-system-0.4.32")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("to /usr/local/tmp/rootfs"))
				Expect(out).Should(ContainSubstring("Booting from: active.img"))

				s.Reboot()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				//	Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal("0.4.32\n"))

				s.Reset()
			})
		})
	})
})
