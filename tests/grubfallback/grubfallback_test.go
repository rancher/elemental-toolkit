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
	"fmt"

	sut "github.com/rancher/elemental-toolkit/v2/tests/vm"

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
			It("fallbacks by booting into recovery", func() {
				Expect(s.BootFrom()).To(Equal(sut.Active))

				_, err := s.Command("mount -o rw,remount /run/elemental/efi")
				Expect(err).ToNot(HaveOccurred())
				cmd := "grub2-editenv"
				_, err = s.Command(fmt.Sprintf("which %s", cmd))
				if err != nil {
					cmd = "grub-editenv"
				}

				_, err = s.Command(fmt.Sprintf("%s /run/elemental/efi/grub_oem_env set state_label=wrongvalue", cmd))
				Expect(err).ToNot(HaveOccurred())

				s.Reboot()

				Expect(s.BootFrom()).To(Equal(sut.Recovery))

				// Here we did fallback from grub. boot assessment didn't kicked in here
				cmdline, _ := s.Command("sudo cat /proc/cmdline")
				Expect(cmdline).ToNot(And(ContainSubstring("upgrade_failure")), cmdline)
			})
		})
	})
})
