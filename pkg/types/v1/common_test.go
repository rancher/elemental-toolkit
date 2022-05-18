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

package v1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
)

var _ = Describe("Types", Label("types", "common"), func() {
	Describe("Source", func() {
		It("initiates each type as expected", func() {
			o := &v1.ImageSource{}
			Expect(o.Value()).To(Equal(""))
			Expect(o.IsDir()).To(BeFalse())
			Expect(o.IsChannel()).To(BeFalse())
			Expect(o.IsDocker()).To(BeFalse())
			Expect(o.IsFile()).To(BeFalse())
			o = v1.NewDirSrc("dir")
			Expect(o.IsDir()).To(BeTrue())
			o = v1.NewFileSrc("file")
			Expect(o.IsFile()).To(BeTrue())
			o = v1.NewDockerSrc("image")
			Expect(o.IsDocker()).To(BeTrue())
			o = v1.NewChannelSrc("channel")
			Expect(o.IsChannel()).To(BeTrue())
			o = v1.NewEmptySrc()
			Expect(o.IsEmpty()).To(BeTrue())
		})
		It("unmarshals each type as expected", func() {
			o := v1.NewEmptySrc()
			_, err := o.CustomUnmarshal("docker://some/image")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsDocker()).To(BeTrue())
			_, err = o.CustomUnmarshal("channel://some/package")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.IsChannel()).To(BeTrue())
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
			Expect(o.IsDocker()).To(BeTrue())
		})
		It("fails to unmarshal non string types", func() {
			o := v1.NewEmptySrc()
			_, err := o.CustomUnmarshal(map[string]string{})
			Expect(err).Should(HaveOccurred())
		})
		It("fails to unmarshal unknown scheme", func() {
			o := v1.NewEmptySrc()
			_, err := o.CustomUnmarshal("scheme:some.opaque.uri.org")
			Expect(err).Should(HaveOccurred())
		})
		It("fails to unmarshal invalid uri", func() {
			o := v1.NewEmptySrc()
			_, err := o.CustomUnmarshal("jp#afs://insanity")
			Expect(err).Should(HaveOccurred())
		})
	})

})
