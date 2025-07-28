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
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elemental booting fallback tests", func() {
	var s *sut.SUT

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
		}
	})

	Context("GRUB cannot mount image", func() {
		When("COS_ACTIVE image was corrupted", func() {
			It("fallbacks by booting into passive", func() {
				Expect(s.BootFrom()).To(Equal(sut.Active))

				_, err := s.Command("mount -o rw,remount /run/initramfs/cos-state")
				Expect(err).ToNot(HaveOccurred())
				_, err = s.Command("rm -rf /run/initramfs/cos-state/cOS/active.img")
				Expect(err).ToNot(HaveOccurred())

				s.Reboot()

				Expect(s.BootFrom()).To(Equal(sut.Passive))

				// Here we did fallback from grub. boot assessment didn't kicked in here
				cmdline, _ := s.Command("sudo cat /proc/cmdline")
				Expect(cmdline).ToNot(And(ContainSubstring("upgrade_failure")), cmdline)
			})
		})
		When("COS_ACTIVE and COS_PASSIVE images are corrupted", func() {
			It("fallbacks by booting into recovery", func() {
				Expect(s.BootFrom()).To(Equal(sut.Active))

				_, err := s.Command("mount -o rw,remount /run/initramfs/cos-state")
				Expect(err).ToNot(HaveOccurred())
				_, err = s.Command("rm -rf /run/initramfs/cos-state/cOS/active.img")
				Expect(err).ToNot(HaveOccurred())
				_, err = s.Command("rm -rf /run/initramfs/cos-state/cOS/passive.img")
				Expect(err).ToNot(HaveOccurred())
				s.Reboot()

				Expect(s.BootFrom()).To(Equal(sut.Recovery))
			})
		})
	})
})
