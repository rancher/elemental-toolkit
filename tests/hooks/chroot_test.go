package cos_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

// TODO this it just for first round of tests, at some point we should use
// an image base on a build for a tagged main commit.
const upgradeImg = "ghcr.io/davidcassany/elemental-green:v0.10.1-29-g9ab23ba5"

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

			out, err := s.Command(s.ElementalCmd("upgrade", "--system.uri", upgradeImg))
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
