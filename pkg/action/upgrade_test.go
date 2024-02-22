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
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	conf "github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ = Describe("Runtime Actions", func() {
	var config *types.RunConfig
	var runner *mocks.FakeRunner
	var fs vfs.FS
	var logger types.Logger
	var mounter *mocks.FakeMounter
	var syscall *mocks.FakeSyscall
	var client *mocks.FakeHTTPClient
	var cloudInit *mocks.FakeCloudInitRunner
	var extractor *mocks.FakeImageExtractor
	var cleanup func()
	var memLog *bytes.Buffer
	var ghwTest mocks.GhwMock
	var bootloader *mocks.FakeBootloader

	BeforeEach(func() {
		runner = mocks.NewFakeRunner()
		syscall = &mocks.FakeSyscall{}
		mounter = mocks.NewFakeMounter()
		client = &mocks.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = types.NewBufferLogger(memLog)
		bootloader = &mocks.FakeBootloader{}
		extractor = mocks.NewFakeImageExtractor(logger)
		var err error
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())

		cloudInit = &mocks.FakeCloudInitRunner{}
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
		Expect(config.Sanitize()).To(Succeed())
	})

	AfterEach(func() { cleanup() })

	Describe("Upgrade Action", Label("upgrade"), func() {
		var spec *types.UpgradeSpec
		var upgrade *action.UpgradeAction
		var memLog *bytes.Buffer

		BeforeEach(func() {
			memLog = &bytes.Buffer{}
			logger = types.NewBufferLogger(memLog)
			config.Logger = logger
			logger.SetLevel(logrus.DebugLevel)

			// Create paths used by tests
			Expect(utils.MkdirAll(fs, constants.RunningStateDir, constants.DirPerm)).To(Succeed())
			Expect(utils.MkdirAll(fs, constants.LiveDir, constants.DirPerm)).To(Succeed())
			Expect(utils.MkdirAll(fs, filepath.Dir(constants.ActiveMode), constants.DirPerm)).To(Succeed())

			mainDisk := block.Disk{
				Name: "device",
				Partitions: []*block.Partition{
					{
						Name:            "device1",
						FilesystemLabel: "COS_GRUB",
						Type:            "vfat",
						MountPoint:      constants.EfiDir,
					},
					{
						Name:            "device2",
						FilesystemLabel: "COS_STATE",
						Type:            "ext4",
						MountPoint:      constants.RunningStateDir,
					},
					{
						Name:            "device5",
						FilesystemLabel: "COS_RECOVERY",
						Type:            "ext4",
						MountPoint:      constants.LiveDir,
					},
					{
						Name:            "device6",
						FilesystemLabel: "COS_OEM",
						Type:            "ext4",
					},
				},
			}
			ghwTest = mocks.GhwMock{}
			ghwTest.AddDisk(mainDisk)
			ghwTest.CreateDevices()
		})
		AfterEach(func() {
			ghwTest.Clean()
		})
		Describe("Booting from active system", func() {
			var err error
			BeforeEach(func() {
				Expect(fs.WriteFile(constants.ActiveMode, []byte("1"), constants.FilePerm)).To(Succeed())

				spec, err = conf.NewUpgradeSpec(config.Config)
				Expect(err).ShouldNot(HaveOccurred())

				spec.System = types.NewDockerSrc("alpine")
				loopCfg, ok := config.Snapshotter.Config.(*types.LoopDeviceConfig)
				Expect(ok).To(BeTrue())
				loopCfg.Size = 16

				err = utils.MkdirAll(config.Fs, filepath.Join(constants.WorkingImgDir, "etc"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())

				err = fs.WriteFile(
					filepath.Join(constants.WorkingImgDir, "etc", "os-release"),
					[]byte("GRUB_ENTRY_NAME=TESTOS"),
					constants.FilePerm,
				)
				Expect(err).ShouldNot(HaveOccurred())

				mounter.Mount("device2", constants.RunningStateDir, "auto", []string{"ro"})
			})
			AfterEach(func() {

			})
			It("Fails if some hook fails and strict is set", func() {
				config.Strict = true
				cloudInit.Error = true
				upgrade, err = action.NewUpgradeAction(config, spec, action.WithUpgradeBootloader(bootloader))
				Expect(err).NotTo(HaveOccurred())
				err := upgrade.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cloud init"))
			})
			It("Fails setting the grub labels", func() {
				bootloader.ErrorSetPersistentVariables = true
				upgrade, err = action.NewUpgradeAction(config, spec, action.WithUpgradeBootloader(bootloader))
				Expect(err).NotTo(HaveOccurred())
				err := upgrade.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("setting persistent variables"))
			})
			It("Fails setting the grub default entry", func() {
				bootloader.ErrorSetDefaultEntry = true
				upgrade, err = action.NewUpgradeAction(config, spec, action.WithUpgradeBootloader(bootloader))
				Expect(err).NotTo(HaveOccurred())
				err := upgrade.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("setting default entry"))
			})
			It("Successfully upgrades from docker image", func() {
				Expect(mocks.FakeLoopDeviceSnapshotsStatus(fs, constants.RunningStateDir, 2)).To(Succeed())
				// Create installState with previous install state
				statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
				installState := &types.InstallState{
					Partitions: map[string]*types.PartitionState{
						constants.StatePartName: {
							FSLabel: "COS_STATE",
							Snapshots: map[int]*types.SystemState{
								2: {
									Source: types.NewDockerSrc("some/image:v2"),
									Digest: "somehash2",
									Active: true,
								},
								1: {
									Source: types.NewDockerSrc("some/image:v1"),
									Digest: "somehash",
								},
							},
						},
					},
				}
				err = config.WriteInstallState(installState, statePath, statePath)
				Expect(err).ShouldNot(HaveOccurred())

				// Limit maximum snapshots to 2
				config.Snapshotter.MaxSnaps = 2

				// Create a new spec to load state yaml
				spec, err = conf.NewUpgradeSpec(config.Config)

				spec.System = types.NewDockerSrc("alpine")
				upgrade, err = action.NewUpgradeAction(config, spec)
				Expect(err).NotTo(HaveOccurred())
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Writes filesystem labels to GRUB oem env file
				grubOEMEnv := filepath.Join(spec.Partitions.EFI.MountPoint, constants.GrubOEMEnv)
				Expect(runner.IncludesCmds(
					[][]string{{"grub2-editenv", grubOEMEnv, "set", "passive_snaps=2"}},
				)).To(Succeed())

				// Expect snapshot 2 and 3 to be there and 1 deleted
				ok, _ := utils.Exists(fs, filepath.Join(constants.RunningStateDir, ".snapshots/3/snapshot.img"))
				Expect(ok).To(BeTrue())
				ok, _ = utils.Exists(fs, filepath.Join(constants.RunningStateDir, ".snapshots/2/snapshot.img"))
				Expect(ok).To(BeTrue())
				ok, _ = utils.Exists(fs, filepath.Join(constants.RunningStateDir, ".snapshots/1/snapshot.img"))
				Expect(ok).To(BeFalse())

				// An upgraded state yaml file should exist
				state, err := config.LoadInstallState()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(state.Partitions[constants.StatePartName].Snapshots[3].Active).
					To(BeTrue())
				Expect(state.Partitions[constants.StatePartName].Snapshots[2].Active).
					To(BeFalse())
				Expect(state.Partitions[constants.StatePartName].Snapshots[3].Digest).
					To(Equal(mocks.FakeDigest))
				Expect(state.Partitions[constants.StatePartName].Snapshots[3].Source.String()).
					To(Equal("oci://alpine:latest"))
				Expect(state.Partitions[constants.StatePartName].Snapshots[2].Source.String()).
					To(Equal("oci://some/image:v2"))
				// Snapshot 1 was deleted
				Expect(state.Partitions[constants.StatePartName].Snapshots[1]).
					To(BeNil())
			})
			It("Successfully reboots after upgrade from docker image", func() {
				Expect(mocks.FakeLoopDeviceSnapshotsStatus(fs, constants.RunningStateDir, 1)).To(Succeed())
				spec.System = types.NewDockerSrc("alpine")
				config.Reboot = true
				upgrade, err = action.NewUpgradeAction(config, spec)
				Expect(err).NotTo(HaveOccurred())
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Expect snapshot 2 to be there
				ok, _ := utils.Exists(fs, filepath.Join(constants.RunningStateDir, ".snapshots/2/snapshot.img"))
				Expect(ok).To(BeTrue())

				// Expect reboot executed
				Expect(runner.IncludesCmds([][]string{{"reboot", "-f"}})).To(BeNil())
			})
			It("Successfully powers off after upgrade from docker image", func() {
				Expect(mocks.FakeLoopDeviceSnapshotsStatus(fs, constants.RunningStateDir, 1)).To(Succeed())
				spec.System = types.NewDockerSrc("alpine")
				config.PowerOff = true
				upgrade, err = action.NewUpgradeAction(config, spec)
				Expect(err).NotTo(HaveOccurred())
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Expect snapshot 2 to be there
				ok, _ := utils.Exists(fs, filepath.Join(constants.RunningStateDir, ".snapshots/2/snapshot.img"))
				Expect(ok).To(BeTrue())

				// Expect poweroff executed
				Expect(runner.IncludesCmds([][]string{{"poweroff", "-f"}})).To(BeNil())
			})
			It("Successfully upgrades recovery from docker image", Label("docker"), func() {
				recoveryImgPath := filepath.Join(constants.LiveDir, constants.RecoveryImgFile)
				spec := PrepareTestRecoveryImage(config, recoveryImgPath, fs, runner)

				// This should be the old image
				info, err := fs.Stat(recoveryImgPath)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be empty
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.IsDir()).To(BeFalse())
				f, _ := fs.ReadFile(recoveryImgPath)
				Expect(f).To(ContainSubstring("recovery"))

				upgrade, err = action.NewUpgradeAction(config, spec)
				Expect(err).NotTo(HaveOccurred())
				err = upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// This should be the new image
				info, err = fs.Stat(recoveryImgPath)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be empty
				Expect(info.Size()).To(BeNumerically("==", 0))
				Expect(info.IsDir()).To(BeFalse())
				f, _ = fs.ReadFile(recoveryImgPath)
				Expect(f).ToNot(ContainSubstring("recovery"))

				// Transition squash should not exist
				info, err = fs.Stat(spec.RecoverySystem.File)
				Expect(err).To(HaveOccurred())
			})
		})
		Describe(fmt.Sprintf("Booting from %s", constants.RecoveryLabel), Label("recovery_label"), func() {
			BeforeEach(func() {
				Expect(fs.WriteFile(constants.RecoveryMode, []byte("1"), constants.FilePerm)).To(Succeed())
			})
			It("Fails to upgrade recovery", func() {
				spec, err := conf.NewUpgradeSpec(config.Config)
				Expect(err).NotTo(HaveOccurred())
				spec.RecoveryUpgrade = true
				_, err = action.NewUpgradeAction(config, spec)
				Expect(err).Should(Equal(action.ErrUpgradeRecoveryFromRecovery))
			})
		})
	})
})
