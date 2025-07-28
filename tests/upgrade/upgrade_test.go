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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sut "github.com/rancher-sandbox/ele-testhelpers/vm"

	comm "github.com/rancher/elemental-toolkit/tests/common"
)

var _ = Describe("Elemental Feature tests", func() {
	var s *sut.SUT
	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
		Expect(s.BootFrom()).To(Equal(sut.Active))
	})

	Context("After install", func() {
		It("upgrades to a signed image including upgrade and reset hooks", func() {
			By("setting /oem/chroot_hooks.yaml")
			err := s.SendFile("../assets/chroot_hooks.yaml", "/oem/chroot_hooks.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())
			originalVersion := s.GetOSRelease("TIMESTAMP")

			By(fmt.Sprintf("and upgrading the to %s", comm.UpgradeImage()))
			// TODO: make use of verified images and check unsigned images can't be pulled
			//out, err := s.Command(s.ElementalCmd("upgrade", "--verify", "--system.uri", comm.UpgradeImage()))
			//Expect(err).To(HaveOccurred())
			out, err := s.Command(s.ElementalCmd("upgrade", "--system.uri", comm.UpgradeImage()))
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade completed"))

			s.Reboot()
			Expect(s.BootFrom()).To(Equal(sut.Active))
			currentVersion := s.GetOSRelease("TIMESTAMP")
			Expect(currentVersion).NotTo(Equal(originalVersion))

			_, err = s.Command("cat /after-upgrade-chroot")
			Expect(err).ToNot(HaveOccurred())

			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).To(HaveOccurred())

			s.Reset()
			currentVersion = s.GetOSRelease("TIMESTAMP")
			Expect(currentVersion).To(Equal(originalVersion))
			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
