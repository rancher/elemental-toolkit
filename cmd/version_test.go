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

	"github.com/rancher/elemental-toolkit/v2/internal/version"
)

var _ = Describe("Version", Label("version", "cmd"), func() {
	BeforeEach(func() {
		rootCmd = NewRootCmd()
		_ = NewVersionCmd(rootCmd)
	})
	It("Reports the version", func() {
		_, output, err := executeCommandC(rootCmd, "version")
		Expect(err).To(BeNil())
		v := version.Get().Version
		Expect(output).To(ContainSubstring(v))
	})
	It("Reports the version in long format", Label("flags"), func() {
		_, output, err := executeCommandC(rootCmd, "version", "--long")
		Expect(err).To(BeNil())
		v := version.Get().Version
		Expect(output).To(ContainSubstring(v))
		Expect(output).To(ContainSubstring("Version"))
		Expect(output).To(ContainSubstring("GitCommit"))
		Expect(output).To(ContainSubstring("GoVersion"))
	})
})
