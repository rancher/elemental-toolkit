package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS", func() {
	BeforeEach(func() {
		eventuallyConnects()
	})

	Context("Settings", func() {
		It("has proper settings", func() {
			out, err := sshCommand("source /etc/cos-upgrade-image && echo $UPGRADE_IMAGE")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(Equal("system/cos\n"))
		})
	})

	Context("After install", func() {
		It("upgrades to latest available (master)", func() {
			out, _ := sshCommand("cos-upgrade && reboot")
			Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
			Expect(out).Should(ContainSubstring("Booting from: active.img"))

			eventuallyConnects()
		})

		It("upgrades to a specific image", func() {
			out, err := sshCommand("source /etc/os-release && echo $VERSION")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).ToNot(Equal(""))
			version := out

			out, _ = sshCommand("cos-upgrade --docker-image raccos/releases-amd64:cos-system-0.4.16 && reboot")
			Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
			Expect(out).Should(ContainSubstring("to /usr/local/tmp/rootfs"))
			Expect(out).Should(ContainSubstring("Booting from: active.img"))

			eventuallyConnects()

			out, err = sshCommand("source /etc/os-release && echo $VERSION")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).ToNot(Equal(""))
			Expect(out).ToNot(Equal(version))
			Expect(out).To(Equal("0.4.16\n"))
		})
	})
})
