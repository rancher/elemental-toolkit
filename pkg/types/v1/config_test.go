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

package v1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("ImageMap", func() {
		It("Sets and gets images as expected", func() {
			imgMap := v1.ImageMap{}
			active := &v1.Image{Label: "active"}
			passive := &v1.Image{Label: "passive"}
			recovery := &v1.Image{Label: "recovery"}
			Expect(imgMap.GetActive()).To(BeNil())
			imgMap.SetActive(active)
			imgMap.SetPassive(passive)
			imgMap.SetRecovery(recovery)
			Expect(imgMap.GetActive()).To(Equal(active))
			Expect(imgMap.GetActive().Label).To(Equal("active"))
			Expect(imgMap.GetPassive()).To(Equal(passive))
			Expect(imgMap.GetPassive().Label).To(Equal("passive"))
			Expect(imgMap.GetRecovery()).To(Equal(recovery))
			Expect(imgMap.GetRecovery().Label).To(Equal("recovery"))

		})
	})

})
