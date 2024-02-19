/*
   Copyright © 2022 - 2024 SUSE LLC

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
	"fmt"
	"path/filepath"

	"github.com/jaypipes/ghw/pkg/block"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	conf "github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	v2mock "github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ = Describe("Reset action tests", func() {
	var config *v2.RunConfig
	var runner *v2mock.FakeRunner
	var fs vfs.FS
	var logger v2.Logger
	var mounter *v2mock.FakeMounter
	var syscall *v2mock.FakeSyscall
	var client *v2mock.FakeHTTPClient
	var cloudInit *v2mock.FakeCloudInitRunner
	var extractor *v2mock.FakeImageExtractor
	var cleanup func()
	var memLog *bytes.Buffer
	var ghwTest v2mock.GhwMock
	var bootloader *v2mock.FakeBootloader

	BeforeEach(func() {
		runner = v2mock.NewFakeRunner()
		syscall = &v2mock.FakeSyscall{}
		mounter = v2mock.NewFakeMounter()
		client = &v2mock.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = v2.NewBufferLogger(memLog)
		extractor = v2mock.NewFakeImageExtractor(logger)
		var err error
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())

		cloudInit = &v2mock.FakeCloudInitRunner{}
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
		var spec *v2.ResetSpec
		var reset *action.ResetAction
		var cmdFail, bootedFrom string
		var err error
		BeforeEach(func() {
			cmdFail = ""
			recoveryImg := filepath.Join(constants.RunningStateDir, constants.RecoveryImgFile)
			err = utils.MkdirAll(fs, filepath.Dir(recoveryImg), constants.DirPerm)
			Expect(err).To(BeNil())
			_, err = fs.Create(recoveryImg)
			Expect(err).To(BeNil())

			bootloader = &v2mock.FakeBootloader{}

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
			ghwTest = v2mock.GhwMock{}
			ghwTest.AddDisk(mainDisk)
			ghwTest.CreateDevices()

			fs.Create(constants.EfiDevice)
			bootedFrom = constants.RecoveryImgFile
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == cmdFail {
					return []byte{}, fmt.Errorf("Command '%s' failed", cmd)
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
			Expect(spec.System.IsEmpty()).To(BeFalse())

			loopCfg, ok := config.Snapshotter.Config.(*v2.LoopDeviceConfig)
			Expect(ok).To(BeTrue())
			loopCfg.Size = 16

			grubCfg := filepath.Join(constants.WorkingImgDir, constants.GrubCfgPath, constants.GrubCfg)
			err = utils.MkdirAll(fs, filepath.Dir(grubCfg), constants.DirPerm)
			Expect(err).To(BeNil())
			_, err = fs.Create(grubCfg)
			Expect(err).To(BeNil())

			reset, err = action.NewResetAction(config, spec, action.WithResetBootloader(bootloader))
			Expect(err).NotTo(HaveOccurred())
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
			spec.System = v2.NewDirSrc(constants.ISOBaseTree)
			Expect(reset.Run()).To(BeNil())
		})
		It("Successfully resets despite having errors on hooks", func() {
			cloudInit.Error = true
			Expect(reset.Run()).To(BeNil())
		})
		It("Successfully resets from a docker image", Label("docker"), func() {
			spec.System = v2.NewDockerSrc("my/image:latest")
			Expect(reset.Run()).To(BeNil())
		})
		It("Successfully resets from a channel package", Label("channel"), func() {
			Expect(reset.Run()).To(BeNil())
		})
		It("Fails setting the persistent grub variables", func() {
			bootloader.ErrorSetPersistentVariables = true
			err = reset.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("setting persistent variables"))
		})
		It("Fails setting the default grub entry", func() {
			bootloader.ErrorSetDefaultEntry = true
			err = reset.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("setting default entry"))
		})
		It("Fails installing grub", func() {
			bootloader.ErrorInstall = true
			Expect(reset.Run()).NotTo(BeNil())
		})
		It("Fails formatting state partition", func() {
			cmdFail = "mkfs.ext4"
			err = reset.Run()
			Expect(err).To(HaveOccurred())
			Expect(runner.IncludesCmds([][]string{{"mkfs.ext4"}}))
			Expect(err.Error()).To(ContainSubstring("Command 'mkfs.ext4' failed"))
		})
		It("Fails mounting partitions", func() {
			mounter.ErrorOnMount = true
			err = reset.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("mount error"))
		})
		It("Fails unmounting partitions", func() {
			mounter.ErrorOnUnmount = true
			err = reset.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unmount error"))
		})
	})
})
