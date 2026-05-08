/*
Copyright © 2022 - 2026 SUSE LLC

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
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/jaypipes/ghw/pkg/block"
	"github.com/rancher/yip/pkg/schema"

	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/rancher/elemental-toolkit/v2/pkg/cloudinit"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"

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
		var logger types.Logger
		var buffer *bytes.Buffer
		BeforeEach(func() {
			buffer = bytes.NewBuffer([]byte{})
			logger = types.NewBufferLogger(buffer)
			logger.SetLevel(types.DebugLevel())
		})

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

			runner := NewYipCloudInitRunner(logger, &types.RealRunner{}, fs)

			err = runner.Run("test", "/some/yip")
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := io.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("baz"))
		})
	})
	Describe("writing yaml files", func() {
		var fs *vfst.TestFS
		var logger types.Logger
		var cleanup func()
		var err error
		var yipRunner *YipCloudInitRunner
		var tempDir string

		BeforeEach(func() {
			logger = types.NewNullLogger()
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			yipRunner = NewYipCloudInitRunner(logger, &types.RealRunner{}, fs)
			tempDir = fs.TempDir()
		})

		AfterEach(func() {
			cleanup()
		})

		It("produces yaml files and is capable to executed them", func() {
			conf := &schema.YipConfig{
				Name: "Example cloud-config file",
				Stages: map[string][]schema.Stage{
					"hello-test": {
						schema.Stage{
							Name: "Some step",
							Commands: []string{
								"echo 'Hello world' > " + tempDir + "/output/hello",
							},
						},
					},
				},
			}
			Expect(utils.MkdirAll(fs, "/output", constants.DirPerm)).To(Succeed())
			Expect(yipRunner.CloudInitFileRender("/conf/exmaple.yaml", conf)).To(Succeed())
			Expect(yipRunner.Run("hello-test", "/conf")).To(Succeed())
			data, err := fs.ReadFile("/output/hello")
			Expect(err).To(BeNil())
			Expect(string(data)).To(Equal("Hello world\n"))
		})
		It("fails writing a file on read only filesystem", func() {
			conf := &schema.YipConfig{
				Name: "Example cloud-config file",
				Stages: map[string][]schema.Stage{
					"hello-test": {
						schema.Stage{
							Name: "Some step",
							Commands: []string{
								"echo 'Hello world' > " + tempDir + "/output/hello",
							},
						},
					},
				},
			}
			Expect(utils.MkdirAll(fs, "/output", constants.DirPerm)).To(Succeed())
			roFS := vfs.NewReadOnlyFS(fs)
			yipRunner = NewYipCloudInitRunner(logger, &types.RealRunner{}, roFS)
			Expect(yipRunner.CloudInitFileRender("/conf/exmaple.yaml", conf)).NotTo(Succeed())
		})
	})
	Describe("layout plugin execution", func() {
		var runner *mocks.FakeRunner
		var afs *vfst.TestFS
		var device, cmdFail string
		var partNum int
		var cleanup func()
		logger := types.NewNullLogger()
		BeforeEach(func() {
			afs, cleanup, _ = vfst.NewTestFS(nil)
			err := utils.MkdirAll(afs, "/some/yip", constants.DirPerm)
			Expect(err).To(BeNil())
			_ = utils.MkdirAll(afs, "/dev", constants.DirPerm)
			device = "/dev/device"
			_, err = afs.Create(device)
			Expect(err).To(BeNil())

			runner = mocks.NewFakeRunner()

			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == cmdFail {
					return []byte{}, errors.New("command error")
				}
				switch cmd {
				case "parted":
					return []byte(printOutput), nil
				default:
					return []byte{}, nil
				}
			}
		})
		AfterEach(func() {
			cleanup()
		})
		It("Does nothing if no changes are defined", func() {
			err := afs.WriteFile("/some/yip/layout.yaml", []byte(fmt.Sprintf(`
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
        path: %s
  - name: Defined already existing partition, does nothing
    layout:
      device:
        label: DEV_LABEL
      add_partitions:
      - fsLabel: DEV_LABEL
        pLabel: partLabel
`, device)), constants.FilePerm)
			Expect(err).To(BeNil())
			ghwTest := mocks.GhwMock{}
			disk := block.Disk{Name: "device", Partitions: []*block.Partition{
				{
					Name:            "device1",
					FilesystemLabel: "DEV_LABEL",
				},
			}}
			ghwTest.AddDisk(disk)
			ghwTest.CreateDevices()
			defer ghwTest.Clean()
			cloudRunner := NewYipCloudInitRunner(logger, runner, afs)
			Expect(cloudRunner.Run("test", "/some/yip")).To(BeNil())
		})
		It("Expands last partition on a MSDOS disk", func() {
			partNum = 3
			_, err := afs.Create(fmt.Sprintf("%s%d", device, partNum))
			Expect(err).To(BeNil())
			err = afs.WriteFile("/some/yip/layout.yaml", []byte(fmt.Sprintf(`
stages:
  test:
  - name: Expanding last partition
    layout:
      device:
        path: %s
      expand_partition:
        size: 0
`, device)), constants.FilePerm)
			Expect(err).To(BeNil())
			ghwTest := mocks.GhwMock{}
			disk := block.Disk{Name: "device", Partitions: []*block.Partition{
				{
					Name: fmt.Sprintf("device%d", partNum),
					Type: "ext4",
				},
			}}
			ghwTest.AddDisk(disk)
			ghwTest.CreateDevices()
			defer ghwTest.Clean()
			cloudRunner := NewYipCloudInitRunner(logger, runner, afs)
			Expect(cloudRunner.Run("test", "/some/yip")).To(BeNil())
		})
		It("Adds a partition on a MSDOS disk", func() {
			partNum = 4
			_, err := afs.Create(fmt.Sprintf("%s%d", device, partNum))
			Expect(err).To(BeNil())
			err = afs.WriteFile("/some/yip/layout.yaml", []byte(fmt.Sprintf(`
stages:
  test:
  - name: Adding new partition
    layout:
      device:
        path: %s
      add_partitions: 
      - fsLabel: SOMELABEL
        pLabel: somelabel
`, device)), constants.FilePerm)
			Expect(err).To(BeNil())
			cloudRunner := NewYipCloudInitRunner(logger, runner, afs)
			Expect(cloudRunner.Run("test", "/some/yip")).To(BeNil())
		})
		It("Fails to add a partition on a MSDOS disk", func() {
			cmdFail = "mkfs.ext4"
			partNum = 4
			_, err := afs.Create(fmt.Sprintf("%s%d", device, partNum))
			Expect(err).To(BeNil())
			err = afs.WriteFile("/some/yip/layout.yaml", []byte(fmt.Sprintf(`
stages:
  test:
  - name: Adding new partition
    layout:
      device:
        path: %s
      add_partitions: 
      - fsLabel: SOMELABEL
        pLabel: somelabel
`, device)), constants.FilePerm)
			Expect(err).To(BeNil())
			cloudRunner := NewYipCloudInitRunner(logger, runner, afs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
		It("Fails to expand last partition", func() {
			partNum = 3
			cmdFail = "resize2fs"
			_, err := afs.Create(fmt.Sprintf("%s%d", device, partNum))
			Expect(err).To(BeNil())
			err = afs.WriteFile("/some/yip/layout.yaml", []byte(fmt.Sprintf(`
stages:
  test:
  - name: Expanding last partition
    layout:
      device:
        path: %s
      expand_partition:
        size: 0
`, device)), constants.FilePerm)
			Expect(err).To(BeNil())
			cloudRunner := NewYipCloudInitRunner(logger, runner, afs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
		It("Fails to find device by path", func() {
			err := afs.WriteFile("/some/yip/layout.yaml", []byte(`
stages:
  test:
  - name: Missing device path
    layout:
      device:
        path: /whatever
`), constants.FilePerm)
			Expect(err).To(BeNil())
			cloudRunner := NewYipCloudInitRunner(logger, runner, afs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
		It("Fails to find device by label", func() {
			err := afs.WriteFile("/some/yip/layout.yaml", []byte(`
stages:
  test:
  - name: Missing device label
    layout:
      device:
        label: IM_NOT_THERE
`), constants.FilePerm)
			Expect(err).To(BeNil())
			cloudRunner := NewYipCloudInitRunner(logger, runner, afs)
			Expect(cloudRunner.Run("test", "/some/yip")).NotTo(BeNil())
		})
	})
})
