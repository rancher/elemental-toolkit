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

package types_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/config"
	conf "github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	v1mocks "github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("Write and load installation state", func() {
		var config *types.RunConfig
		var runner *v1mocks.FakeRunner
		var fs vfs.FS
		var mounter *v1mocks.FakeMounter
		var cleanup func()
		var err error
		var systemState *types.SystemState
		var installState *types.InstallState
		var statePath, recoveryPath string

		BeforeEach(func() {
			runner = v1mocks.NewFakeRunner()
			mounter = v1mocks.NewFakeMounter()
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())

			config = conf.NewRunConfig(
				conf.WithFs(fs),
				conf.WithRunner(runner),
				conf.WithMounter(mounter),
			)
			systemState = &types.SystemState{
				Source: types.NewDockerSrc("registry.org/my/image:tag"),
				Label:  "active_label",
				FS:     "ext2",
				Digest: "adadgadg",
			}
			installState = &types.InstallState{
				Date: "somedate",
				Snapshotter: types.SnapshotterConfig{
					Type:     "loopdevice",
					MaxSnaps: 7,
					Config: &types.LoopDeviceConfig{
						Size: 1024,
						FS:   constants.SquashFs,
					},
				},
				Partitions: map[string]*types.PartitionState{
					"state": {
						FSLabel: "state_label",
						Snapshots: map[int]*types.SystemState{
							1: systemState,
						},
					},
					"recovery": {
						FSLabel:       "state_label",
						RecoveryImage: systemState,
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
			incompleteState += "  1:\n"
			incompleteState += "    dir:///some/root\n"
			incompleteState += "recovery:\n"
			incompleteState += "  recovery:\n"
			incompleteState += "    source: dir:///tmp/new_root\n"
			Expect(fs.WriteFile(statePath, []byte(incompleteState), constants.FilePerm)).To(Succeed())

			loadedInstallState, err := config.LoadInstallState()

			Expect(err).ShouldNot(HaveOccurred())
			Expect(loadedInstallState.Partitions[constants.RecoveryPartName].FSLabel).To(Equal(constants.RecoveryLabel))
			Expect(loadedInstallState.Partitions[constants.StatePartName].FSLabel).To(Equal(constants.StateLabel))
		})
		It("Loads a state file with other missing fields", func() {
			incompleteState := "state:\n"
			incompleteState += "  1:\n"
			incompleteState += "    dir:///some/root\n"
			Expect(fs.WriteFile(statePath, []byte(incompleteState), constants.FilePerm)).To(Succeed())

			loadedInstallState, err := config.LoadInstallState()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(loadedInstallState.Partitions[constants.RecoveryPartName]).To(BeNil())
			Expect(loadedInstallState.Partitions[constants.StatePartName].FSLabel).To(Equal(constants.StateLabel))
		})
	})
	Describe("ElementalPartitions", func() {
		var p types.PartitionList
		var ep types.ElementalPartitions
		BeforeEach(func() {
			ep = types.ElementalPartitions{}
			p = types.PartitionList{
				&types.Partition{
					FilesystemLabel: "COS_OEM",
					Size:            0,
					Name:            "oem",
					FS:              "",
					Flags:           nil,
					MountPoint:      "/some/path/nested",
					Path:            "",
					Disk:            "",
				},
				&types.Partition{
					FilesystemLabel: "COS_CUSTOM",
					Size:            0,
					Name:            "persistent",
					FS:              "",
					Flags:           nil,
					MountPoint:      "/some/path",
					Path:            "",
					Disk:            "",
				},
				&types.Partition{
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
		It("sets firmware partitions on Boot", func() {
			Expect(ep.Boot == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(types.EFI, types.GPT)
			Expect(err).Should(HaveOccurred())
		})
		It("sets firmware partitions on bios", func() {
			Expect(ep.Boot == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(types.BIOS, types.GPT)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ep.Boot == nil && ep.BIOS != nil).To(BeTrue())
		})
		It("sets firmware partitions on msdos", func() {
			ep.State = &types.Partition{}
			Expect(ep.Boot == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(types.BIOS, types.MSDOS)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ep.Boot == nil && ep.BIOS == nil).To(BeTrue())
			Expect(ep.State.Flags != nil && ep.State.Flags[0] == "boot").To(BeTrue())
		})
		It("fails to set firmware partitions of state is not defined on msdos", func() {
			Expect(ep.Boot == nil && ep.BIOS == nil).To(BeTrue())
			err := ep.SetFirmwarePartitions(types.BIOS, types.MSDOS)
			Expect(err).Should(HaveOccurred())
		})
		It("initializes an ElementalPartitions from a PartitionList", func() {
			// Use custom label for recovery partition
			ep := types.NewElementalPartitionsFromList(p, &types.InstallState{
				Partitions: map[string]*types.PartitionState{
					constants.RecoveryPartName: {
						FSLabel: "SOMETHING",
					},
				},
			})
			Expect(ep.Persistent != nil).To(BeTrue())
			Expect(ep.OEM != nil).To(BeTrue())
			Expect(ep.BIOS == nil).To(BeTrue())
			Expect(ep.Boot == nil).To(BeTrue())
			Expect(ep.State == nil).To(BeTrue())
			Expect(ep.Recovery != nil).To(BeTrue())
		})
		Describe("returns a partition list by install order", func() {
			It("with no extra parts", func() {
				ep := types.NewElementalPartitionsFromList(p, nil)
				lst := ep.PartitionsByInstallOrder([]*types.Partition{})
				Expect(len(lst)).To(Equal(2))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
			})
			It("with extra parts with size > 0", func() {
				// Use custom label for state partition
				ep := types.NewElementalPartitionsFromList(p, &types.InstallState{
					Partitions: map[string]*types.PartitionState{
						constants.StatePartName: {
							FSLabel: "SOMETHING",
						},
					},
				})
				var extraParts []*types.Partition
				extraParts = append(extraParts, &types.Partition{Name: "extra", Size: 5})

				lst := ep.PartitionsByInstallOrder(extraParts)
				Expect(len(lst)).To(Equal(4))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "somename").To(BeTrue())
				Expect(lst[2].Name == "extra").To(BeTrue())
				Expect(lst[3].Name == "persistent").To(BeTrue())
			})
			It("with extra part with size == 0 and persistent.Size == 0", func() {
				ep := types.NewElementalPartitionsFromList(p, &types.InstallState{})
				var extraParts []*types.Partition
				extraParts = append(extraParts, &types.Partition{Name: "extra", Size: 0})
				lst := ep.PartitionsByInstallOrder(extraParts)
				// Should ignore the wrong partition had have the persistent over it
				Expect(len(lst)).To(Equal(2))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
			})
			It("with extra part with size == 0 and persistent.Size > 0", func() {
				ep := types.NewElementalPartitionsFromList(p, nil)
				ep.Persistent.Size = 10
				var extraParts []*types.Partition
				extraParts = append(extraParts, &types.Partition{Name: "extra", FilesystemLabel: "LABEL", Size: 0})
				lst := ep.PartitionsByInstallOrder(extraParts)
				// Will have our size == 0 partition the latest
				Expect(len(lst)).To(Equal(3))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
				Expect(lst[2].Name == "extra").To(BeTrue())
			})
			It("with several extra parts with size == 0 and persistent.Size > 0", func() {
				ep := types.NewElementalPartitionsFromList(p, nil)
				ep.Persistent.Size = 10
				var extraParts []*types.Partition
				extraParts = append(extraParts, &types.Partition{Name: "extra1", Size: 0})
				extraParts = append(extraParts, &types.Partition{Name: "extra2", Size: 0})
				lst := ep.PartitionsByInstallOrder(extraParts)
				// Should ignore the wrong partition had have the first partition with size 0 added last
				Expect(len(lst)).To(Equal(3))
				Expect(lst[0].Name == "oem").To(BeTrue())
				Expect(lst[1].Name == "persistent").To(BeTrue())
				Expect(lst[2].Name == "extra1").To(BeTrue())
			})
		})

		It("returns a partition list by mount order", func() {
			ep := types.NewElementalPartitionsFromList(p, nil)
			lst := ep.PartitionsByMountPoint(false)
			Expect(len(lst)).To(Equal(2))
			Expect(lst[0].Name == "persistent").To(BeTrue())
			Expect(lst[1].Name == "oem").To(BeTrue())
		})
		It("returns a partition list by mount reverse order", func() {
			ep := types.NewElementalPartitionsFromList(p, nil)
			lst := ep.PartitionsByMountPoint(true)
			Expect(len(lst)).To(Equal(2))
			Expect(lst[0].Name == "oem").To(BeTrue())
			Expect(lst[1].Name == "persistent").To(BeTrue())
		})
	})
	Describe("Partitionlist", func() {
		var p types.PartitionList
		BeforeEach(func() {
			p = types.PartitionList{
				&types.Partition{
					FilesystemLabel: "ONE",
					Size:            0,
					Name:            "one",
					FS:              "",
					Flags:           nil,
					MountPoint:      "",
					Path:            "",
					Disk:            "",
				},
				&types.Partition{
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
			Expect(p.GetByName("two")).To(Equal(&types.Partition{
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
			Expect(p.GetByLabel("TWO")).To(Equal(&types.Partition{
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
		var spec *types.InstallSpec

		BeforeEach(func() {
			cfg := config.NewConfig(config.WithMounter(v1mocks.NewFakeMounter()))
			spec = config.NewInstallSpec(*cfg)
		})
		Describe("sanitize", func() {
			It("runs method", func() {
				Expect(spec.Partitions.Boot).ToNot(BeNil())
				Expect(spec.System.IsEmpty()).To(BeTrue())

				// Creates firmware partitions
				spec.System = types.NewDirSrc("/dir")
				spec.Firmware = types.EFI
				err := spec.Sanitize()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(spec.Partitions.Boot).NotTo(BeNil())

				// Sets image labels to empty string on squashfs
				spec.RecoverySystem.FS = constants.SquashFs
				err = spec.Sanitize()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(spec.RecoverySystem.Label).To(BeEmpty())

				// Fails without state partition
				spec.Partitions.State = nil
				err = spec.Sanitize()
				Expect(err).Should(HaveOccurred())

				// Fails without an install source
				spec.System = types.NewEmptySrc()
				err = spec.Sanitize()
				Expect(err).Should(HaveOccurred())
			})
			Describe("with extra partitions", func() {
				BeforeEach(func() {
					// Set a source for the install
					spec.System = types.NewDirSrc("/dir")
				})
				It("fails if persistent and an extra partition have size == 0", func() {
					spec.ExtraPartitions = append(spec.ExtraPartitions, &types.Partition{Size: 0})
					err := spec.Sanitize()
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("both persistent partition and extra partitions have size set to 0"))
				})
				It("fails if more than one extra partition has size == 0", func() {
					spec.Partitions.Persistent.Size = 10
					spec.ExtraPartitions = append(spec.ExtraPartitions, &types.Partition{Name: "1", Size: 0})
					spec.ExtraPartitions = append(spec.ExtraPartitions, &types.Partition{Name: "2", Size: 0})
					err := spec.Sanitize()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("more than one extra partition has its size set to 0"))
				})
				It("does not fail if persistent size is > 0 and an extra partition has size == 0", func() {
					spec.ExtraPartitions = append(spec.ExtraPartitions, &types.Partition{Size: 0})
					spec.Partitions.Persistent.Size = 10
					err := spec.Sanitize()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
	Describe("ResetSpec", func() {
		It("runs sanitize method", func() {
			spec := &types.ResetSpec{
				System: types.NewDirSrc("/dir"),
				Partitions: types.ElementalPartitions{
					State: &types.Partition{
						MountPoint: "mountpoint",
					},
				},
			}
			err := spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())

			//Fails on missing state partition
			spec.Partitions.State = nil
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())

			//Fails on empty source
			spec.System = types.NewEmptySrc()
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("UpgradeSpec", func() {
		It("runs sanitize method", func() {
			spec := &types.UpgradeSpec{
				System: types.NewDirSrc("/dir"),
				RecoverySystem: types.Image{
					Source: types.NewDirSrc("/dir"),
					Label:  "SOMELABEL",
				},
				Partitions: types.ElementalPartitions{
					State: &types.Partition{
						MountPoint: "mountpoint",
					},
					Recovery: &types.Partition{
						MountPoint: "mountpoint",
					},
				},
			}
			err := spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())

			// Sets image labels to empty string on squashfs
			spec.RecoverySystem.FS = constants.SquashFs
			err = spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(spec.RecoverySystem.Label).To(BeEmpty())

			//Fails on empty source for active upgrade
			spec.System = types.NewEmptySrc()
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())

			//Sets recovery source to system source if empty
			spec.System = types.NewDockerSrc("some/image:tag")
			spec.RecoveryUpgrade = true
			spec.RecoverySystem.Source = types.NewEmptySrc()
			err = spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(spec.RecoverySystem.Source.Value()).To(Equal(spec.System.Value()))

			//Fails on missing state partition for active upgrade
			spec.Partitions.State = nil
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
			spec := &types.LiveISO{
				RootFS: []*types.ImageSource{
					types.NewDirSrc("/system/os"),
				},
			}
			spec.BootloaderInRootFs = true
			Expect(spec.Sanitize()).ShouldNot(HaveOccurred())
			Expect(iso.Sanitize()).ShouldNot(HaveOccurred())

			//Fails when packages were provided in incorrect format
			spec = &types.LiveISO{
				RootFS: []*types.ImageSource{
					nil,
				},
				UEFI: []*types.ImageSource{
					nil,
				},
				Image: []*types.ImageSource{
					nil,
				},
			}
			Expect(spec.Sanitize()).Should(HaveOccurred())
		})
	})
	Describe("MountSpec", func() {
		It("sanitizes empty paths", func() {
			spec := types.MountSpec{
				Ephemeral: types.EphemeralMounts{
					Type:  constants.Tmpfs,
					Paths: []string{"/var", "", "/etc"},
				},
				Persistent: types.PersistentMounts{
					Mode:  constants.OverlayMode,
					Paths: []string{"/etc/rancher", "", "/root"},
				},
			}

			Expect(spec.Sanitize()).To(Succeed())

			Expect(spec.Ephemeral.Paths).To(Equal([]string{"/var", "/etc"}))
			Expect(spec.Persistent.Paths).To(Equal([]string{"/root", "/etc/rancher"}))
		})
	})
	Describe("KeyValuePair", func() {
		It("should decode from comma separated string", func() {
			input := "myFirstLabel=foo,mySecondLabel=bar"
			wantLabels := types.KeyValuePair{"myFirstLabel": "foo", "mySecondLabel": "bar"}
			Expect(types.KeyValuePairFromData(input)).Should(Equal(wantLabels))
		})
	})
})
