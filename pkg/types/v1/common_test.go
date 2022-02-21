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
		It("Initiates each type as expected", func() {
			o := v1.ImageSource{}
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

		})
	})

})
