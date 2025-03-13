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

package http_test

import (
	"os"
	"path/filepath"

	"github.com/rancher/elemental-toolkit/v2/pkg/http"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const source = "https://raw.githubusercontent.com/rancher/elemental-toolkit/main/README.md"

var _ = Describe("HTTPClient", Label("http"), func() {
	var client *http.Client
	var log types.Logger
	var destDir string
	BeforeEach(func() {
		client = http.NewClient()
		log = types.NewNullLogger()
		destDir, _ = os.MkdirTemp("", "elemental-test")
	})
	AfterEach(func() {
		os.RemoveAll(destDir)
	})
	It("Downloads a test file to destination folder", func() {
		// Download a public elemental release
		_, err := os.Stat(filepath.Join(destDir, "README.md"))
		Expect(err).NotTo(BeNil())
		Expect(client.GetURL(log, source, destDir)).To(BeNil())
		_, err = os.Stat(filepath.Join(destDir, "README.md"))
		Expect(err).To(BeNil())
	})
	It("Downloads a test file to some specified destination file", func() {
		// Download a public elemental release
		_, err := os.Stat(filepath.Join(destDir, "testfile"))
		Expect(err).NotTo(BeNil())
		Expect(client.GetURL(log, source, filepath.Join(destDir, "testfile"))).To(BeNil())
		_, err = os.Stat(filepath.Join(destDir, "testfile"))
		Expect(err).To(BeNil())
	})
	It("Fails to download a non existing url", func() {
		source := "http://nonexisting.stuff"
		Expect(client.GetURL(log, source, destDir)).NotTo(BeNil())
	})
	It("Fails to download a broken url", func() {
		source := "scp://23412342341234.wqer.234|@#~ł€@¶|@~#"
		Expect(client.GetURL(log, source, destDir)).NotTo(BeNil())
	})
})
