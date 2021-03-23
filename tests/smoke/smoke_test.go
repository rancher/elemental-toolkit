package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Smoke tests", func() {
	var s *sut.SUT
	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	Context("After install", func() {
		It("can boot into passive", func() {
			s.ChangeBoot(sut.Passive)
			By("rebooting into passive")
			s.Reboot()

			Expect(s.BootFrom()).To(Equal(sut.Passive))

			By("switching back to active")
			s.ChangeBoot(sut.Active)
			s.Reboot()
			Expect(s.BootFrom()).To(Equal(sut.Active))
		})

		It("can boot into recovery", func() {
			s.ChangeBoot(sut.Recovery)
			By("rebooting into recovery")
			s.Reboot()

			Expect(s.BootFrom()).To(Equal(sut.Recovery))

			By("switching back to active")
			s.ChangeBoot(sut.Active)
			s.Reboot()
			Expect(s.BootFrom()).To(Equal(sut.Active))
		})

		It("is booting from COS_ACTIVE", func() {
			out, err := s.Command("blkid -L COS_ACTIVE")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("/dev/loop0"))
		})

		It("fails running cos-reset from COS_ACTIVE", func() {
			out, err := s.Command("cos-reset")
			Expect(err).To(HaveOccurred())
			Expect(out).Should(ContainSubstring("cos-reset can be run only from recovery"))
		})
	})

	Context("Settings", func() {
		It("has correct defaults", func() {
			out, err := s.Command("source /etc/cos-upgrade-image && echo $UPGRADE_IMAGE")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(Equal("system/cos\n"))

			out, err = s.Command("source /etc/os-release && echo $PRETTY_NAME")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("cOS"))
		})

		It("has default date in UTC format from cloud-init", func() {
			out, err := s.Command("date")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("UTC"))
		})

		It("has default localectl configuration from cloud-init", func() {
			out, err := s.Command("localectl status")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("LANG=en_US.UTF-8"))
			Expect(out).Should(ContainSubstring("VC Keymap: us"))
		})

		It("is booting from active partition", func() {
			Expect(s.BootFrom()).To(Equal(sut.Active))
		})
	})
})
