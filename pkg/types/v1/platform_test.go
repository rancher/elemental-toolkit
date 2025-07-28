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

package v1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
)

var _ = Describe("Platform", Label("types", "platform"), func() {
	Describe("Source", func() {
		It("initiates platform as expected", func() {
			platform, err := v1.NewPlatform("linux", "x86_64")
			Expect(err).To(BeNil())
			Expect(platform.OS).To(Equal("linux"))
			Expect(platform.Arch).To(Equal("x86_64"))
			Expect(platform.GolangArch).To(Equal("amd64"))
		})
		It("parses platform as expected", func() {
			platform, err := v1.ParsePlatform("linux/amd64")
			Expect(err).To(BeNil())
			Expect(platform.OS).To(Equal("linux"))
			Expect(platform.Arch).To(Equal("x86_64"))
			Expect(platform.GolangArch).To(Equal("amd64"))
		})
		It("initiates arm64 platform as expected", func() {
			platform, err := v1.NewPlatformFromArch("arm64")
			Expect(err).To(BeNil())
			Expect(platform.OS).To(Equal("linux"))
			Expect(platform.Arch).To(Equal("arm64"))
			Expect(platform.GolangArch).To(Equal("arm64"))
		})
	})
})
