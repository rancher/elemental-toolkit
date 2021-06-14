package cos_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Upgrade tests - local upgrades", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	Context("After install can upgrade and reset", func() {
		When("specifying a directory to upgrade from", func() {
			It("upgrades from the specified path", func() {
				out, err := s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out

				out, err = s.Command("mkdir /run/update && luet util unpack quay.io/costoolkit/releases-opensuse:cos-system-0.5.1 /run/update")
				if err != nil{
					fmt.Fprintf(GinkgoWriter, "Error from luet util unpack: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				out, err = s.Command("cos-upgrade --no-verify --directory /run/update")
				if err != nil{
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
				Expect(out).To(Equal("0.5.1\n"))

				By("rollbacking state")
				s.Reset()

				out, err = s.Command("source /etc/os-release && echo $VERSION")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal("0.5.1\n"))
				Expect(out).To(Equal(version))
			})
		})
	})
})
