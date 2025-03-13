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

	sut "github.com/rancher/elemental-toolkit/v2/tests/vm"

	comm "github.com/rancher/elemental-toolkit/v2/tests/common"
)

var _ = Describe("Elemental Feature tests", func() {
	var s *sut.SUT
	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
		s.EventuallyBootedFrom(sut.Active)
	})

	Context("After install", func() {
		It("upgrades to a signed image including upgrade and reset hooks", func() {
			By("setting /oem/chroot_hooks.yaml")
			err := s.SendFile("../assets/chroot_hooks.yaml", "/oem/chroot_hooks.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			// Upgrade tests might not include it as it booted from an older disk image.
			err = s.SendFile("../assets/remote_login.yaml", "/oem/remote_login.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			originalVersion := s.GetOSRelease("TIMESTAMP")

			By(fmt.Sprintf("and upgrading to %s", comm.UpgradeImage()))

			upgradeCmd := s.ElementalCmd("upgrade", "--tls-verify=false", "--bootloader", "--system", comm.UpgradeImage())
			out, err := s.NewPodmanRunCommand(comm.ToolkitImage(), fmt.Sprintf("-c \"mount --rbind /host/run /run && %s\"", upgradeCmd)).
				Privileged().
				NoTLSVerify().
				WithMount("/", "/host").
				Run()

			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade completed"))

			s.Reboot()
			s.EventuallyBootedFrom(sut.Active)
			currentVersion := s.GetOSRelease("TIMESTAMP")
			Expect(currentVersion).NotTo(Equal(originalVersion))

			_, err = s.Command("cat /after-upgrade-chroot")
			Expect(err).ToNot(HaveOccurred())

			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).To(HaveOccurred())
		})
	})
})
