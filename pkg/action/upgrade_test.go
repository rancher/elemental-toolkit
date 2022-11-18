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

package action_test

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/jaypipes/ghw/pkg/block"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/elemental-cli/pkg/action"
	conf "github.com/rancher/elemental-cli/pkg/config"
	"github.com/rancher/elemental-cli/pkg/constants"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
	v1mock "github.com/rancher/elemental-cli/tests/mocks"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
)

var _ = Describe("Runtime Actions", func() {
	var config *v1.RunConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var mounter *v1mock.ErrorMounter
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var cloudInit *v1mock.FakeCloudInitRunner
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
		)
	})

	AfterEach(func() { cleanup() })

	Describe("Upgrade Action", Label("upgrade"), func() {
		var spec *v1.UpgradeSpec
		var upgrade *action.UpgradeAction
		var memLog *bytes.Buffer
		var l *v1mock.FakeLuet
		activeImg := fmt.Sprintf("%s/cOS/%s", constants.RunningStateDir, constants.ActiveImgFile)
		passiveImg := fmt.Sprintf("%s/cOS/%s", constants.RunningStateDir, constants.PassiveImgFile)
		recoveryImgSquash := fmt.Sprintf("%s/cOS/%s", constants.LiveDir, constants.RecoverySquashFile)
		recoveryImg := fmt.Sprintf("%s/cOS/%s", constants.LiveDir, constants.RecoveryImgFile)

		BeforeEach(func() {
			memLog = &bytes.Buffer{}
			logger = v1.NewBufferLogger(memLog)
			config.Logger = logger
			logger.SetLevel(logrus.DebugLevel)
			l = &v1mock.FakeLuet{}
			config.Luet = l

			// Create paths used by tests
			utils.MkdirAll(fs, fmt.Sprintf("%s/cOS", constants.RunningStateDir), constants.DirPerm)
			utils.MkdirAll(fs, fmt.Sprintf("%s/cOS", constants.LiveDir), constants.DirPerm)

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
						MountPoint:      constants.RunningStateDir,
					},
					{
						Name:            "loop0",
						FilesystemLabel: "COS_ACTIVE",
						Type:            "ext4",
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
		Describe(fmt.Sprintf("Booting from %s", constants.ActiveLabel), Label("active_label"), func() {
			var err error
			BeforeEach(func() {
				spec, err = conf.NewUpgradeSpec(config.Config)
				Expect(err).ShouldNot(HaveOccurred())

				spec.Active.Source = v1.NewChannelSrc("system/cos-config")
				spec.Active.Size = 16

				err = utils.MkdirAll(config.Fs, filepath.Join(spec.Active.MountPoint, "etc"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())

				err = fs.WriteFile(
					filepath.Join(spec.Active.MountPoint, "etc", "os-release"),
					[]byte("GRUB_ENTRY_NAME=TESTOS"),
					constants.FilePerm,
				)
				Expect(err).ShouldNot(HaveOccurred())

				runner.SideEffect = func(command string, args ...string) ([]byte, error) {
					if command == "cat" && args[0] == "/proc/cmdline" {
						return []byte(constants.ActiveLabel), nil
					}
					if command == "mv" && args[0] == "-f" && args[1] == activeImg && args[2] == passiveImg {
						// we doing backup, do the "move"
						source, _ := fs.ReadFile(activeImg)
						_ = fs.WriteFile(passiveImg, source, constants.FilePerm)
						_ = fs.RemoveAll(activeImg)
					}
					if command == "mv" && args[0] == "-f" && args[1] == spec.Active.File && args[2] == activeImg {
						// we doing the image substitution, do the "move"
						source, _ := fs.ReadFile(spec.Active.File)
						_ = fs.WriteFile(activeImg, source, constants.FilePerm)
						_ = fs.RemoveAll(spec.Active.File)
					}
					return []byte{}, nil
				}
				config.Runner = runner
				// Create fake active/passive files
				_ = fs.WriteFile(activeImg, []byte("active"), constants.FilePerm)
				_ = fs.WriteFile(passiveImg, []byte("passive"), constants.FilePerm)
				// Mount state partition as it is expected to be mounted when booting from active
				mounter.Mount("device2", constants.RunningStateDir, "auto", []string{"ro"})
			})
			AfterEach(func() {
				_ = fs.RemoveAll(activeImg)
				_ = fs.RemoveAll(passiveImg)
			})
			It("Fails if some hook fails and strict is set", func() {
				runner.SideEffect = func(command string, args ...string) ([]byte, error) {
					if command == "cat" && args[0] == "/proc/cmdline" {
						return []byte(constants.ActiveLabel), nil
					}
					return []byte{}, nil
				}
				config.Strict = true
				cloudInit.Error = true
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).To(HaveOccurred())
				// Make sure is a cloud init error!
				Expect(err.Error()).To(ContainSubstring("cloud init"))
			})
			It("Successfully upgrades from docker image", Label("docker"), func() {
				spec.Active.Source = v1.NewDockerSrc("alpine")
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check luet was called to unpack a docker image
				Expect(l.UnpackCalled()).To(BeTrue())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be the config.ImgSize as its truncated from the upgrade
				Expect(info.Size()).To(BeNumerically("==", int64(spec.Active.Size*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be a tiny image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(spec.Active.Size*1024*1024)))
				f, _ := fs.ReadFile(passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(spec.Active.File)
				Expect(err).To(HaveOccurred())
			})
			It("Successfully reboots after upgrade from docker image", Label("docker"), func() {
				spec.Active.Source = v1.NewDockerSrc("alpine")
				config.Reboot = true
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check luet was called to unpack a docker image
				Expect(l.UnpackCalled()).To(BeTrue())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be the config.ImgSize as its truncated from the upgrade
				Expect(info.Size()).To(BeNumerically("==", int64(spec.Active.Size*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be a tiny image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(spec.Active.Size*1024*1024)))
				f, _ := fs.ReadFile(passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(spec.Active.File)
				Expect(err).To(HaveOccurred())
				Expect(runner.IncludesCmds([][]string{{"reboot", "-f"}})).To(BeNil())
			})
			It("Successfully powers off after upgrade from docker image", Label("docker"), func() {
				spec.Active.Source = v1.NewDockerSrc("alpine")
				config.PowerOff = true
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check luet was called to unpack a docker image
				Expect(l.UnpackCalled()).To(BeTrue())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be the config.ImgSize as its truncated from the upgrade
				Expect(info.Size()).To(BeNumerically("==", int64(spec.Active.Size*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be a tiny image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(spec.Active.Size*1024*1024)))
				f, _ := fs.ReadFile(passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(spec.Active.File)
				Expect(err).To(HaveOccurred())
				Expect(runner.IncludesCmds([][]string{{"poweroff", "-f"}})).To(BeNil())
			})
			It("Successfully upgrades from directory", Label("directory"), func() {
				dirSrc, _ := utils.TempDir(fs, "", "elementalupgrade")
				// Create the dir on real os as rsync works on the real os
				defer fs.RemoveAll(dirSrc)
				spec.Active.Source = v1.NewDirSrc(dirSrc)
				// create a random file on it
				err := fs.WriteFile(fmt.Sprintf("%s/file.file", dirSrc), []byte("something"), constants.FilePerm)
				Expect(err).ToNot(HaveOccurred())

				upgrade = action.NewUpgradeAction(config, spec)
				err = upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Not much that we can create here as the dir copy was done on the real os, but we do the rest of the ops on a mem one
				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should not be empty
				Expect(info.Size()).To(BeNumerically("==", int64(spec.Active.Size*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be a tiny image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.img
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(spec.Active.Size*1024*1024)))
				f, _ := fs.ReadFile(passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(spec.Active.File)
				Expect(err).To(HaveOccurred())

			})
			It("Successfully upgrades from channel upgrade", Label("channel"), func() {
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Not much that we can create here as the dir copy was done on the real os, but we do the rest of the ops on a mem one
				// This should be the new image
				// Should probably do well in mounting the image and checking contents to make sure everything worked
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should not be empty
				Expect(info.Size()).To(BeNumerically("==", int64(spec.Active.Size*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be an really small image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(spec.Active.Size*1024*1024)))
				f, _ := fs.ReadFile(passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(spec.Active.File)
				Expect(err).To(HaveOccurred())
			})
			It("Successfully upgrades with cosign", Pending, Label("channel", "cosign"), func() {})
			It("Successfully upgrades with mtree", Pending, Label("channel", "mtree"), func() {})
			It("Successfully upgrades with strict", Pending, Label("channel", "strict"), func() {})
		})
		Describe(fmt.Sprintf("Booting from %s", constants.PassiveLabel), Label("passive_label"), func() {
			var err error
			BeforeEach(func() {
				spec, err = conf.NewUpgradeSpec(config.Config)
				Expect(err).ShouldNot(HaveOccurred())

				spec.Active.Source = v1.NewChannelSrc("system/cos-config")
				spec.Active.Size = 16

				err = utils.MkdirAll(config.Fs, filepath.Join(spec.Active.MountPoint, "etc"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())

				err = fs.WriteFile(
					filepath.Join(spec.Active.MountPoint, "etc", "os-release"),
					[]byte("GRUB_ENTRY_NAME=TESTOS"),
					constants.FilePerm,
				)
				Expect(err).ShouldNot(HaveOccurred())

				runner.SideEffect = func(command string, args ...string) ([]byte, error) {
					if command == "cat" && args[0] == "/proc/cmdline" {
						return []byte(constants.PassiveLabel), nil
					}
					if command == "mv" && args[0] == "-f" && args[1] == spec.Active.File && args[2] == activeImg {
						// we doing the image substitution, do the "move"
						source, _ := fs.ReadFile(spec.Active.File)
						_ = fs.WriteFile(activeImg, source, constants.FilePerm)
						_ = fs.RemoveAll(spec.Active.File)
					}
					return []byte{}, nil
				}
				config.Runner = runner
				// Create fake active/passive files
				_ = fs.WriteFile(activeImg, []byte("active"), constants.FilePerm)
				_ = fs.WriteFile(passiveImg, []byte("passive"), constants.FilePerm)
				// Mount state partition as it is expected to be mounted when booting from active
				mounter.Mount("device2", constants.RunningStateDir, "auto", []string{"ro"})
			})
			AfterEach(func() {
				_ = fs.RemoveAll(activeImg)
				_ = fs.RemoveAll(passiveImg)
			})
			It("does not backup active img to passive", Label("docker"), func() {
				spec.Active.Source = v1.NewDockerSrc("alpine")
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should not be empty
				Expect(info.Size()).To(BeNumerically("==", int64(spec.Active.Size*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Passive should have not been touched
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be a tiny image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(spec.Active.Size*1024*1024)))
				f, _ := fs.ReadFile(passiveImg)
				Expect(f).To(ContainSubstring("passive"))

				// Expect transition image to be gone
				_, err = fs.Stat(spec.Active.File)
				Expect(err).To(HaveOccurred())

			})
		})
		Describe(fmt.Sprintf("Booting from %s", constants.RecoveryLabel), Label("recovery_label"), func() {
			Describe("Using squashfs", Label("squashfs"), func() {
				var err error
				BeforeEach(func() {
					// Mount recovery partition as it is expected to be mounted when booting from recovery
					mounter.Mount("device5", constants.LiveDir, "auto", []string{"ro"})
					// Create installState with squashed recovery
					statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
					installState := &v1.InstallState{
						Partitions: map[string]*v1.PartitionState{
							constants.RecoveryPartName: {
								FSLabel: constants.RecoveryLabel,
								Images: map[string]*v1.ImageState{
									constants.RecoveryImgName: {
										FS: constants.SquashFs,
									},
								},
							},
						},
					}
					err = config.WriteInstallState(installState, statePath, statePath)
					Expect(err).ShouldNot(HaveOccurred())
					err = fs.WriteFile(recoveryImgSquash, []byte("recovery"), constants.FilePerm)
					Expect(err).ShouldNot(HaveOccurred())

					spec, err = conf.NewUpgradeSpec(config.Config)
					Expect(err).ShouldNot(HaveOccurred())

					spec.RecoveryUpgrade = true
					spec.Recovery.Source = v1.NewChannelSrc("system/cos-config")
					spec.Recovery.Size = 16

					runner.SideEffect = func(command string, args ...string) ([]byte, error) {
						if command == "cat" && args[0] == "/proc/cmdline" {
							return []byte(constants.RecoveryLabel), nil
						}
						if command == "mksquashfs" && args[1] == spec.Recovery.File {
							// create the transition img for squash to fake it
							_, _ = fs.Create(spec.Recovery.File)
						}
						if command == "mv" && args[0] == "-f" && args[1] == spec.Recovery.File && args[2] == recoveryImgSquash {
							// fake "move"
							f, _ := fs.ReadFile(spec.Recovery.File)
							_ = fs.WriteFile(recoveryImgSquash, f, constants.FilePerm)
							_ = fs.RemoveAll(spec.Recovery.File)
						}
						return []byte{}, nil
					}
					config.Runner = runner
				})
				It("Successfully upgrades recovery from docker image", Label("docker"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := fs.ReadFile(recoveryImgSquash)
					Expect(f).To(ContainSubstring("recovery"))

					spec.Recovery.Source = v1.NewDockerSrc("alpine")
					upgrade = action.NewUpgradeAction(config, spec)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(l.UnpackCalled()).To(BeTrue())

					// This should be the new image
					info, err = fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ = fs.ReadFile(recoveryImgSquash)
					Expect(f).ToNot(ContainSubstring("recovery"))

					// Transition squash should not exist
					info, err = fs.Stat(spec.Recovery.File)
					Expect(err).To(HaveOccurred())

				})
				It("Successfully upgrades recovery from directory", Label("directory"), func() {
					srcDir, _ := utils.TempDir(fs, "", "elemental")
					// create a random file on it
					_ = fs.WriteFile(fmt.Sprintf("%s/file.file", srcDir), []byte("something"), constants.FilePerm)

					spec.Recovery.Source = v1.NewDirSrc(srcDir)
					upgrade = action.NewUpgradeAction(config, spec)
					err := upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// This should be the new image
					info, err := fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())

					// Transition squash should not exist
					info, err = fs.Stat(spec.Recovery.File)
					Expect(err).To(HaveOccurred())

				})
				It("Successfully upgrades recovery from channel upgrade", Label("channel"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := fs.ReadFile(recoveryImgSquash)
					Expect(f).To(ContainSubstring("recovery"))

					upgrade = action.NewUpgradeAction(config, spec)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(l.UnpackChannelCalled()).To(BeTrue())

					// This should be the new image
					info, err = fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ = fs.ReadFile(recoveryImgSquash)
					Expect(f).ToNot(ContainSubstring("recovery"))

					// Transition squash should not exist
					info, err = fs.Stat(spec.Recovery.File)
					Expect(err).To(HaveOccurred())
				})
			})
			Describe("Not using squashfs", Label("non-squashfs"), func() {
				var err error
				BeforeEach(func() {
					// Create recoveryImg so it identifies that we are using nonsquash recovery
					err = fs.WriteFile(recoveryImg, []byte("recovery"), constants.FilePerm)
					Expect(err).ShouldNot(HaveOccurred())

					spec, err = conf.NewUpgradeSpec(config.Config)
					Expect(err).ShouldNot(HaveOccurred())

					spec.RecoveryUpgrade = true
					spec.Recovery.Source = v1.NewChannelSrc("system/cos-config")
					spec.Recovery.Size = 16

					runner.SideEffect = func(command string, args ...string) ([]byte, error) {
						if command == "cat" && args[0] == "/proc/cmdline" {
							return []byte(constants.RecoveryLabel), nil
						}
						if command == "mv" && args[0] == "-f" && args[1] == spec.Recovery.File && args[2] == recoveryImg {
							// fake "move"
							f, _ := fs.ReadFile(spec.Recovery.File)
							_ = fs.WriteFile(recoveryImg, f, constants.FilePerm)
							_ = fs.RemoveAll(spec.Recovery.File)
						}
						return []byte{}, nil
					}
					config.Runner = runner
					_ = fs.WriteFile(recoveryImg, []byte("recovery"), constants.FilePerm)
					// Mount recovery partition as it is expected to be mounted when booting from recovery
					mounter.Mount("device5", constants.LiveDir, "auto", []string{"ro"})
				})
				It("Successfully upgrades recovery from docker image", Label("docker"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should not be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.Size()).To(BeNumerically("<", int64(spec.Recovery.Size*1024*1024)))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := fs.ReadFile(recoveryImg)
					Expect(f).To(ContainSubstring("recovery"))

					spec.Recovery.Source = v1.NewDockerSrc("apline")

					upgrade = action.NewUpgradeAction(config, spec)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(l.UnpackCalled()).To(BeTrue())

					// Should have created recovery image
					info, err = fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be default size
					Expect(info.Size()).To(BeNumerically("==", int64(spec.Recovery.Size*1024*1024)))

					// Expect the rest of the images to not be there
					for _, img := range []string{activeImg, passiveImg, recoveryImgSquash} {
						_, err := fs.Stat(img)
						Expect(err).To(HaveOccurred())
					}
				})
				It("Successfully upgrades recovery from directory", Label("directory"), func() {
					srcDir, _ := utils.TempDir(fs, "", "elemental")
					// create a random file on it
					_ = fs.WriteFile(fmt.Sprintf("%s/file.file", srcDir), []byte("something"), constants.FilePerm)

					spec.Recovery.Source = v1.NewDirSrc(srcDir)

					upgrade = action.NewUpgradeAction(config, spec)
					err := upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// This should be the new image
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be default size
					Expect(info.Size()).To(BeNumerically("==", int64(spec.Recovery.Size*1024*1024)))
					Expect(info.IsDir()).To(BeFalse())

					// Transition squash should not exist
					info, err = fs.Stat(spec.Recovery.File)
					Expect(err).To(HaveOccurred())
				})
				It("Successfully upgrades recovery from channel upgrade", Label("channel"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should not be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.Size()).To(BeNumerically("<", int64(spec.Recovery.Size*1024*1024)))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := fs.ReadFile(recoveryImg)
					Expect(f).To(ContainSubstring("recovery"))

					upgrade = action.NewUpgradeAction(config, spec)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(l.UnpackChannelCalled()).To(BeTrue())

					// Should have created recovery image
					info, err = fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Should have default image size
					Expect(info.Size()).To(BeNumerically("==", int64(spec.Recovery.Size*1024*1024)))

					// Expect the rest of the images to not be there
					for _, img := range []string{activeImg, passiveImg, recoveryImgSquash} {
						_, err := fs.Stat(img)
						Expect(err).To(HaveOccurred())
					}
				})
			})
		})
	})
})
