/*
   Copyright © 2022 - 2023 SUSE LLC

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
	"os"
	"path/filepath"
	"strings"

	"github.com/jaypipes/ghw/pkg/block"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-toolkit/pkg/action"
	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
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
	var extractor *v1mock.FakeImageExtractor
	var cleanup func()
	var memLog *bytes.Buffer
	var ghwTest v1mock.GhwMock
	var bootloader *v1mock.FakeBootloader

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		bootloader = &v1mock.FakeBootloader{}
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

	Describe("Upgrade Action", Label("upgrade"), func() {
		var spec *v1.UpgradeSpec
		var upgrade *action.UpgradeAction
		var memLog *bytes.Buffer
		activeImg := fmt.Sprintf("%s/cOS/%s", constants.RunningStateDir, constants.ActiveImgFile)
		passiveImg := fmt.Sprintf("%s/cOS/%s", constants.RunningStateDir, constants.PassiveImgFile)

		recoveryImg := fmt.Sprintf("%s/cOS/%s", constants.LiveDir, constants.RecoveryImgFile)

		BeforeEach(func() {
			memLog = &bytes.Buffer{}
			logger = v1.NewBufferLogger(memLog)
			config.Logger = logger
			logger.SetLevel(logrus.DebugLevel)

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

				spec.Active.Source = v1.NewDockerSrc("alpine")
				spec.Active.Size = 16

				err = utils.MkdirAll(config.Fs, filepath.Join(constants.WorkingImgDir, "etc"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())

				err = fs.WriteFile(
					filepath.Join(constants.WorkingImgDir, "etc", "os-release"),
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
					if command == "grub2-editenv" && args[1] == "set" {
						f, err := fs.OpenFile(args[0], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						Expect(err).To(BeNil())

						_, err = f.Write([]byte(fmt.Sprintf("%s\n", args[2])))
						Expect(err).To(BeNil())
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
			It("Fails setting the grub labels", func() {
				bootloader.ErrorSetPersistentVariables = true
				upgrade = action.NewUpgradeAction(config, spec, action.WithUpgradeBootloader(bootloader))
				err := upgrade.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("setting persistent variables"))
			})
			It("Fails setting the grub default entry", func() {
				bootloader.ErrorSetDefaultEntry = true
				upgrade = action.NewUpgradeAction(config, spec, action.WithUpgradeBootloader(bootloader))
				err := upgrade.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("setting default entry"))
			})
			It("Successfully upgrades from docker image with custom labels", Label("docker"), func() {
				// Create installState with previous install state
				statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
				installState := &v1.InstallState{
					Partitions: map[string]*v1.PartitionState{
						constants.StatePartName: {
							FSLabel: "COS_STATE",
							Images: map[string]*v1.ImageState{
								constants.ActiveImgName: {
									Label: "CUSTOM_ACTIVE_LABEL",
									FS:    constants.LinuxImgFs,
								},
								constants.PassiveImgName: {
									Label: "CUSTOM_PASSIVE_LABEL",
									FS:    constants.LinuxImgFs,
								},
							},
						},
					},
				}
				err = config.WriteInstallState(installState, statePath, statePath)
				Expect(err).ShouldNot(HaveOccurred())

				// Create a new spec to load state yaml
				spec, err = conf.NewUpgradeSpec(config.Config)
				spec.Active.Size = 16
				Expect(err).ShouldNot(HaveOccurred())

				spec.Active.Source = v1.NewDockerSrc("alpine")
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

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

				// An upgraded state yaml file should exist
				state, err := config.LoadInstallState()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(
					state.Partitions[constants.StatePartName].
						Images[constants.ActiveImgName].Source.String()).
					To(Equal("oci://alpine:latest"))
				Expect(
					state.Partitions[constants.StatePartName].
						Images[constants.ActiveImgName].Label).
					To(Equal("CUSTOM_ACTIVE_LABEL"))
				Expect(
					state.Partitions[constants.StatePartName].
						Images[constants.PassiveImgName].Label).
					To(Equal("CUSTOM_PASSIVE_LABEL"))
			})
			It("Writes filesystem labels to GRUB oem env file", Label("grub"), func() {
				statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
				installState := &v1.InstallState{
					Partitions: map[string]*v1.PartitionState{
						constants.RecoveryPartName: {
							FSLabel: "COS_RECOVERY",
							Images: map[string]*v1.ImageState{
								constants.RecoveryImgName: {
									Label: "CUSTOM_RECOVERYIMG_LABEL",
									FS:    constants.LinuxImgFs,
								},
							},
						},
						constants.StatePartName: {
							FSLabel: "COS_STATE",
							Images: map[string]*v1.ImageState{
								constants.ActiveImgName: {
									Label: "CUSTOM_ACTIVE_LABEL",
									FS:    constants.LinuxImgFs,
								},
								constants.PassiveImgName: {
									Label: "CUSTOM_PASSIVE_LABEL",
									FS:    constants.LinuxImgFs,
								},
							},
						},
					},
				}

				err = config.WriteInstallState(installState, statePath, statePath)
				Expect(err).ShouldNot(HaveOccurred())

				spec, err = conf.NewUpgradeSpec(config.Config)
				spec.Active.Source = v1.NewDockerSrc("alpine")
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				actualBytes, err := fs.ReadFile(filepath.Join(constants.RunningStateDir, "grub_oem_env"))
				Expect(err).To(BeNil())

				expected := map[string]string{
					"state_label":        "COS_STATE",
					"active_label":       "CUSTOM_ACTIVE_LABEL",
					"passive_label":      "CUSTOM_PASSIVE_LABEL",
					"recovery_label":     "COS_RECOVERY",
					"system_label":       "CUSTOM_RECOVERYIMG_LABEL",
					"oem_label":          "COS_OEM",
					"persistent_label":   "COS_PERSISTENT",
					"default_menu_entry": "TESTOS",
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
			It("Successfully reboots after upgrade from docker image", Label("docker"), func() {
				spec.Active.Source = v1.NewDockerSrc("alpine")
				config.Reboot = true
				upgrade = action.NewUpgradeAction(config, spec)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

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

				// An upgraded state yaml file should exist
				state, err := config.LoadInstallState()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(
					state.Partitions[constants.StatePartName].
						Images[constants.ActiveImgName].Source.String()).
					To(Equal("oci://alpine:latest"))
				Expect(
					state.Partitions[constants.StatePartName].
						Images[constants.PassiveImgName].Label).
					To(Equal(constants.PassiveLabel))
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
			It("Successfully upgrades with cosign", Pending, Label("channel", "cosign"), func() {})
			It("Successfully upgrades with strict", Pending, Label("channel", "strict"), func() {})
		})
		Describe(fmt.Sprintf("Booting from %s", constants.PassiveLabel), Label("passive_label"), func() {
			var err error
			BeforeEach(func() {
				spec, err = conf.NewUpgradeSpec(config.Config)
				Expect(err).ShouldNot(HaveOccurred())

				spec.Active.Source = v1.NewDockerSrc("elementalos:latest")
				spec.Active.Size = 16

				err = utils.MkdirAll(config.Fs, filepath.Join(constants.WorkingImgDir, "etc"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())

				err = fs.WriteFile(
					filepath.Join(constants.WorkingImgDir, "etc", "os-release"),
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
					err = fs.WriteFile(recoveryImg, []byte("recovery"), constants.FilePerm)
					Expect(err).ShouldNot(HaveOccurred())

					spec, err = conf.NewUpgradeSpec(config.Config)
					Expect(err).ShouldNot(HaveOccurred())

					spec.RecoveryUpgrade = true
					spec.Recovery.Source = v1.NewDockerSrc("alpine")
					spec.Recovery.Size = 16

					runner.SideEffect = func(command string, args ...string) ([]byte, error) {
						if command == "cat" && args[0] == "/proc/cmdline" {
							return []byte(constants.RecoveryLabel), nil
						}
						if command == "mksquashfs" && args[1] == spec.Recovery.File {
							// create the transition img for squash to fake it
							_, _ = fs.Create(spec.Recovery.File)
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
				})
				It("Successfully upgrades recovery from docker image", Label("docker"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := fs.ReadFile(recoveryImg)
					Expect(f).To(ContainSubstring("recovery"))

					spec.Recovery.Source = v1.NewDockerSrc("alpine")
					upgrade = action.NewUpgradeAction(config, spec)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// This should be the new image
					info, err = fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ = fs.ReadFile(recoveryImg)
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
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())

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
					spec.Recovery.Source = v1.NewDockerSrc("alpine")
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

					// Should have created recovery image
					info, err = fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be default size
					Expect(info.Size()).To(BeNumerically("==", int64(spec.Recovery.Size*1024*1024)))

					// Expect the rest of the images to not be there
					for _, img := range []string{activeImg, passiveImg} {
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

					// Transition should not exist
					info, err = fs.Stat(spec.Recovery.File)
					Expect(err).To(HaveOccurred())

					// An upgraded state yaml file should exist
					state, err := config.LoadInstallState()
					Expect(err).ShouldNot(HaveOccurred())
					Expect(
						state.Partitions[constants.RecoveryPartName].
							Images[constants.RecoveryImgName].Source.String()).
						To(Equal(spec.Recovery.Source.String()))
				})
			})
		})
	})
})
