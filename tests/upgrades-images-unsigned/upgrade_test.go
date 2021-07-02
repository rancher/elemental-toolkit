package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Upgrade tests - Images unsigned", func() {
	var s *sut.SUT
	var isVagrant bool

	BeforeSuite(func() {
		isVagrant = sut.IsVagrantTest()
		if isVagrant {
			sut.SnapshotVagrant()
		}
	})

	AfterSuite(func() {
		if isVagrant {
			sut.SnapshotVagrantDelete()
		}
	})

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed == false {
			if isVagrant {
				sut.ResetWithVagrant()
			} else {
				s.Reset()
			}

		}
	})
	Context("After install", func() {
		When("images are not signed", func() {
			It("upgrades to v0.5.7 with --no-verify", func() {
				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out
				out, err = s.Command("cos-upgrade --no-verify --docker-image quay.io/costoolkit/releases-opensuse:cos-system-0.5.7")
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from cos-upgrade: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade done, now you might want to reboot"))
				Expect(out).Should(ContainSubstring("Upgrade target: active.img"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal("0.5.7\n"))

				By("rollbacking state")
				s.Reset()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal("0.5.7\n"))
				Expect(out).To(Equal(version))
			})
		})
	})
})
