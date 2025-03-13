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
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	sut "github.com/rancher/elemental-toolkit/v2/tests/vm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elemental booting an expandable disk image", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})
	Context("Wait until system is expanded", func() {
		It("eventually is active", func() {
			Eventually(func() string {
				out, _ := s.Command("cat /run/cos/active_mode")
				return out
			}, 15*time.Minute, 10*time.Second).Should(ContainSubstring("1"))

			// Check the current elemental has 'state' command, upgrade test runs this code
			// against and old image disk image
			helpStr, err := s.Command(s.ElementalCmd("help"))
			Expect(err).NotTo(HaveOccurred())
			if strings.Contains(helpStr, "Shows the install state") {

				By("check state file includes expected actions for the first snapshot and recovery image")
				stateStr, err := s.Command(s.ElementalCmd("state"))
				Expect(err).NotTo(HaveOccurred())

				state := map[string]interface{}{}
				Expect(yaml.Unmarshal([]byte(stateStr), state)).
					To(Succeed())
				Expect(state["state"].(map[string]interface{})["snapshots"].(map[interface{}]interface{})[1].(map[string]interface{})["fromAction"]).
					To(Equal("reset"))
				Expect(state["recovery"].(map[string]interface{})["recovery"].(map[string]interface{})["fromAction"]).
					To(Equal("build-disk"))
			}
		})
	})
})
