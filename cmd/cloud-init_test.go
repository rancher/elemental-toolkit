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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cloud-init", Label("cloud-init", "cmd"), func() {
	Describe("execution", func() {
		When("invoked with inline yaml", Label("inline", "yaml"), func() {
			BeforeEach(func() {
				rootCmd = NewRootCmd()
				_ = NewCloudInitCmd(rootCmd)
			})

			It("executes command correctly", func() {
				_, out, err := executeCommandC(
					rootCmd,
					"cloud-init",
					"-s",
					"tests",
					"-d",
					"'stages.tests[0].commands[0]=\"echo foobarz\"'",
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(out).To(ContainSubstring("foobarz"))
			})

			It("fails when a malformed yaml is given", Label("args"), func() {
				_, _, err := executeCommandC(
					rootCmd,
					"cloud-init",
					"-s",
					"tests",
					"-d",
					"'stages.tests=foo'",
				)
				Expect(err).To(HaveOccurred())
			})

			It("ignores empty input", Label("args"), func() {
				_, _, err := executeCommandC(
					rootCmd,
					"cloud-init",
					"-s",
					"tests",
					"-",
				)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
