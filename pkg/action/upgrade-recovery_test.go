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

var _ = Describe("Upgrade Recovery Actions", func() {
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

	BeforeEach(func() {
		runner = mocks.NewFakeRunner()
		syscall = &mocks.FakeSyscall{}
		mounter = mocks.NewFakeMounter()
		client = &mocks.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = types.NewBufferLogger(memLog)
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

	Describe("UpgradeRecovery Action", Label("upgrade-recovery"), func() {
		var spec *types.UpgradeSpec
		var upgradeRecovery *action.UpgradeRecoveryAction
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
			It("Fails if updateInstallState and State partition is not present", func() {
				spec.Partitions.State = nil
				upgradeRecovery, err = action.NewUpgradeRecoveryAction(config, spec, action.WithUpdateInstallState(true))
				Expect(err).To(HaveOccurred())
			})
			It("Fails if updateInstallState and current state can not be loaded", func() {
				spec.State = nil
				upgradeRecovery, err = action.NewUpgradeRecoveryAction(config, spec, action.WithUpdateInstallState(true))
				Expect(err).To(HaveOccurred())
			})
			It("Fails if Recovery partition is not present", func() {
				config.Strict = true
				cloudInit.Error = true
				spec.Partitions.Recovery = nil
				upgradeRecovery, err = action.NewUpgradeRecoveryAction(config, spec)
				Expect(err).To(HaveOccurred())
			})
			It("Successfully upgrades recovery from docker image", Label("docker"), func() {
				recoveryImgPath := filepath.Join(constants.LiveDir, constants.RecoveryImgFile)
				spec := PrepareTestRecoveryImage(config, constants.LiveDir, fs, runner)

				// This should be the old image
				info, err := fs.Stat(recoveryImgPath)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be empty
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.IsDir()).To(BeFalse())
				f, _ := fs.ReadFile(recoveryImgPath)
				Expect(f).To(ContainSubstring("recovery"))

				upgradeRecovery, err = action.NewUpgradeRecoveryAction(config, spec, action.WithUpdateInstallState(true))
				Expect(err).NotTo(HaveOccurred())
				err = upgradeRecovery.Run()
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

				// Create a new spec to load state yaml
				spec, err = conf.NewUpgradeSpec(config.Config)
				Expect(err).NotTo(HaveOccurred())
				// Just a small test to ensure we touched the state file
				Expect(spec.State.Date).ToNot(BeEmpty(), "post-upgrade state should contain a date")
			})
			It("Successfully skips updateInstallState", Label("docker"), func() {
				recoveryImgPath := filepath.Join(constants.LiveDir, constants.RecoveryImgFile)
				spec := PrepareTestRecoveryImage(config, constants.LiveDir, fs, runner)

				// This should be the old image
				info, err := fs.Stat(recoveryImgPath)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be empty
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.IsDir()).To(BeFalse())
				f, _ := fs.ReadFile(recoveryImgPath)
				Expect(f).To(ContainSubstring("recovery"))

				spec.Partitions.State = nil
				spec.State = nil
				upgradeRecovery, err = action.NewUpgradeRecoveryAction(config, spec, action.WithUpdateInstallState(false))
				Expect(err).NotTo(HaveOccurred())
				err = upgradeRecovery.Run()
				Expect(err).ToNot(HaveOccurred())

				// Create a new spec to load state yaml
				spec, err = conf.NewUpgradeSpec(config.Config)
				Expect(err).NotTo(HaveOccurred())
				// Just a small test to ensure we touched the state file
				Expect(spec.State.Date).To(BeEmpty(), "install state files should have not been touched")
			})
		})
		Describe(fmt.Sprintf("Booting from %s", constants.RecoveryLabel), Label("recovery_label"), func() {
			BeforeEach(func() {
				Expect(fs.WriteFile(constants.RecoveryMode, []byte("1"), constants.FilePerm)).To(Succeed())
			})
			It("Fails to upgrade recovery", func() {
				spec, err := conf.NewUpgradeSpec(config.Config)
				Expect(err).NotTo(HaveOccurred())
				_, err = action.NewUpgradeRecoveryAction(config, spec)
				Expect(err).Should(Equal(action.ErrUpgradeRecoveryFromRecovery))
			})
		})
	})
})

func PrepareTestRecoveryImage(config *types.RunConfig, recoveryPath string, fs vfs.FS, runner *mocks.FakeRunner) *types.UpgradeSpec {
	GinkgoHelper()
	// Create installState with squashed recovery
	statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
	installState := &types.InstallState{
		Partitions: map[string]*types.PartitionState{
			constants.RecoveryPartName: {
				FSLabel: constants.RecoveryLabel,
				RecoveryImage: &types.SystemState{
					Label:  constants.SystemLabel,
					FS:     constants.SquashFs,
					Source: types.NewDirSrc("/some/dir"),
				},
			},
		},
	}
	Expect(config.WriteInstallState(installState, statePath, statePath)).ShouldNot(HaveOccurred())

	recoveryImgPath := filepath.Join(recoveryPath, constants.RecoveryImgFile)
	Expect(fs.WriteFile(recoveryImgPath, []byte("recovery"), constants.FilePerm)).ShouldNot(HaveOccurred())

	transitionDir := filepath.Join(recoveryPath, "transition.imgTree")
	Expect(utils.MkdirAll(fs, filepath.Join(transitionDir, "lib/modules/6.6"), constants.DirPerm)).ShouldNot(HaveOccurred())
	bootDir := filepath.Join(transitionDir, "boot")
	Expect(utils.MkdirAll(fs, bootDir, constants.DirPerm)).ShouldNot(HaveOccurred())
	Expect(fs.WriteFile(filepath.Join(bootDir, "vmlinuz-6.6"), []byte("kernel"), constants.FilePerm)).ShouldNot(HaveOccurred())
	Expect(fs.WriteFile(filepath.Join(bootDir, "elemental.initrd-6.6"), []byte("initrd"), constants.FilePerm)).ShouldNot(HaveOccurred())

	spec, err := conf.NewUpgradeSpec(config.Config)
	Expect(err).ShouldNot(HaveOccurred())

	spec.System = types.NewDockerSrc("alpine")
	spec.RecoveryUpgrade = true
	spec.RecoverySystem.Source = spec.System
	spec.RecoverySystem.Size = 16

	runner.SideEffect = func(command string, args ...string) ([]byte, error) {
		if command == "mksquashfs" && args[1] == spec.RecoverySystem.File {
			// create the transition img for squash to fake it
			_, _ = fs.Create(spec.RecoverySystem.File)
		}
		return []byte{}, nil
	}
	config.Runner = runner

	return spec
}
