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

package config_test

import (
	"path/filepath"

	"github.com/jaypipes/ghw/pkg/block"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("Config", func() {
		var err error
		var cleanup func()
		var fs *vfst.TestFS
		var mounter *mocks.FakeMounter
		var runner *mocks.FakeRunner
		var client *mocks.FakeHTTPClient
		var sysc *mocks.FakeSyscall
		var logger types.Logger
		var ci *mocks.FakeCloudInitRunner
		var c *types.Config
		BeforeEach(func() {
			fs, cleanup, err = vfst.NewTestFS(nil)
			Expect(err).ToNot(HaveOccurred())
			mounter = mocks.NewFakeMounter()
			runner = mocks.NewFakeRunner()
			client = &mocks.FakeHTTPClient{}
			sysc = &mocks.FakeSyscall{}
			logger = types.NewNullLogger()
			ci = &mocks.FakeCloudInitRunner{}
			c = config.NewConfig(
				config.WithFs(fs),
				config.WithMounter(mounter),
				config.WithRunner(runner),
				config.WithSyscall(sysc),
				config.WithLogger(logger),
				config.WithCloudInitRunner(ci),
				config.WithClient(client),
			)
		})
		AfterEach(func() {
			cleanup()
		})
		Describe("ConfigOptions", func() {
			It("Sets the proper interfaces in the config struct", func() {
				Expect(c.Fs).To(Equal(fs))
				Expect(c.Mounter).To(Equal(mounter))
				Expect(c.Runner).To(Equal(runner))
				Expect(c.Syscall).To(Equal(sysc))
				Expect(c.Logger).To(Equal(logger))
				Expect(c.CloudInitRunner).To(Equal(ci))
				Expect(c.Client).To(Equal(client))
			})
			It("Sets the runner if we dont pass one", func() {
				fs, cleanup, err := vfst.NewTestFS(nil)
				defer cleanup()
				Expect(err).ToNot(HaveOccurred())
				c := config.NewConfig(
					config.WithFs(fs),
					config.WithMounter(mounter),
				)
				Expect(c.Fs).To(Equal(fs))
				Expect(c.Mounter).To(Equal(mounter))
				Expect(c.Runner).ToNot(BeNil())
			})
		})
		Describe("ConfigOptions no mounter specified", Label("mount", "mounter"), func() {
			It("should use the default mounter", Label("systemctl"), func() {
				runner := mocks.NewFakeRunner()
				sysc := &mocks.FakeSyscall{}
				logger := types.NewNullLogger()
				c := config.NewConfig(
					config.WithRunner(runner),
					config.WithSyscall(sysc),
					config.WithLogger(logger),
				)
				Expect(c.Mounter).ToNot(BeNil())
			})
		})
		Describe("RunConfig", func() {
			cfg := config.NewRunConfig(config.WithMounter(mounter))
			Expect(cfg.Mounter).To(Equal(mounter))
			Expect(cfg.Runner).NotTo(BeNil())
			It("sets the default snapshot", func() {
				Expect(cfg.Snapshotter.MaxSnaps).To(Equal(constants.LoopDeviceMaxSnaps))
				Expect(cfg.Snapshotter.Type).To(Equal(constants.LoopDeviceSnapshotterType))
				snapshotterCfg, ok := cfg.Snapshotter.Config.(*types.LoopDeviceConfig)
				Expect(ok).To(BeTrue())
				Expect(snapshotterCfg.FS).To(Equal(constants.LinuxImgFs))
				Expect(snapshotterCfg.Size).To(Equal(constants.ImgSize))
			})
		})
		Describe("InstallSpec", func() {
			It("sets installation defaults from install media with recovery", Label("install"), func() {
				// Set EFI firmware detection
				err = utils.MkdirAll(fs, filepath.Dir(constants.EfiDevice), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())
				_, err = fs.Create(constants.EfiDevice)
				Expect(err).ShouldNot(HaveOccurred())

				// Set ISO base tree detection
				err = utils.MkdirAll(fs, filepath.Dir(constants.ISOBaseTree), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())
				_, err = fs.Create(constants.ISOBaseTree)
				Expect(err).ShouldNot(HaveOccurred())

				spec := config.NewInstallSpec(*c)
				Expect(spec.Firmware).To(Equal(types.EFI))
				Expect(spec.System.Value()).To(Equal(constants.ISOBaseTree))
				Expect(spec.RecoverySystem.Source.Value()).To(Equal(spec.System.Value()))
				Expect(spec.PartTable).To(Equal(types.GPT))

				Expect(spec.Partitions.Boot).NotTo(BeNil())
			})
			It("sets installation defaults without being on installation media", Label("install"), func() {
				spec := config.NewInstallSpec(*c)
				Expect(spec.Firmware).To(Equal(types.EFI))
				Expect(spec.System.IsEmpty()).To(BeTrue())
				Expect(spec.RecoverySystem.Source.IsEmpty()).To(BeTrue())
				Expect(spec.PartTable).To(Equal(types.GPT))
			})
		})
		Describe("ResetSpec", Label("reset"), func() {
			Describe("Successful executions", func() {
				var ghwTest mocks.GhwMock
				BeforeEach(func() {
					mainDisk := block.Disk{
						Name: "device",
						Partitions: []*block.Partition{
							{
								Name:            "device1",
								FilesystemLabel: constants.EfiLabel,
								Type:            "vfat",
							},
							{
								Name:            "device2",
								FilesystemLabel: constants.OEMLabel,
								Type:            "ext4",
							},
							{
								Name:            "device3",
								FilesystemLabel: constants.RecoveryLabel,
								Type:            "ext4",
							},
							{
								Name:            "device4",
								FilesystemLabel: constants.StateLabel,
								Type:            "ext4",
							},
							{
								Name:            "device5",
								FilesystemLabel: constants.PersistentLabel,
								Type:            "ext4",
							},
						},
					}
					ghwTest = mocks.GhwMock{}
					ghwTest.AddDisk(mainDisk)
					ghwTest.CreateDevices()

					runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
						switch cmd {
						case "cat":
							return []byte(constants.RecoveryImgFile), nil
						default:
							return []byte{}, nil
						}
					}
				})
				AfterEach(func() {
					ghwTest.Clean()
				})
				It("sets reset defaults on efi from squashed recovery", func() {
					// Set EFI firmware detection
					err = utils.MkdirAll(fs, filepath.Dir(constants.EfiDevice), constants.DirPerm)
					Expect(err).ShouldNot(HaveOccurred())
					_, err = fs.Create(constants.EfiDevice)
					Expect(err).ShouldNot(HaveOccurred())

					// Set squashfs detection
					err = utils.MkdirAll(fs, filepath.Dir(constants.ISOBaseTree), constants.DirPerm)
					Expect(err).ShouldNot(HaveOccurred())
					_, err = fs.Create(constants.ISOBaseTree)
					Expect(err).ShouldNot(HaveOccurred())

					spec, err := config.NewResetSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.Partitions.Boot.MountPoint).To(Equal(constants.EfiDir))
				})
				It("sets reset defaults to recovery image", func() {
					// Set non-squashfs recovery image detection
					recoveryImg := filepath.Join(constants.RunningStateDir, constants.RecoveryImgFile)
					err = utils.MkdirAll(fs, filepath.Dir(recoveryImg), constants.DirPerm)
					Expect(err).ShouldNot(HaveOccurred())
					_, err = fs.Create(recoveryImg)
					Expect(err).ShouldNot(HaveOccurred())

					spec, err := config.NewResetSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.System.Value()).To(Equal(recoveryImg))
				})
				It("sets reset defaults to empty of no recovery image is available", func() {
					spec, err := config.NewResetSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.System.IsEmpty()).To(BeTrue())
				})
			})
			Describe("Failures", func() {
				var bootedFrom string
				var ghwTest mocks.GhwMock
				BeforeEach(func() {
					bootedFrom = ""
					runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
						switch cmd {
						case "cat":
							return []byte(bootedFrom), nil
						default:
							return []byte{}, nil
						}
					}

					// Set an empty disk for tests, otherwise reads the hosts hardware
					mainDisk := block.Disk{
						Name: "device",
						Partitions: []*block.Partition{
							{
								Name:            "device4",
								FilesystemLabel: constants.StateLabel,
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
				It("fails to set defaults if not booted from recovery", func() {
					_, err := config.NewResetSpec(*c)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("reset can only be called from the recovery system"))
				})
				It("fails to set defaults if no recovery partition detected", func() {
					bootedFrom = constants.RecoveryImgFile
					_, err := config.NewResetSpec(*c)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("recovery partition not found"))
				})
				It("fails to set defaults if no state partition detected", func() {
					mainDisk := block.Disk{
						Name:       "device",
						Partitions: []*block.Partition{},
					}
					ghwTest = mocks.GhwMock{}
					ghwTest.AddDisk(mainDisk)
					ghwTest.CreateDevices()
					defer ghwTest.Clean()

					bootedFrom = constants.RecoveryImgFile
					_, err := config.NewResetSpec(*c)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("state partition not found"))
				})
				It("fails to set defaults if no efi partition on efi firmware", func() {
					// Set EFI firmware detection
					err = utils.MkdirAll(fs, filepath.Dir(constants.EfiDevice), constants.DirPerm)
					Expect(err).ShouldNot(HaveOccurred())
					_, err = fs.Create(constants.EfiDevice)
					Expect(err).ShouldNot(HaveOccurred())

					bootedFrom = constants.RecoveryImgFile
					_, err := config.NewResetSpec(*c)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Bootloader partition not found"))
				})
			})
		})
		Describe("UpgradeSpec", Label("upgrade"), func() {
			Describe("Successful executions", func() {
				var ghwTest mocks.GhwMock
				BeforeEach(func() {
					mainDisk := block.Disk{
						Name: "device",
						Partitions: []*block.Partition{
							{
								Name:            "device1",
								FilesystemLabel: constants.EfiLabel,
								Type:            "vfat",
							},
							{
								Name:            "device2",
								FilesystemLabel: constants.OEMLabel,
								Type:            "ext4",
							},
							{
								Name:            "device3",
								FilesystemLabel: constants.RecoveryLabel,
								Type:            "ext4",
								MountPoint:      constants.LiveDir,
							},
							{
								Name:            "device4",
								FilesystemLabel: constants.StateLabel,
								Type:            "ext4",
							},
							{
								Name:            "device5",
								FilesystemLabel: constants.PersistentLabel,
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
			})
		})
		Describe("BuildConfig", Label("build"), func() {
			It("initiates a new build config", func() {
				build := config.NewBuildConfig(config.WithMounter(mounter))
				Expect(build.Name).To(Equal(constants.BuildImgName))
			})
		})
		Describe("LiveISO", Label("iso"), func() {
			It("initiates a new LiveISO", func() {
				iso := config.NewISO()
				Expect(iso.Label).To(Equal(constants.ISOLabel))
				Expect(len(iso.UEFI)).To(Equal(0))
				Expect(len(iso.Image)).To(Equal(0))
			})
		})
	})
})
