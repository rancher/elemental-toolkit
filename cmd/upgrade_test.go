/*
Copyright © 2022 SUSE LLC

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
	"bytes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Upgrade", Label("upgrade", "cmd", "systemctl"), func() {
	It("Returns error if both --docker-image and --directory flags are used", Label("flags"), func() {
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		_, _, err := executeCommandC(rootCmd, "upgrade", "--docker-image", "img", "--directory", "/tmp")
		// Restore cobra output
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		Expect(err).To(HaveOccurred())
	})
})
