package cos_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sut "github.com/rancher/elemental-toolkit/tests/vm"
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
					out, err := s.Command(s.ElementalCmd("install", "--squash-no-compression", "/dev/vda"))
					fmt.Printf(out)
					Expect(err).To(BeNil())
					Expect(out).To(ContainSubstring("Mounting disk partitions"))
					Expect(out).To(ContainSubstring("Partitioning device..."))
					Expect(out).To(ContainSubstring("Running after-install hook"))

					// Reboot so we boot into the just installed system
					s.Reboot()
					By("Checking we booted from the installed system")
					ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
					// check partition values
					// Values have to match the yaml under ../assets/layout.yaml
					// That is the file that the installer uses so partitions should match those values
					disk := s.GetDiskLayout("/dev/vda")

					for _, part := range []sut.PartitionEntry{
						{
							Label: "COS_STATE",
							Size:  6144,
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
							Size:   1024,
							FsType: sut.Ext2,
						},
					} {
						CheckPartitionValues(disk, part)
					}
				})
			})
		})
	})
})
