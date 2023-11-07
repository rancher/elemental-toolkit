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

package config_test

import (
	"path/filepath"

	"github.com/jaypipes/ghw/pkg/block"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/vfst"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("Config", func() {
		var err error
		var cleanup func()
		var fs *vfst.TestFS
		var mounter *v1mock.ErrorMounter
		var runner *v1mock.FakeRunner
		var client *v1mock.FakeHTTPClient
		var sysc *v1mock.FakeSyscall
		var logger v1.Logger
		var ci *v1mock.FakeCloudInitRunner
		var c *v1.Config
		BeforeEach(func() {
			fs, cleanup, err = vfst.NewTestFS(nil)
			Expect(err).ToNot(HaveOccurred())
			mounter = v1mock.NewErrorMounter()
			runner = v1mock.NewFakeRunner()
			client = &v1mock.FakeHTTPClient{}
			sysc = &v1mock.FakeSyscall{}
			logger = v1.NewNullLogger()
			ci = &v1mock.FakeCloudInitRunner{}
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
				runner := v1mock.NewFakeRunner()
				sysc := &v1mock.FakeSyscall{}
				logger := v1.NewNullLogger()
				c := config.NewConfig(
					config.WithRunner(runner),
					config.WithSyscall(sysc),
					config.WithLogger(logger),
				)
				Expect(c.Mounter).To(Equal(mount.New(constants.MountBinary)))
			})
		})
		Describe("RunConfig", func() {
			cfg := config.NewRunConfig(config.WithMounter(mounter))
			Expect(cfg.Mounter).To(Equal(mounter))
			Expect(cfg.Runner).NotTo(BeNil())
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
				Expect(spec.Firmware).To(Equal(v1.EFI))
				Expect(spec.Active.Source.Value()).To(Equal(constants.ISOBaseTree))
				Expect(spec.Recovery.Source.Value()).To(Equal(spec.Active.File))
				Expect(spec.PartTable).To(Equal(v1.GPT))

				// No firmware partitions added yet
				Expect(spec.Partitions.EFI).To(BeNil())

				// Adding firmware partitions
				err = spec.Partitions.SetFirmwarePartitions(spec.Firmware, spec.PartTable)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(spec.Partitions.EFI).NotTo(BeNil())
			})
			It("sets installation defaults without being on installation media", Label("install"), func() {
				spec := config.NewInstallSpec(*c)
				Expect(spec.Firmware).To(Equal(v1.EFI))
				Expect(spec.Active.Source.IsEmpty()).To(BeTrue())
				Expect(spec.Recovery.Source.Value()).To(Equal(spec.Active.File))
				Expect(spec.PartTable).To(Equal(v1.GPT))
			})
		})
		Describe("ResetSpec", Label("reset"), func() {
			Describe("Successful executions", func() {
				var ghwTest v1mock.GhwMock
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
					ghwTest = v1mock.GhwMock{}
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
					Expect(spec.Partitions.EFI.MountPoint).To(Equal(constants.EfiDir))
				})
				It("sets reset defaults on bios from non-squashed recovery", func() {
					// Set non-squashfs recovery image detection
					recoveryImg := filepath.Join(constants.RunningStateDir, "cOS", constants.RecoveryImgFile)
					err = utils.MkdirAll(fs, filepath.Dir(recoveryImg), constants.DirPerm)
					Expect(err).ShouldNot(HaveOccurred())
					_, err = fs.Create(recoveryImg)
					Expect(err).ShouldNot(HaveOccurred())

					spec, err := config.NewResetSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.Active.Source.Value()).To(Equal(recoveryImg))
				})
				It("sets reset defaults on bios from unknown recovery", func() {
					spec, err := config.NewResetSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.Active.Source.IsEmpty()).To(BeTrue())
				})
			})
			Describe("Failures", func() {
				var bootedFrom string
				var ghwTest v1mock.GhwMock
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
					ghwTest = v1mock.GhwMock{}
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
					ghwTest = v1mock.GhwMock{}
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
					Expect(err.Error()).To(ContainSubstring("EFI partition not found"))
				})
			})
		})
		Describe("UpgradeSpec", Label("upgrade"), func() {
			Describe("Successful executions", func() {
				var ghwTest v1mock.GhwMock
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
					ghwTest = v1mock.GhwMock{}
					ghwTest.AddDisk(mainDisk)
					ghwTest.CreateDevices()
				})
				AfterEach(func() {
					ghwTest.Clean()
				})
				It("sets upgrade defaults for active upgrade", func() {
					spec, err := config.NewUpgradeSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.Active.Source.IsEmpty()).To(BeTrue())
				})
				It("sets upgrade defaults for non-squashed recovery upgrade", func() {
					spec, err := config.NewUpgradeSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.Recovery.Source.IsEmpty()).To(BeTrue())
					Expect(spec.Recovery.FS).To(Equal(constants.LinuxImgFs))
				})
				It("sets upgrade defaults for squashed recovery upgrade", func() {
					//Set squashed recovery detection
					// Create installState with squashed recovery
					Expect(utils.MkdirAll(c.Fs, constants.RunningStateDir, constants.DirPerm)).To(Succeed())
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
					err = c.WriteInstallState(installState, statePath, statePath)
					Expect(err).ShouldNot(HaveOccurred())

					spec, err := config.NewUpgradeSpec(*c)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(spec.Recovery.Source.IsEmpty()).To(BeTrue())
					Expect(spec.Recovery.FS).To(Equal(constants.SquashFs))
				})
			})
		})
		Describe("BuildConfig", Label("build"), func() {
			It("initiates a new build config", func() {
				build := config.NewBuildConfig()
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
