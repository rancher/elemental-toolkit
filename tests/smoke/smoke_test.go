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
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sut "github.com/rancher/elemental-toolkit/tests/vm"
)

var _ = Describe("Elemental Smoke tests", func() {
	var s *sut.SUT
	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
		Expect(s.BootFrom()).To(Equal(sut.Active))
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
	})

	Context("After install", func() {
		It("has default services on", func() {
			for _, svc := range []string{"systemd-timesyncd"} {
				sut.SystemdUnitIsActive(svc, s)
			}
		})

		It("it can reboot into recovery and back to active having active persistent data still available", func() {
			By("Adding some persistent data in root folder")
			persistentFileName := fmt.Sprintf("file-%v.txt", rand.Int())
			persistentData := rand.Uint32()
			_, err := s.Command(fmt.Sprintf("echo %v > %v", persistentData, persistentFileName))
			Expect(err).ToNot(HaveOccurred())

			Expect(err).ToNot(HaveOccurred())
			s.ChangeBoot(sut.Recovery)
			By("rebooting into recovery")
			s.Reboot()

			Expect(s.BootFrom()).To(Equal(sut.Recovery))

			out, err := s.Command("cat /run/cos/recovery_mode")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal("1"))

			By("switching back to active")
			s.ChangeBoot(sut.Active)
			s.Reboot()
			Expect(s.BootFrom()).To(Equal(sut.Active))

			By("reading previously stored data")
			data, err := s.Command(fmt.Sprintf("cat %v", persistentFileName))
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(Equal(fmt.Sprintf("%v\n", persistentData)))
			_, err = s.Command(fmt.Sprintf("rm %v", persistentFileName))
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails running elemental reset from COS_ACTIVE", func() {
			Expect(s.BootFrom()).To(Equal(sut.Active))
			out, err := s.Command(s.ElementalCmd("reset"))
			Expect(err).To(HaveOccurred())
			Expect(out).Should(ContainSubstring("reset can only be called from the recovery system"))
		})
	})

	Context("Settings", func() {
		It("has correct defaults", func() {
			out, err := s.Command("source /etc/os-release && echo $GRUB_ENTRY_NAME")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Elemental"))
		})

		It("has default date in UTC format from cloud-init", func() {
			out, err := s.Command("date")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("UTC"))
		})

		It("has default localectl configuration from cloud-init", func() {
			out, err := s.Command("localectl status")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("LANG=en_US.UTF-8"))
			Expect(out).Should(ContainSubstring("VC Keymap: us"))
		})
	})
})
