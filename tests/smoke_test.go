package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS", func() {

	Context("Settings", func() {
		It("connects", func() {
			eventuallyConnects(10)
		})

		It("has proper settings", func() {
			out, err := sshCommand("source /etc/cos-upgrade-image && echo $UPGRADE_IMAGE")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(Equal("system/cos\n"))
		})
	})

	Context("Upgrades", func() {
		It("connects", func() {
			eventuallyConnects(10)
		})

		It("upgrades to latest available (master)", func() {
			out, _ := sshCommand("cos-upgrade && reboot")
			Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
			fmt.Println(out)

			eventuallyConnects(120)
		})

		It("upgrades to a specific image", func() {
			out, err := sshCommand("/bin/bash -c 'source /etc/os-release && echo $VERSION'")
			Expect(out).ToNot(Equal(""))
			fmt.Println(out)
			Expect(err).ToNot(HaveOccurred())

			version := out

			out, _ = sshCommand("cos-upgrade --docker-image raccos/releases-amd64:cos-system-0.4.16 && reboot")
			Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
			fmt.Println(out)

			eventuallyConnects(120)

			out, err = sshCommand("source /etc/os-release && echo $VERSION")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).ToNot(Equal(""))
			Expect(out).ToNot(Equal(version))
			Expect(out).To(Equal("0.4.16"))
		})
	})
})
