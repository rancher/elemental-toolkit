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
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"

	. "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// unit test stolen from yip
var _ = Describe("CloudRunner", Label("CloudRunner", "types", "cloud-init"), func() {
	Describe("loading yaml files", func() {
		logger := logrus.New()
		logger.SetOutput(ioutil.Discard)

		runner := NewYipCloudInitRunner(logger)

		It("executes commands", func() {

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/01_first.yaml": `
stages:
  test:
  - commands:
    - sed -i 's/boo/bar/g' ` + temp + `/tmp/test/bar
`,
				"/some/yip/02_second.yaml": `
stages:
  test:
  - commands:
    - sed -i 's/bar/baz/g' ` + temp + `/tmp/test/bar
`,
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = fs2.Mkdir("/tmp", os.ModePerm)
			Expect(err).Should(BeNil())
			err = fs2.Mkdir("/tmp/test", os.ModePerm)
			Expect(err).Should(BeNil())

			err = fs2.WriteFile("/tmp/test/bar", []byte(`boo`), os.ModePerm)
			Expect(err).Should(BeNil())

			runner.SetFs(fs)
			err = runner.Run("test", "/some/yip")
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("baz"))
		})
	})
})
