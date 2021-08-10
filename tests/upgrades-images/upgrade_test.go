package cos_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Upgrade tests - Images signed", func() {
	var s *sut.SUT
	testImage := "quay.io/costoolkit/releases-green:cos-system-"
	testVersion := "0.6.2"

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
			s.GatherLog("/tmp/luet.log")
		}
		if CurrentGinkgoTestDescription().Failed == false {
			s.Reset()
		}
	})
	Context("After install", func() {
		When("verifying images", func() {
			It("upgrades to latest available (master)", func() {
				out, err := s.Command("cos-upgrade")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))
				By("rebooting and checking out the version")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))
			})
			It(fmt.Sprintf("upgrades to %s with verify", testVersion), func() {
				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				By("upgrading to an old image")
				out, err = s.Command(fmt.Sprintf("cos-upgrade --docker-image %s%s", testImage, testVersion))
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("to /usr/local/.cos-upgrade/tmp/rootfs"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))

				By("rebooting and checking out the version")
				s.Reboot()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal(fmt.Sprintf("%s\n", testVersion)))
			})
		})
		When("not verifying images", func() {
			It(fmt.Sprintf("upgrades to %s with --no-verify", testVersion), func() {
				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				out, err = s.Command(fmt.Sprintf("cos-upgrade --no-verify --docker-image %s%s", testImage, testVersion))
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from cos-upgrade: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))
				By("rebooting and checking out the version")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal(fmt.Sprintf("%s\n", testVersion)))

				// This should probably go into its own test, to check if we rollback properly
				By("rollbacking state")
				s.Reset()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(fmt.Sprintf("%s\n", testVersion)))
				Expect(out).To(Equal(version))
			})
		})
	})
})
