package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS", func() {
	var s *SUT
	BeforeEach(func() {
		s = NewSUT("", "", "")
		s.EventuallyConnects()
	})

	Context("Settings", func() {
		It("has correct defaults", func() {
			out, err := s.Command("source /etc/cos-upgrade-image && echo $UPGRADE_IMAGE", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(Equal("system/cos\n"))

			out, err = s.Command("source /etc/os-release && echo $PRETTY_NAME", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("cOS"))
		})
	})

	Context("After install", func() {
		It("upgrades to latest available (master)", func() {
			out, err := s.Command("cos-upgrade", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
			Expect(out).Should(ContainSubstring("Booting from: active.img"))

			s.Reboot()

			s.EventuallyConnects()
		})

		It("upgrades to a specific image", func() {
			out, err := s.Command("source /etc/os-release && echo $VERSION", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).ToNot(Equal(""))
			version := out

			out, err = s.Command("cos-upgrade --docker-image raccos/releases-amd64:cos-system-0.4.16", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
			Expect(out).Should(ContainSubstring("to /usr/local/tmp/rootfs"))
			Expect(out).Should(ContainSubstring("Booting from: active.img"))

			s.Reboot()

			s.EventuallyConnects()

			out, err = s.Command("source /etc/os-release && echo $VERSION", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).ToNot(Equal(""))
			Expect(out).ToNot(Equal(version))
			Expect(out).To(Equal("0.4.16\n"))
		})
	})
})
