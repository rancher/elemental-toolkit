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

	"github.com/rancher/elemental-toolkit/pkg/action"
	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var _ = Describe("Upgrade Recovery Actions", func() {
	var config *v1.RunConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var mounter *v1mock.FakeMounter
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
		mounter = v1mock.NewFakeMounter()
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
		Expect(config.Sanitize()).To(Succeed())
	})

	AfterEach(func() { cleanup() })

	Describe("UpgradeRecovery Action", Label("upgrade-recovery"), func() {
		var spec *v1.UpgradeSpec
		var upgradeRecovery *action.UpgradeRecoveryAction
		var memLog *bytes.Buffer

		BeforeEach(func() {
			memLog = &bytes.Buffer{}
			logger = v1.NewBufferLogger(memLog)
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
			ghwTest = v1mock.GhwMock{}
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

				spec.System = v1.NewDockerSrc("alpine")
				loopCfg, ok := config.Snapshotter.Config.(*v1.LoopDeviceConfig)
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
				spec := PrepareTestRecoveryImage(config, recoveryImgPath, fs, runner)

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
				spec := PrepareTestRecoveryImage(config, recoveryImgPath, fs, runner)

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

func PrepareTestRecoveryImage(config *v1.RunConfig, recoveryImgPath string, fs vfs.FS, runner *v1mock.FakeRunner) *v1.UpgradeSpec {
	GinkgoHelper()
	// Create installState with squashed recovery
	statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
	installState := &v1.InstallState{
		Partitions: map[string]*v1.PartitionState{
			constants.RecoveryPartName: {
				FSLabel: constants.RecoveryLabel,
				RecoveryImage: &v1.SystemState{
					Label:  constants.SystemLabel,
					FS:     constants.SquashFs,
					Source: v1.NewDirSrc("/some/dir"),
				},
			},
		},
	}
	Expect(config.WriteInstallState(installState, statePath, statePath)).ShouldNot(HaveOccurred())

	Expect(fs.WriteFile(recoveryImgPath, []byte("recovery"), constants.FilePerm)).ShouldNot(HaveOccurred())

	spec, err := conf.NewUpgradeSpec(config.Config)
	Expect(err).ShouldNot(HaveOccurred())

	spec.System = v1.NewDockerSrc("alpine")
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
