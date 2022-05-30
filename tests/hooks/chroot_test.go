package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

var _ = Describe("cOS Feature tests", func() {
	var s *sut.SUT
	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	Context("After install", func() {
		It("can run chroot hooks during upgrade and reset", func() {
			err := s.SendFile("../assets/chroot_hooks.yaml", "/oem/chroot_hooks.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			out, err := s.Command("elemental upgrade")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade completed"))
			By("rebooting")
			s.Reboot()
			Expect(s.BootFrom()).To(Equal(sut.Active))

			_, err = s.Command("cat /after-upgrade-chroot")
			Expect(err).ToNot(HaveOccurred())

			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).To(HaveOccurred())

			s.Reset()

			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
