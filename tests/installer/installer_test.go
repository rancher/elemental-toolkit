package cos_test

import (
	"github.com/rancher-sandbox/cOS/tests/sut"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS Installer tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			s.GatherAllLogs()
		} else {
			// Trash disk so we boot from iso after each test
			s.EmptyDisk("/dev/sda")
			// Reboot so it boots from iso
			s.Reboot()
		}
	})
	Context("Using legacy bios", func() {
		Context("generic tests", func() {
			It("booted from iso", func() {
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.LiveCD))
				out, err := s.Command("grep /dev/sr /etc/mtab")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).To(ContainSubstring("iso9660"))
				out, err = s.Command("df -h /")
				Expect(err).ToNot(HaveOccurred())
				Expect(out).To(ContainSubstring("LiveOS_rootfs"))
			})
		})
		Context("source tests", func() {
			It("from iso", func() {
				out, err := s.Command("cos-installer /dev/sda")
				Expect(err).To(BeNil())
				Expect(out).To(ContainSubstring("Copying cOS.."))
				Expect(out).To(ContainSubstring("Installing GRUB.."))
				Expect(out).To(ContainSubstring("Preparing recovery.."))
				Expect(out).To(ContainSubstring("Preparing passive boot.."))
				Expect(out).To(ContainSubstring("Formatting drives.."))
				// Reboot so we boot into the just installed cos
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
			})
			PIt("from url", func() {})
		})
		PContext("partition layout tests", func() {
			It("with partition layout", func() {})
		})
		PContext("efi/gpt tests", func() {
			It("forces gpt", func() {})
			It("forces efi", func() {})
		})
		PContext("config file tests", func() {
			It("uses a proper config file", func() {})
			It("uses a wrong config file", func() {})
		})
	})

})
