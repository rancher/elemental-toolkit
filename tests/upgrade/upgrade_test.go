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

	"gopkg.in/yaml.v3"

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

			By("checking upgrade hooks were applied")
			_, err = s.Command("cat /after-upgrade-chroot")
			Expect(err).ToNot(HaveOccurred())

			_, err = s.Command("cat /after-reset-chroot")
			Expect(err).To(HaveOccurred())

			By("check state file includes expected actions for the upgraded snapshot")
			stateStr, err := s.Command(s.ElementalCmd("state"))
			Expect(err).NotTo(HaveOccurred())

			state := map[string]interface{}{}
			Expect(yaml.Unmarshal([]byte(stateStr), state)).
				To(Succeed())
			Expect(state["state"].(map[string]interface{})["snapshots"].(map[interface{}]interface{})[2].(map[string]interface{})["fromAction"]).
				To(Equal("upgrade"))

			By("booting into recovery to check it is still functional")
			Expect(s.ChangeBootOnce(sut.Recovery)).To(Succeed())

			s.Reboot()
			s.EventuallyBootedFrom(sut.Recovery)

			Expect(originalVersion).To(Equal(s.GetOSRelease("TIMESTAMP")))

			By("reboot back to active to upgrade recovery now")
			s.Reboot()
			s.EventuallyBootedFrom(sut.Active)

			upgradeRecCmd := s.ElementalCmd("upgrade-recovery", "--recovery-system.uri", comm.UpgradeImage())
			_, err = s.NewPodmanRunCommand(comm.ToolkitImage(), fmt.Sprintf("-c \"mount --rbind /host/run /run && %s\"", upgradeRecCmd)).
				Privileged().
				NoTLSVerify().
				WithMount("/", "/host").
				Run()
			Expect(err).ToNot(HaveOccurred())

			By("booting into recovery to check it is still functional")
			Expect(s.ChangeBootOnce(sut.Recovery)).To(Succeed())

			s.Reboot()
			s.EventuallyBootedFrom(sut.Recovery)

			By("check state file incluldes the 'upgrade-recovery' action on the recovery image")
			stateStr, err = s.Command(s.ElementalCmd("state"))
			Expect(err).NotTo(HaveOccurred())

			state = map[string]interface{}{}
			Expect(yaml.Unmarshal([]byte(stateStr), state)).
				To(Succeed())
			Expect(state["recovery"].(map[string]interface{})["recovery"].(map[string]interface{})["fromAction"]).
				To(Equal("upgrade-recovery"))

			Expect(currentVersion).To(Equal(s.GetOSRelease("TIMESTAMP")))

			By("all done, back to active")
			s.Reboot()
		})
	})
})
