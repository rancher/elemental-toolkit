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
		}
	})
	Context("Using bios", func() {
		BeforeEach(func() {
			s.EmptyDisk("/dev/sda")
			By("Reboot to make sure we boot from CD")
			s.Reboot()
			// Assert we are booting from CD before running the tests
			By("Making sure we booted from CD")
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.LiveCD))
			out, err := s.Command("grep /dev/sr /etc/mtab")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("iso9660"))
			out, err = s.Command("df -h /")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("LiveOS_rootfs"))
		})
		Context("install source tests", func() {
			It("from iso", func() {
				By("Running the cos-installer")
				out, err := s.Command("cos-installer /dev/sda")
				Expect(err).To(BeNil())
				Expect(out).To(ContainSubstring("Copying cOS.."))
				Expect(out).To(ContainSubstring("Installing GRUB.."))
				Expect(out).To(ContainSubstring("Preparing recovery.."))
				Expect(out).To(ContainSubstring("Preparing passive boot.."))
				Expect(out).To(ContainSubstring("Formatting drives.."))
				// Reboot so we boot into the just installed cos
				s.Reboot()
				By("Checking we booted from the installed cOS")
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
	Context("Using efi", func() {
		// EFI variant tests is not able to set the boot order and there is no plan to let the user do so according to virtualbox
		// So we need to manually eject/insert the cd to force the boot order. CD inserted -> boot from cd, CD empty -> boot from disk
		BeforeEach(func() {
			// Store COS iso path
			s.SetCOSCDLocation()
			// Set Cos CD in the drive
			s.RestoreCOSCD()
			s.EmptyDisk("/dev/sda")
			By("Reboot to make sure we boot from CD")
			s.Reboot()
			// Assert we are booting from CD before running the tests
			By("Making sure we booted from CD")
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.LiveCD))
			out, err := s.Command("grep /dev/sr /etc/mtab")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("iso9660"))
			out, err = s.Command("df -h /")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("LiveOS_rootfs"))
		})
		Context("install source tests", func() {
			It("from iso", func() {
				By("Running the cos-installer")
				out, err := s.Command("cos-installer /dev/sda")
				Expect(err).To(BeNil())
				Expect(out).To(ContainSubstring("Copying cOS.."))
				Expect(out).To(ContainSubstring("Installing GRUB.."))
				Expect(out).To(ContainSubstring("Preparing recovery.."))
				Expect(out).To(ContainSubstring("Preparing passive boot.."))
				Expect(out).To(ContainSubstring("Formatting drives.."))
				// Remove iso so we boot directly from the disk
				s.EjectCOSCD()
				// Reboot so we boot into the just installed cos
				s.Reboot()
				By("Checking we booted from the installed cOS")
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
