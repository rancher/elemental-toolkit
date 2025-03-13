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

	sut "github.com/rancher/elemental-toolkit/v2/tests/vm"

	comm "github.com/rancher/elemental-toolkit/v2/tests/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elemental Recovery upgrade tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
	})

	Context("upgrading COS_ACTIVE from the recovery partition", func() {
		It("upgrades to a specific image", Label("second-test"), func() {
			Expect(s.BootFrom()).To(Equal(sut.Active))
			currentVersion := s.GetOSRelease("TIMESTAMP")

			By("booting into recovery to check the OS version")
			Expect(s.ChangeBoot(sut.Recovery)).To(Succeed())

			s.Reboot()
			s.EventuallyBootedFrom(sut.Recovery)

			By(fmt.Sprintf("upgrading to %s", comm.UpgradeImage()))

			cmd := s.ElementalCmd("upgrade", "--system", comm.UpgradeImage())
			By(fmt.Sprintf("running %s", cmd))

			out, err := s.Command(cmd)
			_, _ = fmt.Fprintln(GinkgoWriter, out)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Upgrade completed"))
			fmt.Fprint(GinkgoWriter, out)
			err = s.ChangeBoot(sut.Active)
			Expect(err).ToNot(HaveOccurred())

			s.Reboot()
			s.EventuallyBootedFrom(sut.Active)

			upgradedVersion := s.GetOSRelease("TIMESTAMP")
			Expect(upgradedVersion).ToNot(Equal(currentVersion))
			s.Reset()
			s.EventuallyBootedFrom(sut.Active)
		})
	})

	// After this test, the VM is no longer in its initial state!!
	Context("upgrading recovery", func() {
		When("using specific images", func() {
			It("upgrades to a specific image", func() {
				Expect(s.BootFrom()).To(Equal(sut.Active))
				By(fmt.Sprintf("upgrading recovery to %s", comm.UpgradeImage()))
				cmd := s.ElementalCmd("upgrade-recovery", "--recovery-system.uri", comm.UpgradeImage())
				By(fmt.Sprintf("running %s", cmd))
				out, err := s.Command(cmd)
				_, _ = fmt.Fprintln(GinkgoWriter, out)
				Expect(err).ToNot(HaveOccurred())
				Expect(out).Should(ContainSubstring("Recovery upgrade completed"))

				By("check state file incluldes the 'upgrade-recovery' action on the recovery image")
				stateStr, err := s.Command(s.ElementalCmd("state"))
				Expect(err).NotTo(HaveOccurred())

				state := map[string]interface{}{}
				Expect(yaml.Unmarshal([]byte(stateStr), state)).
					To(Succeed())
				Expect(state["recovery"].(map[string]interface{})["recovery"].(map[string]interface{})["fromAction"]).
					To(Equal("upgrade-recovery"))

				By("booting into recovery to check the OS version")
				err = s.ChangeBootOnce(sut.Recovery)
				Expect(err).ToNot(HaveOccurred())

				s.Reboot()
				s.EventuallyBootedFrom(sut.Recovery)

				By("rebooting back to active")
				s.Reboot()
				s.EventuallyBootedFrom(sut.Active)
			})
		})
	})
})
