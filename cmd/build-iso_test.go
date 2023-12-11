/*
Copyright © 2022 - 2023 SUSE LLC

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
	"github.com/spf13/viper"
)

var _ = Describe("BuidISO", Label("iso", "cmd"), func() {
	var buf *bytes.Buffer
	BeforeEach(func() {
		rootCmd = NewRootCmd()
		_ = NewBuildISO(rootCmd, false)
		buf = new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
	})
	AfterEach(func() {
		viper.Reset()
	})
	It("Errors out setting firmware to anything else than efi", Label("flags"), func() {
		_, _, err := executeCommandC(rootCmd, "build-iso", "--firmware", "bios")
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(ContainSubstring("invalid argument"))
		Expect(err.Error()).To(ContainSubstring("'bios' is not included in: efi"))
	})
	It("Errors out setting consign-key without setting cosign", Label("flags"), func() {
		_, _, err := executeCommandC(rootCmd, "build-iso", "--cosign-key", "pubKey.url")
		Expect(err).ToNot(BeNil())
		Expect(buf.String()).To(ContainSubstring("Usage:"))
		Expect(err.Error()).To(ContainSubstring("'cosign-key' requires 'cosign' option to be enabled"))
	})
	It("Errors out if no rootfs sources are defined", Label("flags"), func() {
		_, _, err := executeCommandC(rootCmd, "build-iso")
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(ContainSubstring("rootfs source image for building ISO was not provided"))
	})
	It("Errors out if rootfs is a non valid argument", Label("flags"), func() {
		_, _, err := executeCommandC(rootCmd, "build-iso", "/no/image/reference")
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(ContainSubstring("invalid image reference"))
	})
	It("Errors out if overlay roofs path does not exist", Label("flags"), func() {
		_, _, err := executeCommandC(
			rootCmd, "build-iso", "system/cos", "--overlay-rootfs", "/nonexistingpath",
		)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(ContainSubstring("Invalid path"))
	})
	It("Errors out if overlay uefi path does not exist", Label("flags"), func() {
		_, _, err := executeCommandC(
			rootCmd, "build-iso", "someimage:latest", "--overlay-uefi", "/nonexistingpath",
		)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(ContainSubstring("Invalid path"))
	})
	It("Errors out if overlay iso path does not exist", Label("flags"), func() {
		_, _, err := executeCommandC(
			rootCmd, "build-iso", "some/image:latest", "--overlay-iso", "/nonexistingpath",
		)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(ContainSubstring("Invalid path"))
	})
})
