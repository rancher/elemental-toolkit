package cos_test

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sut "github.com/rancher/elemental-toolkit/v2/tests/vm"
)

func generatePassphrase(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)

	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}

var _ = Describe("Elemental Installer encryption tests", func() {
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
					err := s.SendFile("../assets/encrypted_extra_partition.yaml", "/etc/elemental/config.d/encrypted_extra_partition.yaml", "0770")
					Expect(err).ToNot(HaveOccurred())

					passphrase := generatePassphrase(10)
					By(fmt.Sprintf("Running the elemental install with passphrase '%s'", passphrase))
					out, err := s.Command(s.ElementalCmd("install", "--squash-no-compression", "/dev/vda", "--encrypt-persistent", "--enroll-passphrase", passphrase))
					Expect(err).ToNot(HaveOccurred())
					Expect(out).To(ContainSubstring("Mounting disk partitions"))
					Expect(out).To(ContainSubstring("Partitioning device..."))
					Expect(out).To(ContainSubstring("Running after-install hook"))

					// Mount OEM before changing boot entry
					_, err = s.Command("mount /dev/disk/by-partlabel/oem /oem")
					Expect(err).ToNot(HaveOccurred())

					// Trying to mount from the installed system will lock
					// waiting for the passphrase, so we boot into recovery to
					// verify we can mount the encrypted partitions using the
					// expected passphrases.
					err = s.ChangeBootOnce(sut.Recovery)
					Expect(err).ToNot(HaveOccurred())
					s.Reboot()
					s.AssertBootedFrom(sut.Recovery)

					By("Mounting cr_persistent")
					_, err = s.Command(fmt.Sprintf("echo %s | cryptsetup open --type luks /dev/disk/by-partlabel/persistent cr_persistent", passphrase))
					Expect(err).ToNot(HaveOccurred())

					By("Mounting cr_extra from config file")
					_, err = s.Command("echo extrapass | cryptsetup open --type luks /dev/disk/by-partlabel/extra cr_extra")
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})
