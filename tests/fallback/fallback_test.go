/*
Copyright © 2022 - 2026 SUSE LLC

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elemental booting fallback tests", func() {
	var s *sut.SUT
	bootAssessmentInstalled := func() {
		// Boot assessment was installed
		out, _ := s.Command("sudo cat /usr/sbin/elemental-boot-assessment")
		Expect(out).To(ContainSubstring("BootAssessment"))

		cmdline, _ := s.Command("sudo cat /proc/cmdline")
		Expect(cmdline).To(ContainSubstring("rd.emergency=reboot rd.shell=0"))
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
			// Auto assessment was installed
			bootAssessmentInstalled()

			err := s.SendFile("../assets/break_upgrade_hook.yaml", "/oem/break_upgrade_hook.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			By("Upgrading to a broken system")
			out, err := s.Command(s.ElementalCmd("upgrade", "--system", "dir:/.snapshots/1/snapshot"))
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).Should(ContainSubstring("Upgrade completed"))

			out, _ = s.Command("sudo cat /oem/grubenv")
			Expect(out).To(ContainSubstring("boot_assessment_check=yes"))

			By("Rebooting after the upgrade completed")
			s.Reboot()

			By("Checking it rebooted to fallback")
			cmdline, err := s.Command("sudo cat /run/elemental/passive_mode")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cmdline).To(ContainSubstring("1"))

			cmdline, err = s.Command("sudo cat /proc/cmdline")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cmdline).To(ContainSubstring("elemental.health_check"))

			_, err = s.Command("sudo rm /oem/break_upgrade_hook.yaml")
			Expect(err).ShouldNot(HaveOccurred())

			err = s.SendFile("../assets/boot_checker_failure.yaml", "/oem/boot_checker_failure.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			By("Upgrading again including checkers that will always fail")
			out, err = s.Command(s.ElementalCmd("upgrade", "--system", "dir:/.snapshots/1/snapshot"))
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).Should(ContainSubstring("Upgrade completed"))

			out, _ = s.Command("sudo cat /oem/grubenv")
			Expect(out).To(ContainSubstring("boot_assessment_check=yes"))

			By("Rebooting after the upgrade completed")
			s.Reboot()

			By("Checking it rebooted to active")
			cmdline, err = s.Command("sudo cat /run/elemental/active_mode")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cmdline).To(ContainSubstring("1"))

			By("Waiting for the failed health checker to trigger a reboot")
			s.EventuallyDisconnects(300)

			By("Eventually rebooting")
			// Give some time to make sure reboot close ssh connection
			time.Sleep(5 * time.Second)
			s.EventuallyConnects()

			By("Checking it rebooted to fallback")
			cmdline, err = s.Command("sudo cat /run/elemental/passive_mode")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cmdline).To(ContainSubstring("1"))

			cmdline, err = s.Command("sudo cat /proc/cmdline")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cmdline).To(ContainSubstring("elemental.health_check"))

			By("Checking boot assessment succeeded")
			_, err = s.Command("sudo systemctl is-active -q elemental-boot-assessment.service")
			Expect(err).ShouldNot(HaveOccurred())

			_, err = s.Command("sudo rm /oem/boot_checker_failure.yaml")
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
