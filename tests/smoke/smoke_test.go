package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

var _ = Describe("cOS Smoke tests", func() {
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
		It("can boot into passive", func() {
			err := s.ChangeBootOnce(sut.Passive)
			Expect(err).ToNot(HaveOccurred())

			By("rebooting into passive")
			s.Reboot()

			Expect(s.BootFrom()).To(Equal(sut.Passive))
			_, err = s.Command("cat /run/cos/recovery_mode")
			Expect(err).To(HaveOccurred())

			_, err = s.Command("cat /run/cos/live_mode")
			Expect(err).To(HaveOccurred())

			By("reboot back to active")
			s.Reboot()
			Expect(s.BootFrom()).To(Equal(sut.Active))
		})

		It("can boot into recovery", func() {
			s.ChangeBoot(sut.Recovery)
			By("rebooting into recovery")
			s.Reboot()

			Expect(s.BootFrom()).To(Equal(sut.Recovery))

			out, err := s.Command("cat /run/cos/recovery_mode")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal("1"))
			_, err = s.Command("cat /run/cos/live_mode")
			Expect(err).To(HaveOccurred())

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

		It("fails running elemental reset from COS_ACTIVE", func() {
			out, err := s.Command("elemental reset")
			Expect(err).To(HaveOccurred())
			Expect(out).Should(ContainSubstring("reset can only be called from the recovery system"))
		})
	})

	Context("Settings", func() {
		It("has correct defaults", func() {
			out, err := s.Command("cat /etc/elemental/config.yaml | grep system/cos")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("uri: channel:system/cos"))

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
