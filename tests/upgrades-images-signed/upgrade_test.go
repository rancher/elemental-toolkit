package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Upgrade tests - Images signed", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	AfterEach(func() {
		// Try to gather mtree logs on failure
		if CurrentGinkgoTestDescription().Failed {
			s.GatherLog("/tmp/image-mtree-check.log")
			s.GatherLog("/tmp/luet_mtree_failures.log")
			s.GatherLog("/tmp/luet_mtree.log")
		}
		s.Reset()
	})
	Context("After install", func() {
		When("upgrading", func() {
			It("upgrades to latest available (master) and reset", func() {
				out, err := s.Command("cos-upgrade")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))
			})
			It("upgrades to a specific image and reset back to the installed version", func() {
				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				By("upgrading to an old image")
				out, err = s.Command("cos-upgrade --docker-image quay.io/costoolkit/releases-opensuse:cos-system-0.5.1")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("to /usr/local/tmp/rootfs"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))

				By("rebooting and checking out the version")
				s.Reboot()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal("0.5.1\n"))
			})
			It("fails if verify is enabled on an unsigned/malformed version", func() {
				out, err := s.Command("cos-upgrade --docker-image raccos/releases-opensuse:cos-system-0.5.0")
				Expect(err).To(HaveOccurred())
				Expect(out).Should(ContainSubstring("image-mtree-check"))
				Expect(out).Should(ContainSubstring("error while executing plugin"))
				out, err = s.Command("cat /tmp/image-mtree-check.log")
				Expect(out).Should(ContainSubstring("Got cos-system-0.5.0, continue..."))
				Expect(out).Should(ContainSubstring("Finished all checks with errors"))

			})
		})
	})
})
