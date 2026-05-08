/*
Copyright © 2022 - 2026 SUSE LLC

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
	"github.com/spf13/viper"
)

var _ = Describe("Upgrade", Label("upgrade", "cmd"), func() {
	BeforeEach(func() {
		rootCmd = NewRootCmd()
		_ = NewUpgradeCmd(rootCmd, false)
	})
	AfterEach(func() {
		viper.Reset()
	})
	It("Returns error if both --docker-image and --directory flags are used", Label("flags"), func() {
		_, _, err := executeCommandC(rootCmd, "upgrade", "--docker-image", "img", "--directory", "/tmp")
		Expect(err).To(HaveOccurred())
	})
})
