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

package v1_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-toolkit/pkg/config"
	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mocks "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("Write and load installation state", func() {
		var config *v1.RunConfig
		var runner *v1mocks.FakeRunner
		var fs vfs.FS
		var mounter *v1mocks.ErrorMounter
		var cleanup func()
		var err error
		var dockerState, channelState *v1.ImageState
		var installState *v1.InstallState
		var statePath, recoveryPath string

		BeforeEach(func() {
			runner = v1mocks.NewFakeRunner()
			mounter = v1mocks.NewErrorMounter()
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())

			config = conf.NewRunConfig(
				conf.WithFs(fs),
				conf.WithRunner(runner),
				conf.WithMounter(mounter),
			)
			dockerState = &v1.ImageState{
				Source: v1.NewDockerSrc("registry.org/my/image:tag"),
				Label:  "active_label",
				FS:     "ext2",
				SourceMetadata: &v1.DockerImageMeta{
					Digest: "adadgadg",
					Size:   23452345,
				},
			}
			installState = &v1.InstallState{
				Date: "somedate",
				Partitions: map[string]*v1.PartitionState{
					"state": {
						FSLabel: "state_label",
						Images: map[string]*v1.ImageState{
							"active": dockerState,
						},
					},
					"recovery": {
						FSLabel: "state_label",
						Images: map[string]*v1.ImageState{
							"recovery": channelState,
						},
					},
				},
			}

			statePath = filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
			recoveryPath = "/recoverypart/state.yaml"
			err = utils.MkdirAll(fs, filepath.Dir(recoveryPath), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			err = utils.MkdirAll(fs, filepath.Dir(statePath), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
		})
		AfterEach(func() {
			cleanup()
		})
		It("Writes and loads an installation data", func() {
			err = config.WriteInstallState(installState, statePath, recoveryPath)
			Expect(err).ShouldNot(HaveOccurred())
			loadedInstallState, err := config.LoadInstallState()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(*loadedInstallState).To(Equal(*installState))
		})
		It("Fails writing to state partition", func() {
			err = fs.RemoveAll(filepath.Dir(statePath))
			Expect(err).ShouldNot(HaveOccurred())
			err = config.WriteInstallState(installState, statePath, recoveryPath)
			Expect(err).Should(HaveOccurred())
		})
		It("Fails writing to recovery partition", func() {
			err = fs.RemoveAll(filepath.Dir(statePath))
			Expect(err).ShouldNot(HaveOccurred())
			err = config.WriteInstallState(installState, statePath, recoveryPath)
			Expect(err).Should(HaveOccurred())
		})
		It("Fails loading state file", func() {
			err = config.WriteInstallState(installState, statePath, recoveryPath)
			Expect(err).ShouldNot(HaveOccurred())
			err = fs.RemoveAll(filepath.Dir(statePath))
			_, err = config.LoadInstallState()
			Expect(err).Should(HaveOccurred())
		})
		It("Loads a state file with missing fields", func() {
			incompleteState := "state:\n"
			incompleteState += "  active:\n"
			incompleteState += "    source: dir:///tmp/new_root\n"
			incompleteState += "    fs: ext2\n"
			incompleteState += "  passive: null\n"
			incompleteState += "recovery:\n"
			incompleteState += "  recovery:\n"
			incompleteState += "    source: dir:///tmp/new_root\n"
			Expect(fs.WriteFile(statePath, []byte(incompleteState), constants.FilePerm)).To(Succeed())

			loadedInstallState, err := config.LoadInstallState()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(loadedInstallState.Partitions[constants.RecoveryPartName].FSLabel).To(Equal(constants.RecoveryLabel))
			Expect(loadedInstallState.Partitions[constants.RecoveryPartName].Images[constants.RecoveryImgName].Label).To(Equal(constants.SystemLabel))
			Expect(loadedInstallState.Partitions[constants.StatePartName].FSLabel).To(Equal(constants.StateLabel))
			Expect(loadedInstallState.Partitions[constants.StatePartName].Images[constants.PassiveImgName]).To(BeNil())
			Expect(loadedInstallState.Partitions[constants.StatePartName].Images[constants.ActiveImgName].Label).To(Equal(constants.ActiveLabel))
		})
		It("Loads a state file with other missing fields", func() {
			incompleteState := "state:\n"
			incompleteState += "  passive:\n"
			incompleteState += "    source: dir:///tmp/new_root\n"
			incompleteState += "    fs: ext2\n"
			incompleteState += "  active: null\n"
			Expect(fs.WriteFile(statePath, []byte(incompleteState), constants.FilePerm)).To(Succeed())

			loadedInstallState, err := config.LoadInstallState()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(loadedInstallState.Partitions[constants.RecoveryPartName]).To(BeNil())
			Expect(loadedInstallState.Partitions[constants.StatePartName].FSLabel).To(Equal(constants.StateLabel))
			Expect(loadedInstallState.Partitions[constants.StatePartName].Images[constants.ActiveImgName]).To(BeNil())
			Expect(loadedInstallState.Partitions[constants.StatePartName].Images[constants.PassiveImgName].Label).To(Equal(constants.PassiveLabel))
		})
	})
	Describe("ElementalPartitions", func() {
		var p v1.PartitionList
		var ep v1.ElementalPartitions
		BeforeEach(func() {
			ep = v1.ElementalPartitions{}
			p = v1.PartitionList{
				&v1.Partition{
					FilesystemLabel: "COS_OEM",
					Size:            0,
					Name:            "oem",
					FS:              "",
					Flags:           nil,
					MountPoint:      "/some/path/nested",
					Path:            "",
					Disk:            "",
				},
				&v1.Partition{
					FilesystemLabel: "COS_CUSTOM",
					Size:            0,
					Name:            "persistent",
					FS:              "",
					Flags:           nil,
					MountPoint:      "/some/path",
					Path:            "",
					Disk:            "",
				},
				&v1.Partition{
					FilesystemLabel: "SOMETHING",
					Size:            0,
					Name:            "somename",
					FS:              "",
					Flags:           nil,
					MountPoint:      "",
					Path:            "",
					Disk:            "",
				},
			}
		})
		It("sets firmware partitions on efi", func() {
			Expect(ep.EFI == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(v1.EFI, v1.GPT)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ep.EFI != nil && ep.BIOS == nil).To(BeTrue())
		})
		It("sets firmware partitions on bios", func() {
			Expect(ep.EFI == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(v1.BIOS, v1.GPT)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ep.EFI == nil && ep.BIOS != nil).To(BeTrue())
		})
		It("sets firmware partitions on msdos", func() {
			ep.State = &v1.Partition{}
			Expect(ep.EFI == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(v1.BIOS, v1.MSDOS)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ep.EFI == nil && ep.BIOS == nil).To(BeTrue())
			Expect(ep.State.Flags != nil && ep.State.Flags[0] == "boot").To(BeTrue())
		})
		It("fails to set firmware partitions of state is not defined on msdos", func() {
			Expect(ep.EFI == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(v1.BIOS, v1.MSDOS)
			Expect(err).Should(HaveOccurred())
		})
		It("initializes an ElementalPartitions from a PartitionList", func() {
			// Use custom label for recovery partition
			ep := v1.NewElementalPartitionsFromList(p, &v1.InstallState{
				Partitions: map[string]*v1.PartitionState{
					constants.RecoveryPartName: {
						FSLabel: "SOMETHING",
					},
				},
			})
			Expect(ep.Persistent != nil).To(BeTrue())
			Expect(ep.OEM != nil).To(BeTrue())
			Expect(ep.BIOS == nil).To(BeTrue())
			Expect(ep.EFI == nil).To(BeTrue())
			Expect(ep.State == nil).To(BeTrue())
			Expect(ep.Recovery != nil).To(BeTrue())
		})
		Describe("returns a partition list by install order", func() {
			It("with no extra parts", func() {
				ep := v1.NewElementalPartitionsFromList(p, nil)
				lst := ep.PartitionsByInstallOrder([]*v1.Partition{})
				Expect(len(lst)).To(Equal(2))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
			})
			It("with extra parts with size > 0", func() {
				// Use custom label for state partition
				ep := v1.NewElementalPartitionsFromList(p, &v1.InstallState{
					Partitions: map[string]*v1.PartitionState{
						constants.StatePartName: {
							FSLabel: "SOMETHING",
						},
					},
				})
				var extraParts []*v1.Partition
				extraParts = append(extraParts, &v1.Partition{Name: "extra", Size: 5})

				lst := ep.PartitionsByInstallOrder(extraParts)
				Expect(len(lst)).To(Equal(4))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "somename").To(BeTrue())
				Expect(lst[2].Name == "extra").To(BeTrue())
				Expect(lst[3].Name == "persistent").To(BeTrue())
			})
			It("with extra part with size == 0 and persistent.Size == 0", func() {
				ep := v1.NewElementalPartitionsFromList(p, &v1.InstallState{})
				var extraParts []*v1.Partition
				extraParts = append(extraParts, &v1.Partition{Name: "extra", Size: 0})
				lst := ep.PartitionsByInstallOrder(extraParts)
				// Should ignore the wrong partition had have the persistent over it
				Expect(len(lst)).To(Equal(2))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
			})
			It("with extra part with size == 0 and persistent.Size > 0", func() {
				ep := v1.NewElementalPartitionsFromList(p, nil)
				ep.Persistent.Size = 10
				var extraParts []*v1.Partition
				extraParts = append(extraParts, &v1.Partition{Name: "extra", FilesystemLabel: "LABEL", Size: 0})
				lst := ep.PartitionsByInstallOrder(extraParts)
				// Will have our size == 0 partition the latest
				Expect(len(lst)).To(Equal(3))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
				Expect(lst[2].Name == "extra").To(BeTrue())
			})
			It("with several extra parts with size == 0 and persistent.Size > 0", func() {
				ep := v1.NewElementalPartitionsFromList(p, nil)
				ep.Persistent.Size = 10
				var extraParts []*v1.Partition
				extraParts = append(extraParts, &v1.Partition{Name: "extra1", Size: 0})
				extraParts = append(extraParts, &v1.Partition{Name: "extra2", Size: 0})
				lst := ep.PartitionsByInstallOrder(extraParts)
				// Should ignore the wrong partition had have the first partition with size 0 added last
				Expect(len(lst)).To(Equal(3))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
				Expect(lst[2].Name == "extra1").To(BeTrue())
			})
		})

		It("returns a partition list by mount order", func() {
			ep := v1.NewElementalPartitionsFromList(p, nil)
			lst := ep.PartitionsByMountPoint(false)
			Expect(len(lst)).To(Equal(2))
			Expect(lst[0].Name == "persistent").To(BeTrue())
			Expect(lst[1].Name == "oem").To(BeTrue())
		})
		It("returns a partition list by mount reverse order", func() {
			ep := v1.NewElementalPartitionsFromList(p, nil)
			lst := ep.PartitionsByMountPoint(true)
			Expect(len(lst)).To(Equal(2))
			Expect(lst[0].Name == "oem").To(BeTrue())
			Expect(lst[1].Name == "persistent").To(BeTrue())
		})
	})
	Describe("Partitionlist", func() {
		var p v1.PartitionList
		BeforeEach(func() {
			p = v1.PartitionList{
				&v1.Partition{
					FilesystemLabel: "ONE",
					Size:            0,
					Name:            "one",
					FS:              "",
					Flags:           nil,
					MountPoint:      "",
					Path:            "",
					Disk:            "",
				},
				&v1.Partition{
					FilesystemLabel: "TWO",
					Size:            0,
					Name:            "two",
					FS:              "",
					Flags:           nil,
					MountPoint:      "",
					Path:            "",
					Disk:            "",
				},
			}
		})
		It("returns partitions by name", func() {
			Expect(p.GetByName("two")).To(Equal(&v1.Partition{
				FilesystemLabel: "TWO",
				Size:            0,
				Name:            "two",
				FS:              "",
				Flags:           nil,
				MountPoint:      "",
				Path:            "",
				Disk:            "",
			}))
		})
		It("returns nil if partiton name not found", func() {
			Expect(p.GetByName("nonexistent")).To(BeNil())
		})
		It("returns partitions by filesystem label", func() {
			Expect(p.GetByLabel("TWO")).To(Equal(&v1.Partition{
				FilesystemLabel: "TWO",
				Size:            0,
				Name:            "two",
				FS:              "",
				Flags:           nil,
				MountPoint:      "",
				Path:            "",
				Disk:            "",
			}))
		})
		It("returns nil if filesystem label not found", func() {
			Expect(p.GetByName("nonexistent")).To(BeNil())
		})
	})
	Describe("InstallSpec", func() {
		var spec *v1.InstallSpec

		BeforeEach(func() {
			cfg := config.NewConfig(config.WithMounter(v1mocks.NewErrorMounter()))
			spec = config.NewInstallSpec(*cfg)
		})
		Describe("sanitize", func() {
			It("runs method", func() {
				Expect(spec.Partitions.EFI).To(BeNil())
				Expect(spec.Active.Source.IsEmpty()).To(BeTrue())

				// Creates firmware partitions
				spec.Active.Source = v1.NewDirSrc("/dir")
				spec.Firmware = v1.EFI
				err := spec.Sanitize()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(spec.Partitions.EFI).NotTo(BeNil())

				// Sets image labels to empty string on squashfs
				spec.Active.FS = constants.SquashFs
				spec.Recovery.FS = constants.SquashFs
				err = spec.Sanitize()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(spec.Recovery.Label).To(BeEmpty())
				Expect(spec.Active.Label).To(BeEmpty())
				Expect(spec.Passive.Label).To(BeEmpty())

				// Fails without state partition
				spec.Partitions.State = nil
				err = spec.Sanitize()
				Expect(err).Should(HaveOccurred())

				// Fails without an install source
				spec.Active.Source = v1.NewEmptySrc()
				err = spec.Sanitize()
				Expect(err).Should(HaveOccurred())
			})
			Describe("with extra partitions", func() {
				BeforeEach(func() {
					// Set a source for the install
					spec.Active.Source = v1.NewDirSrc("/dir")
				})
				It("fails if persistent and an extra partition have size == 0", func() {
					spec.ExtraPartitions = append(spec.ExtraPartitions, &v1.Partition{Size: 0})
					err := spec.Sanitize()
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("both persistent partition and extra partitions have size set to 0"))
				})
				It("fails if more than one extra partition has size == 0", func() {
					spec.Partitions.Persistent.Size = 10
					spec.ExtraPartitions = append(spec.ExtraPartitions, &v1.Partition{Name: "1", Size: 0})
					spec.ExtraPartitions = append(spec.ExtraPartitions, &v1.Partition{Name: "2", Size: 0})
					err := spec.Sanitize()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("more than one extra partition has its size set to 0"))
				})
				It("does not fail if persistent size is > 0 and an extra partition has size == 0", func() {
					spec.ExtraPartitions = append(spec.ExtraPartitions, &v1.Partition{Size: 0})
					spec.Partitions.Persistent.Size = 10
					err := spec.Sanitize()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
	Describe("ResetSpec", func() {
		It("runs sanitize method", func() {
			spec := &v1.ResetSpec{
				Active: v1.Image{
					Source: v1.NewDirSrc("/dir"),
				},
				Partitions: v1.ElementalPartitions{
					State: &v1.Partition{
						MountPoint: "mountpoint",
					},
				},
			}
			err := spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())

			// Sets image labels to empty string on squashfs
			spec.Active.FS = constants.SquashFs
			err = spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(spec.Active.Label).To(BeEmpty())
			Expect(spec.Passive.Label).To(BeEmpty())

			//Fails on missing state partition
			spec.Partitions.State = nil
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())

			//Fails on empty source
			spec.Active.Source = v1.NewEmptySrc()
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("UpgradeSpec", func() {
		It("runs sanitize method", func() {
			spec := &v1.UpgradeSpec{
				Active: v1.Image{
					Source: v1.NewDirSrc("/dir"),
				},
				Recovery: v1.Image{
					Source: v1.NewDirSrc("/dir"),
				},
				Partitions: v1.ElementalPartitions{
					State: &v1.Partition{
						MountPoint: "mountpoint",
					},
					Recovery: &v1.Partition{
						MountPoint: "mountpoint",
					},
				},
			}
			err := spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())

			// Sets image labels to empty string on squashfs
			spec.Active.FS = constants.SquashFs
			spec.Recovery.FS = constants.SquashFs
			err = spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(spec.Recovery.Label).To(BeEmpty())
			Expect(spec.Active.Label).To(BeEmpty())
			Expect(spec.Passive.Label).To(BeEmpty())

			//Fails on empty source for active upgrade
			spec.Active.Source = v1.NewEmptySrc()
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())

			//Fails on missing state partition for active upgrade
			spec.Partitions.State = nil
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())

			//Fails on empty source for recovery upgrade
			spec.RecoveryUpgrade = true
			spec.Recovery.Source = v1.NewEmptySrc()
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())

			//Fails on missing recovery partition for recovery upgrade
			spec.Partitions.Recovery = nil
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("LiveISO", func() {
		It("runs sanitize method", func() {
			iso := config.NewISO()
			Expect(iso.Sanitize()).ShouldNot(HaveOccurred())

			//Success when properly provided source packages
			spec := &v1.LiveISO{
				RootFS: []*v1.ImageSource{
					v1.NewDirSrc("/system/os"),
				},
			}
			spec.BootloaderInRootFs = true
			Expect(spec.Sanitize()).ShouldNot(HaveOccurred())
			Expect(iso.Sanitize()).ShouldNot(HaveOccurred())

			//Fails when packages were provided in incorrect format
			spec = &v1.LiveISO{
				RootFS: []*v1.ImageSource{
					nil,
				},
				UEFI: []*v1.ImageSource{
					nil,
				},
				Image: []*v1.ImageSource{
					nil,
				},
			}
			Expect(spec.Sanitize()).Should(HaveOccurred())
		})
	})
})
