/*
Copyright © 2021 SUSE LLC

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

package cmd

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("run-stage", Label("run-stage", "cmd"), func() {
	When("execution", func() {
		BeforeEach(func() {
			rootCmd = NewRootCmd()
			_ = NewRunStage(rootCmd)
		})
		AfterEach(func() {
			viper.Reset()
		})
		It("executes command correctly", func() {
			_, out, err := executeCommandC(
				rootCmd,
				"run-stage",
				"test",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("test"))
			Expect(out).To(ContainSubstring("test.before"))
			Expect(out).To(ContainSubstring("test.after"))
		})
		// This requires fixing the env vars, otherwise it wont work
		It("picks extra paths correctly", Pending, Label("env"), func() {
			d, _ := os.MkdirTemp("", "elemental")
			defer os.RemoveAll(d)
			_ = os.Setenv("ELEMENTAL_CLOUD_INIT_PATHS", d)
			_, out, err := executeCommandC(
				rootCmd,
				"run-stage",
				"test",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring(d))
		})
		It("fails when stage is missing", Label("args"), func() {
			_, _, err := executeCommandC(
				rootCmd,
				"run-stage",
			)
			Expect(err).To(HaveOccurred())
		})
	})
})
