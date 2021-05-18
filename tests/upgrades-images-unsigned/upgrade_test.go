package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Upgrade tests - Images unsigned", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	AfterEach(func() {
		s.Reset()
	})
	Context("After install", func() {
		When("images are not signed", func() {
			It("upgrades to latest available (master) and reset", func() {
				out, err := s.Command("cos-upgrade")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Booting from: active.img"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))
			})
			It("upgrades on an unsigned upgrade channel", func() {

				By("pointing to an old release branch")
				out, err := s.Command("sed -i 's|raccos/releases-.*|raccos/releases-amd64\"|g' /etc/luet/luet.yaml && cos-upgrade")
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(err).ToNot(HaveOccurred())

				// That version is very old and incompatible. It ships oem files inside /oem, that overrides configuration now shipped in
				// /system/cos. Mainly, they override the /etc/cos-upgrade-image file to an incompatible format
				out, err = s.Command("rm -rfv /oem/*_*.yaml")
				Expect(out).Should(ContainSubstring("removed"))
				Expect(err).ToNot(HaveOccurred())

			})
		})
	})
})
