/*
Copyright © 2022 - 2025 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package elemental_test

import (
	"fmt"
	"time"

	sut "github.com/rancher-sandbox/ele-testhelpers/vm"

	comm "github.com/rancher/elemental-toolkit/tests/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elemental booting fallback tests", func() {
	var s *sut.SUT
	bootAssessmentInstalled := func() {
		// Auto assessment was installed
		out, _ := s.Command("sudo cat /run/initramfs/cos-state/grubcustom")
		Expect(out).To(ContainSubstring("bootfile_loc"))

		out, _ = s.Command("sudo cat /run/initramfs/cos-state/grub_boot_assessment")
		Expect(out).To(ContainSubstring("boot_assessment_file"))

		cmdline, _ := s.Command("sudo cat /proc/cmdline")
		Expect(cmdline).To(ContainSubstring("rd.emergency=reboot rd.shell=0 panic=5"))
	}

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})
	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
		if !CurrentSpecReport().Failed() {
			s.Reset()
			// Verify after reset the boot assessment is reinstalled
			bootAssessmentInstalled()
		}
	})

	Context("image is corrupted", func() {
		breakPaths := []string{"usr/lib/systemd", "bin/sh", "bin/bash", "usr/bin/bash", "usr/bin/sh"}
		It("boots in fallback when rootfs is damaged, triggered by missing files", func() {
			currentVersion := s.GetOSRelease("TIMESTAMP")

			// Auto assessment was installed
			bootAssessmentInstalled()

			out, err := s.Command(s.ElementalCmd("upgrade", "--system.uri", comm.UpgradeImage()))
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).Should(ContainSubstring("Upgrade completed"))

			out, _ = s.Command("sudo cat /run/initramfs/cos-state/boot_assessment")
			Expect(out).To(ContainSubstring("enable_boot_assessment=yes"))

			// Break the upgrade
			out, _ = s.Command("sudo mount -o rw,remount /run/initramfs/cos-state")
			fmt.Println(out)

			out, _ = s.Command("sudo mkdir -p /tmp/mnt/STATE")
			fmt.Println(out)

			s.Command("sudo mount /run/initramfs/cos-state/cOS/active.img /tmp/mnt/STATE")

			for _, d := range breakPaths {
				out, _ = s.Command("sudo rm -rfv /tmp/mnt/STATE/" + d)
			}

			out, _ = s.Command("sudo ls -liah /tmp/mnt/STATE/")
			fmt.Println(out)

			out, _ = s.Command("sudo umount /tmp/mnt/STATE")
			s.Command("sudo sync")

			s.Reboot(700)

			v := s.GetOSRelease("TIMESTAMP")
			Expect(v).To(Equal(currentVersion))

			cmdline, _ := s.Command("sudo cat /proc/cmdline")
			Expect(cmdline).To(And(ContainSubstring("passive.img"), ContainSubstring("upgrade_failure")), cmdline)

			Eventually(func() string {
				out, _ := s.Command("sudo ls -liah /run/cos")
				return out
			}, 5*time.Minute, 10*time.Second).Should(ContainSubstring("upgrade_failure"))
		})
	})
})
