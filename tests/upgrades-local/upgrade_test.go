package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
	comm "github.com/rancher/elemental-toolkit/tests/common"
)

var _ = Describe("cOS Upgrade tests - local upgrades", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
	})

	Context("After install can upgrade and reset", func() {
		When("specifying a directory to upgrade from", func() {
			It("upgrades from the specified path", func() {

				out, err := s.Command("source /etc/os-release && echo $TIMESTAMP")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				version := out

				out, err = s.Command(fmt.Sprintf("mkdir /run/update"))
				out, err = s.Command(s.ElementalCmd("pull-image", comm.UpgradeImage(), "/run/update"))
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from elemental pull-image: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))

				out, err = s.Command(s.ElementalCmd("upgrade", "--directory", "/run/update"))
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error from elemental upgrade: %v\n", err)
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Upgrade completed"))
				By("rebooting")
				s.Reboot()
				Expect(s.BootFrom()).To(Equal(sut.Active))

				out, err = s.Command("source /etc/os-release && echo $TIMESTAMP")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).ToNot(Equal(version))

				By("rollbacking state")
				s.Reset()

				out, err = s.Command("source /etc/os-release && echo $TIMESTAMP")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).ToNot(Equal(""))
				Expect(out).To(Equal(version))
			})
		})
	})
})
