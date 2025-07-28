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

package features_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/elemental-toolkit/pkg/features"
)

func TestTypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "features test suite")
}

var _ = Describe("Init Action", func() {
	Describe("Features", Label("features"), func() {
		It("Returns an empty list of features", func() {
			feats, err := features.Get([]string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(feats).To(BeEmpty())
		})
		It("Returns error for unknown feature", func() {
			_, err := features.Get([]string{"unknown-abc"})
			Expect(err).To(HaveOccurred())
		})
	})
})
