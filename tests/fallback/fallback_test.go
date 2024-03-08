/*
Copyright © 2022 - 2024 SUSE LLC

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
	"time"

	sut "github.com/rancher/elemental-toolkit/v2/tests/vm"

	comm "github.com/rancher/elemental-toolkit/v2/tests/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elemental booting fallback tests", func() {
	var s *sut.SUT
	bootAssessmentInstalled := func() {
		// Auto assessment was installed
		out, _ := s.Command("sudo cat /run/elemental/efi/grubcustom")
		Expect(out).To(ContainSubstring("bootfile"))

		out, _ = s.Command("sudo cat /run/elemental/efi/grub_boot_assessment")
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
		It("boots in fallback when rootfs is damaged, triggered by missing files", func() {
			currentVersion := s.GetOSRelease("TIMESTAMP")

			// Auto assessment was installed
			bootAssessmentInstalled()

			err := s.SendFile("../assets/break_upgrade_hook.yaml", "/oem/break_upgrade_hook.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			out, err := s.Command(s.ElementalCmd("upgrade", "--system", comm.UpgradeImage()))
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).Should(ContainSubstring("Upgrade completed"))

			out, _ = s.Command("sudo cat /run/elemental/efi/boot_assessment")
			Expect(out).To(ContainSubstring("enable_boot_assessment=yes"))

			s.Reboot(700)

			v := s.GetOSRelease("TIMESTAMP")
			Expect(v).To(Equal(currentVersion))

			cmdline, _ := s.Command("sudo cat /proc/cmdline")
			Expect(cmdline).To(And(ContainSubstring("passive"), ContainSubstring("upgrade_failure")), cmdline)

			Eventually(func() string {
				out, _ := s.Command("sudo ls -liah /run/elemental")
				return out
			}, 5*time.Minute, 10*time.Second).Should(ContainSubstring("upgrade_failure"))
		})
	})
})
