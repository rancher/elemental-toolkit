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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"os"
)

var _ = Describe("Types", func() {
	var luet *v1.Luet
	var target string
	BeforeEach(func() {
		var err error
		target, err = os.MkdirTemp("", "elemental")
		Expect(err).To(BeNil())
		luet = v1.NewLuet(v1.NewNullLogger(), []string{}...)
	})
	AfterEach(func() {
		Expect(os.RemoveAll(target)).To(BeNil())
	})
	Context("Luet", func() {
		It("Fails to unpack without root privileges", func() {
			image := "quay.io/costoolkit/releases-green:cloud-config-system-0.11-1"
			Expect(luet.Unpack(target, image)).NotTo(BeNil())
		})
	})
})
