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

package cloudinit_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"

	. "github.com/rancher-sandbox/elemental/pkg/cloudinit"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental/tests/mocks"
	"github.com/spf13/afero"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Parted print sample output
const printOutput = `BYT;
/dev/loop0:50593792s:loopback:512:512:msdos:Loopback device:;
1:2048s:98303s:96256s:ext4::type=83;
2:98304s:29394943s:29296640s:ext4::boot, type=83;
3:29394944s:45019135s:15624192s:ext4::type=83;`

var _ = Describe("CloudRunner", Label("CloudRunner", "types", "cloud-init"), func() {
	// unit test stolen from yip
	Describe("loading yaml files", func() {
		logger := logrus.New()
		logger.SetOutput(ioutil.Discard)

		runner := NewYipCloudInitRunner(logger, &v1.RealRunner{})

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
	Describe("layout plugin execution", func() {
		var runner *v1mock.FakeRunner
		var cloudRunner *YipCloudInitRunner
		var afs afero.Fs
		var cmdFail string
		var partNum int
		BeforeEach(func() {
			afs = afero.NewOsFs()
			_, err := afs.Create("/tmp/device")
			Expect(err).To(BeNil())
			runner = v1mock.NewFakeRunner()
			logger := logrus.New()
			logger.SetOutput(ioutil.Discard)
			cloudRunner = NewYipCloudInitRunner(logger, runner)
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == cmdFail {
					return []byte{}, errors.New("command error")
				}
				switch cmd {
				case "parted":
					return []byte(printOutput), nil
				case "blkid":
					if args[0] == "/tmp/device3" {
						return []byte("ext4"), nil
					}
					if args[0] == "--label" && args[1] == "DEV_LABEL" {
						return []byte("/tmp/device"), nil
					}
					return []byte{}, nil
				case "lsblk":
					if args[0] == "-ltnpo" {
						return []byte(fmt.Sprintf("/tmp/device%d part", partNum)), nil
					}
					return []byte(`{"blockdevices":[{"label":"DEV_LABEL","size":1,"partlabel":"pfake", "pkname": "/tmp/device"}]}`), nil
				default:
					return []byte{}, nil
				}
			}
		})
		AfterEach(func() {
			afs.Remove("/tmp/device")
		})
		It("Does nothing if no changes are defined", func() {
			fs, cleanup, _ := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/layout.yaml": `
stages:
  test:
  - name: Nothing to do
    layout:
  - name: Empty device, does nothing
    layout:
      device:
        label: ""
        path: ""
  - name: Defined device without partitions, does nothing
    layout:
      device:
        path: /tmp/device
  - name: Defined already existing partition, does nothing
    layout:
      device:
        label: DEV_LABEL
      add_partitions:
      - fsLabel: DEV_LABEL
        pLabel: partLabel
`,
			})
			defer cleanup()
			cloudRunner.SetFs(fs)
			Expect(cloudRunner.Run("test", "/some/yip")).To(BeNil())
		})
		It("Expands last partition on a MSDOS disk", func() {
			partNum = 3
			fs, cleanup, _ := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/layout.yaml": `
stages:
  test:
  - name: Expanding last partition
    layout:
      device:
        path: /tmp/device
      expand_partition:
        size: 0
`,
			})
			defer cleanup()
			cloudRunner.SetFs(fs)
			Expect(cloudRunner.Run("test", "/some/yip")).To(BeNil())
		})
		It("Adds a partition on a MSDOS disk", func() {
			partNum = 4
			fs, cleanup, _ := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/layout.yaml": `
stages:
  test:
  - name: Adding new partition
    layout:
      device:
        path: /tmp/device
      add_partitions: 
      - fsLabel: SOMELABEL
        pLabel: somelabel
`,
			})
			defer cleanup()
			cloudRunner.SetFs(fs)
			Expect(cloudRunner.Run("test", "/some/yip")).To(BeNil())
		})
		It("Fails to add a partition on a MSDOS disk", func() {
			cmdFail = "mkfs.ext4"
			partNum = 4
			fs, cleanup, _ := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/layout.yaml": `
stages:
  test:
  - name: Adding new partition
    layout:
      device:
        path: /tmp/device
      add_partitions: 
      - fsLabel: SOMELABEL
        pLabel: somelabel
`,
			})
			defer cleanup()
			cloudRunner.SetFs(fs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
		It("Fails to expand last partition", func() {
			partNum = 3
			cmdFail = "resize2fs"
			fs, cleanup, _ := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/layout.yaml": `
stages:
  test:
  - name: Expanding last partition
    layout:
      device:
        path: /tmp/device
      expand_partition:
        size: 0
`,
			})
			defer cleanup()
			cloudRunner.SetFs(fs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
		It("Fails to find device by path", func() {
			fs, cleanup, _ := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/layout.yaml": `
stages:
  test:
  - name: Missing device path
    layout:
      device:
        path: /whatever
`,
			})
			defer cleanup()
			cloudRunner.SetFs(fs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
		It("Fails to find device by label", func() {
			fs, cleanup, _ := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/layout.yaml": `
stages:
  test:
  - name: Missing device label
    layout:
      device:
        label: IM_NOT_THERE
`,
			})
			defer cleanup()
			cloudRunner.SetFs(fs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
	})
})
