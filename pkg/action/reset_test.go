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

package action_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaypipes/ghw/pkg/block"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-toolkit/pkg/action"
	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var _ = Describe("Reset action tests", func() {
	var config *v1.RunConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var mounter *v1mock.ErrorMounter
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var cloudInit *v1mock.FakeCloudInitRunner
	var extractor *v1mock.FakeImageExtractor
	var cleanup func()
	var memLog *bytes.Buffer
	var ghwTest v1mock.GhwMock

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		extractor = v1mock.NewFakeImageExtractor(logger)
		var err error
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())

		cloudInit = &v1mock.FakeCloudInitRunner{}
		config = conf.NewRunConfig(
			conf.WithFs(fs),
			conf.WithRunner(runner),
			conf.WithLogger(logger),
			conf.WithMounter(mounter),
			conf.WithSyscall(syscall),
			conf.WithClient(client),
			conf.WithCloudInitRunner(cloudInit),
			conf.WithImageExtractor(extractor),
		)
	})

	AfterEach(func() { cleanup() })

	Describe("Reset Action", Label("reset"), func() {
		var spec *v1.ResetSpec
		var reset *action.ResetAction
		var cmdFail, bootedFrom string
		var err error
		BeforeEach(func() {

			Expect(err).ShouldNot(HaveOccurred())
			cmdFail = ""
			recoveryImg := filepath.Join(constants.RunningStateDir, "cOS", constants.RecoveryImgFile)
			err = utils.MkdirAll(fs, filepath.Dir(recoveryImg), constants.DirPerm)
			Expect(err).To(BeNil())
			_, err = fs.Create(recoveryImg)
			Expect(err).To(BeNil())

			mainDisk := block.Disk{
				Name: "device",
				Partitions: []*block.Partition{
					{
						Name:            "device1",
						FilesystemLabel: "COS_GRUB",
						Type:            "ext4",
					},
					{
						Name:            "device2",
						FilesystemLabel: "COS_STATE",
						Type:            "ext4",
					},
					{
						Name:            "device3",
						FilesystemLabel: "COS_PERSISTENT",
						Type:            "ext4",
					},
					{
						Name:            "device4",
						FilesystemLabel: "COS_OEM",
						Type:            "ext4",
					},
					{
						Name:            "device5",
						FilesystemLabel: "COS_RECOVERY",
						Type:            "ext4",
					},
				},
			}
			ghwTest = v1mock.GhwMock{}
			ghwTest.AddDisk(mainDisk)
			ghwTest.CreateDevices()

			fs.Create(constants.EfiDevice)
			bootedFrom = constants.RecoveryImgFile
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == cmdFail {
					return []byte{}, errors.New("Command failed")
				}
				switch cmd {
				case "cat":
					return []byte(bootedFrom), nil
				default:
					return []byte{}, nil
				}
			}

			spec, err = conf.NewResetSpec(config.Config)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(spec.Active.Source.IsEmpty()).To(BeFalse())

			spec.Active.Size = 16

			grubCfg := filepath.Join(constants.WorkingImgDir, spec.GrubConf)
			err = utils.MkdirAll(fs, filepath.Dir(grubCfg), constants.DirPerm)
			Expect(err).To(BeNil())
			_, err = fs.Create(grubCfg)
			Expect(err).To(BeNil())

			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmdFail == cmd {
					return []byte{}, errors.New("Command failed")
				}
				if cmd == "grub2-editenv" && args[1] == "set" {
					f, err := fs.OpenFile(args[0], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					Expect(err).To(BeNil())

					_, err = f.Write([]byte(fmt.Sprintf("%s\n", args[2])))
					Expect(err).To(BeNil())
				}
				return []byte{}, nil
			}
			reset = action.NewResetAction(config, spec)
		})

		AfterEach(func() {
			ghwTest.Clean()
		})

		It("Successfully resets on non-squashfs recovery", func() {
			config.Reboot = true
			Expect(reset.Run()).To(BeNil())
			Expect(runner.IncludesCmds([][]string{{"reboot", "-f"}}))
		})
		It("Successfully resets on non-squashfs recovery including persistent data", func() {
			config.PowerOff = true
			spec.FormatPersistent = true
			spec.FormatOEM = true
			Expect(reset.Run()).To(BeNil())
			Expect(runner.IncludesCmds([][]string{{"poweroff", "-f"}}))
		})
		It("Successfully resets from a squashfs recovery image", Label("channel"), func() {
			err := utils.MkdirAll(config.Fs, constants.ISOBaseTree, constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			spec.Active.Source = v1.NewDirSrc(constants.ISOBaseTree)
			Expect(reset.Run()).To(BeNil())
		})
		It("Successfully resets despite having errors on hooks", func() {
			cloudInit.Error = true
			Expect(reset.Run()).To(BeNil())
		})
		It("Successfully writes GRUB labels to oem_env file", func() {
			Expect(reset.Run()).To(BeNil())

			actualBytes, err := fs.ReadFile(filepath.Join(constants.StateDir, "grub_oem_env"))
			Expect(err).To(BeNil())

			expected := map[string]string{
				"state_label":        "COS_STATE",
				"active_label":       "COS_ACTIVE",
				"passive_label":      "COS_PASSIVE",
				"recovery_label":     "COS_RECOVERY",
				"system_label":       "COS_SYSTEM",
				"oem_label":          "COS_OEM",
				"persistent_label":   "COS_PERSISTENT",
				"default_menu_entry": "Elemental",
			}

			lines := strings.Split(string(actualBytes), "\n")

			By(string(actualBytes))

			Expect(len(lines)).To(Equal(len(expected)))

			for _, line := range lines {
				if line == "" {
					continue
				}

				split := strings.SplitN(line, "=", 2)

				Expect(split[1]).To(Equal(expected[split[0]]))
			}
		})
		It("Successfully resets from a docker image", Label("docker"), func() {
			spec.Active.Source = v1.NewDockerSrc("my/image:latest")
			Expect(reset.Run()).To(BeNil())
		})
		It("Successfully resets from a channel package", Label("channel"), func() {
			Expect(reset.Run()).To(BeNil())
		})
		It("Fails installing grub", func() {
			cmdFail = "grub2-install"
			Expect(reset.Run()).NotTo(BeNil())
			Expect(runner.IncludesCmds([][]string{{"grub2-install"}}))
		})
		It("Fails formatting state partition", func() {
			cmdFail = "mkfs.ext4"
			Expect(reset.Run()).NotTo(BeNil())
			Expect(runner.IncludesCmds([][]string{{"mkfs.ext4"}}))
		})
		It("Fails setting the active label on non-squashfs recovery", func() {
			cmdFail = "tune2fs"
			Expect(reset.Run()).NotTo(BeNil())
		})
		It("Fails setting the passive label on squashfs recovery", func() {
			cmdFail = "tune2fs"
			Expect(reset.Run()).NotTo(BeNil())
			Expect(runner.IncludesCmds([][]string{{"tune2fs"}}))
		})
		It("Fails mounting partitions", func() {
			mounter.ErrorOnMount = true
			Expect(reset.Run()).NotTo(BeNil())
		})
		It("Fails unmounting partitions", func() {
			mounter.ErrorOnUnmount = true
			Expect(reset.Run()).NotTo(BeNil())
		})
	})
})
