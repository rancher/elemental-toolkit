package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

var _ = Describe("Elemental Installer EFI tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	Context("Using efi", func() {
		// EFI variant tests is not able to set the boot order and there is no plan to let the user do so according to virtualbox
		// So we need to manually eject/insert the cd to force the boot order. CD inserted -> boot from cd, CD empty -> boot from disk
		BeforeEach(func() {
			// Assert we are booting from CD before running the tests
			By("Making sure we booted from CD")
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.LiveCD))
			s.EmptyDisk("/dev/sda")
			_, _ = s.Command("sync")
		})
		AfterEach(func() {
			if CurrentSpecReport().Failed() {
				s.GatherAllLogs()
			}
		})

		Context("partition layout tests", func() {
			Context("with partition layout", func() {
				It("performs a standard install", func() {
					err := s.SendFile("../assets/custom_partitions.yaml", "/etc/elemental/config.d/custom_partitions.yaml", "0770")
					By("Running the elemental install with a layout file")
					Expect(err).To(BeNil())
					out, err := s.Command(s.ElementalCmd("install", "--part-table gpt", "/dev/sda"))
					fmt.Printf(out)
					Expect(err).To(BeNil())
					Expect(out).To(ContainSubstring("Mounting disk partitions"))
					Expect(out).To(ContainSubstring("Partitioning device..."))
					Expect(out).To(ContainSubstring("Running after-install hook"))
				})

				// This section of the test is flaky in our CI w/EFI. Commenting it out for the time being
				PIt("Forcing GPT", func() {
					err := s.SendFile("../assets/custom_partitions.yaml", "/etc/elemental/config.d/custom_partitions.yaml", "0770")
					By("Running the elemental install with a layout file")
					Expect(err).To(BeNil())
					out, err := s.Command(s.ElementalCmd("install", "--force-gpt", "/dev/sda"))
					fmt.Printf(out)
					Expect(err).To(BeNil())
					Expect(out).To(ContainSubstring("Installing GRUB.."))
					Expect(out).To(ContainSubstring("Mounting disk partitions"))
					Expect(out).To(ContainSubstring("Partitioning device..."))
					Expect(out).To(ContainSubstring("Running after-install hook"))

					// Remove iso so we boot directly from the disk
					s.EjectCD()
					// Reboot so we boot into the just installed cos
					s.Reboot()
					By("Checking we booted from the installed cOS")
					ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
					// check partition values
					// Values have to match the yaml under ../assets/layout.yaml
					// That is the file that the installer uses so partitions should match those values
					disk := s.GetDiskLayout("/dev/sda")

					for _, part := range []sut.PartitionEntry{
						{
							Label:  "COS_STATE",
							Size:   6144,
							FsType: sut.Ext4,
						},
						{
							Label:  "COS_OEM",
							Size:   128,
							FsType: sut.Ext4,
						},
						{
							Label:  "COS_RECOVERY",
							Size:   4096,
							FsType: sut.Ext2,
						},
						{
							Label:  "COS_PERSISTENT",
							Size:   8192,
							FsType: sut.Ext2,
						},
					} {
						CheckPartitionValues(disk, part)
					}
				})
				// Marked as pending to reduce the number of efi tests. VBox efi support is
				// not good enough to run extensive tests
				PIt("Not forcing GPT", func() {
					err := s.SendFile("../assets/custom_partitions.yaml", "/etc/elemental/config.d/custom_partitions.yaml", "0770")
					By("Running the elemental install with a layout file")
					Expect(err).To(BeNil())
					out, err := s.Command(s.ElementalCmd("install", "/dev/sda"))
					Expect(err).To(BeNil())
					Expect(out).To(ContainSubstring("Installing GRUB.."))
					Expect(out).To(ContainSubstring("Mounting disk partitions"))
					Expect(out).To(ContainSubstring("Partitioning device..."))
					Expect(out).To(ContainSubstring("Running after-install hook"))
					// Remove iso so we boot directly from the disk
					s.EjectCD()
					// Reboot so we boot into the just installed cos
					s.Reboot()
					By("Checking we booted from the installed cOS")
					ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
					// check partition values
					// Values have to match the yaml under ../assets/layout.yaml
					// That is the file that the installer uses so partitions should match those values
					disk := s.GetDiskLayout("/dev/sda")

					for _, part := range []sut.PartitionEntry{
						{
							Label:  "COS_STATE",
							Size:   8192,
							FsType: sut.Ext4,
						},
						{
							Label:  "COS_OEM",
							Size:   10,
							FsType: sut.Ext4,
						},
						{
							Label:  "COS_RECOVERY",
							Size:   4000,
							FsType: sut.Ext2,
						},
						{
							Label:  "COS_PERSISTENT",
							Size:   100,
							FsType: sut.Ext2,
						},
					} {
						CheckPartitionValues(disk, part)
					}
				})
			})
		})
		// Marked as pending to reduce the number of efi tests. VBox efi support is
		// not good enough to run extensive tests
		PContext("install source tests", func() {
			It("from iso", func() {
				By("Running the elemental install")
				out, err := s.Command(s.ElementalCmd("install", "/dev/sda"))
				Expect(err).To(BeNil())
				Expect(out).To(ContainSubstring("Installing GRUB.."))
				Expect(out).To(ContainSubstring("Mounting disk partitions"))
				Expect(out).To(ContainSubstring("Partitioning device..."))
				Expect(out).To(ContainSubstring("Running after-install hook"))
				// Remove iso so we boot directly from the disk
				s.EjectCD()
				// Reboot so we boot into the just installed cos
				s.Reboot()
				By("Checking we booted from the installed cOS")
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
			})
			PIt("from url", func() {})
			It("from docker image", func() {
				By("Running the elemental install")
				out, err := s.Command(s.ElementalCmd("install", "--system.uri", fmt.Sprintf("docker:%s:cos-system-%s", s.GetArtifactsRepo(), s.TestVersion), "/dev/sda"))
				Expect(err).To(BeNil())
				Expect(out).To(ContainSubstring("Installing GRUB.."))
				Expect(out).To(ContainSubstring("Mounting disk partitions"))
				Expect(out).To(ContainSubstring("Partitioning device..."))
				Expect(out).To(ContainSubstring("Running after-install hook"))
				s.EjectCD()
				// Reboot so we boot into the just installed cos
				s.Reboot()
				By("Checking we booted from the installed cOS")
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
				Expect(s.GetOSRelease("VERSION")).To(Equal(s.TestVersion))
			})
		})
		// Marked as pending to reduce the number of efi tests. VBox efi support is
		// not good enough to run extensive tests
		PContext("efi/gpt tests", func() {
			It("forces gpt", func() {
				By("Running the installer")
				out, err := s.Command(s.ElementalCmd("install", "--force-gpt", "/dev/sda"))
				Expect(err).To(BeNil())
				Expect(out).To(ContainSubstring("Installing GRUB.."))
				Expect(out).To(ContainSubstring("Mounting disk partitions"))
				Expect(out).To(ContainSubstring("Partitioning device..."))
				Expect(out).To(ContainSubstring("Running after-install hook"))
				// Remove iso so we boot directly from the disk
				s.EjectCD()
				// Reboot so we boot into the just installed cos
				s.Reboot()
				By("Checking we booted from the installed cOS")
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
			})
			It("forces efi", func() {
				By("Running the installer")
				out, err := s.Command(s.ElementalCmd("install", "--force-efi", "/dev/sda"))
				Expect(err).To(BeNil())
				Expect(out).To(ContainSubstring("Installing GRUB.."))
				Expect(out).To(ContainSubstring("Mounting disk partitions"))
				Expect(out).To(ContainSubstring("Partitioning device..."))
				Expect(out).To(ContainSubstring("Running after-install hook"))
				// Remove iso so we boot directly from the disk
				s.EjectCD()
				// Reboot so we boot into the just installed cos
				s.Reboot()
				// We are on an efi system, should boot from active
				By("Checking we booted from Active partition")
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
			})
		})
	})
})
