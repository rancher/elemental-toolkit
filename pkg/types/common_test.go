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

package types_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

var _ = Describe("Types", Label("types", "common"), func() {
	Describe("Source", func() {
		It("initiates each type as expected", func() {
			o := &types.ImageSource{}
			Expect(o.Value()).To(Equal(""))
			Expect(o.IsDir()).To(BeFalse())
			Expect(o.IsImage()).To(BeFalse())
			Expect(o.IsFile()).To(BeFalse())
			o = types.NewDirSrc("dir")
			Expect(o.IsDir()).To(BeTrue())
			o = types.NewFileSrc("file")
			Expect(o.IsFile()).To(BeTrue())
			o = types.NewDockerSrc("image")
			Expect(o.IsImage()).To(BeTrue())
			o = types.NewEmptySrc()
			Expect(o.IsEmpty()).To(BeTrue())
			o, err := types.NewSrcFromURI("registry.company.org/image")
			Expect(o.IsImage()).To(BeTrue())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.Value()).To(Equal("registry.company.org/image:latest"))
			o, err = types.NewSrcFromURI("oci://registry.company.org/image:tag")
			Expect(o.IsImage()).To(BeTrue())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.Value()).To(Equal("registry.company.org/image:tag"))
		})
		It("unmarshals each type as expected", func() {
			o := types.NewEmptySrc()
			_, err := o.CustomUnmarshal("docker://some/image")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsImage()).To(BeTrue())
			_, err = o.CustomUnmarshal("dir:///some/absolute/path")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsDir()).To(BeTrue())
			Expect(o.Value() == "/some/absolute/path").To(BeTrue())
			_, err = o.CustomUnmarshal("file://some/relative/path")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsFile()).To(BeTrue())
			Expect(o.Value() == "some/relative/path").To(BeTrue())

			// Opaque URI
			_, err = o.CustomUnmarshal("docker:some/image")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsImage()).To(BeTrue())

			// No scheme is parsed as an image reference and
			// defaults to latest tag if none
			_, err = o.CustomUnmarshal("registry.company.org/my/image")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsImage()).To(BeTrue())
			Expect(o.Value()).To(Equal("registry.company.org/my/image:latest"))

			_, err = o.CustomUnmarshal("registry.company.org/my/image:tag")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsImage()).To(BeTrue())
			Expect(o.Value()).To(Equal("registry.company.org/my/image:tag"))
		})
		It("convertion to string URI works are expected", func() {
			o := types.NewDirSrc("/some/dir")
			Expect(o.IsDir()).To(BeTrue())
			Expect(o.String()).To(Equal("dir:///some/dir"))
			o = types.NewFileSrc("filename")
			Expect(o.IsFile()).To(BeTrue())
			Expect(o.String()).To(Equal("file://filename"))
			o = types.NewDockerSrc("container/image")
			Expect(o.IsImage()).To(BeTrue())
			Expect(o.String()).To(Equal("oci://container/image"))
			o = types.NewEmptySrc()
			Expect(o.IsEmpty()).To(BeTrue())
			Expect(o.String()).To(Equal(""))
			o, err := types.NewSrcFromURI("registry.company.org/image")
			Expect(o.IsImage()).To(BeTrue())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.String()).To(Equal("oci://registry.company.org/image:latest"))
		})
		It("fails to unmarshal non string types", func() {
			o := types.NewEmptySrc()
			_, err := o.CustomUnmarshal(map[string]string{})
			Expect(err).Should(HaveOccurred())
		})
		It("fails to unmarshal unknown scheme and invalid image reference", func() {
			o := types.NewEmptySrc()
			_, err := o.CustomUnmarshal("scheme://some.uri.org")
			Expect(err).Should(HaveOccurred())
		})
		It("fails to unmarshal invalid uri", func() {
			o := types.NewEmptySrc()
			_, err := o.CustomUnmarshal("jp#afs://insanity")
			Expect(err).Should(HaveOccurred())
		})
	})

})
