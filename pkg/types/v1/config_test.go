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

package v1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/config"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	v1mocks "github.com/rancher-sandbox/elemental/tests/mocks"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("ElementalPartitions", func() {
		var p v1.PartitionList
		var ep v1.ElementalPartitions
		BeforeEach(func() {
			ep = v1.ElementalPartitions{}
			p = v1.PartitionList{
				&v1.Partition{
					Label:      "COS_OEM",
					Size:       0,
					Name:       "oem",
					FS:         "",
					Flags:      nil,
					MountPoint: "/some/path/nested",
					Path:       "",
					Disk:       "",
				},
				&v1.Partition{
					Label:      "COS_CUSTOM",
					Size:       0,
					Name:       "persistent",
					FS:         "",
					Flags:      nil,
					MountPoint: "/some/path",
					Path:       "",
					Disk:       "",
				},
				&v1.Partition{
					Label:      "SOMETHING",
					Size:       0,
					Name:       "somethingelse",
					FS:         "",
					Flags:      nil,
					MountPoint: "",
					Path:       "",
					Disk:       "",
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
		It("initalized an ElementalPartitions from a PartitionList", func() {
			ep := v1.NewElementalPartitionsFromList(p)
			Expect(ep.Persistent != nil).To(BeTrue())
			Expect(ep.OEM != nil).To(BeTrue())
			Expect(ep.BIOS == nil).To(BeTrue())
			Expect(ep.EFI == nil).To(BeTrue())
			Expect(ep.State == nil).To(BeTrue())
			Expect(ep.Recovery == nil).To(BeTrue())
		})
		It("returns a partition list by install order", func() {
			ep := v1.NewElementalPartitionsFromList(p)
			lst := ep.PartitionsByInstallOrder()
			Expect(len(lst)).To(Equal(2))
			Expect(lst[0].Name == "oem").To(BeTrue())
			Expect(lst[1].Name == "persistent").To(BeTrue())
		})
		It("returns a partition list by mount order", func() {
			ep := v1.NewElementalPartitionsFromList(p)
			lst := ep.PartitionsByMountPoint(false)
			Expect(len(lst)).To(Equal(2))
			Expect(lst[0].Name == "persistent").To(BeTrue())
			Expect(lst[1].Name == "oem").To(BeTrue())
		})
		It("returns a partition list by mount reverse order", func() {
			ep := v1.NewElementalPartitionsFromList(p)
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
					Label:      "ONE",
					Size:       0,
					Name:       "one",
					FS:         "",
					Flags:      nil,
					MountPoint: "",
					Path:       "",
					Disk:       "",
				},
				&v1.Partition{
					Label:      "TWO",
					Size:       0,
					Name:       "two",
					FS:         "",
					Flags:      nil,
					MountPoint: "",
					Path:       "",
					Disk:       "",
				},
			}
		})
		It("returns partitions by name", func() {
			Expect(p.GetByName("two")).To(Equal(&v1.Partition{
				Label:      "TWO",
				Size:       0,
				Name:       "two",
				FS:         "",
				Flags:      nil,
				MountPoint: "",
				Path:       "",
				Disk:       "",
			}))
		})
		It("returns nil if partiton name not found", func() {
			Expect(p.GetByName("nonexistent")).To(BeNil())
		})
		It("returns partitions by filesystem label", func() {
			Expect(p.GetByLabel("TWO")).To(Equal(&v1.Partition{
				Label:      "TWO",
				Size:       0,
				Name:       "two",
				FS:         "",
				Flags:      nil,
				MountPoint: "",
				Path:       "",
				Disk:       "",
			}))
		})
		It("returns nil if filesystem label not found", func() {
			Expect(p.GetByName("nonexistent")).To(BeNil())
		})
	})
	Describe("RunConfig", func() {
		It("runs sanitize method", func() {
			cfg := config.NewRunConfig(config.WithMounter(v1mocks.NewErrorMounter()))
			cfg.Config.Verify = true

			// Sets the luet mtree pluing
			err := cfg.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Luet.GetPlugins()).To(Equal([]string{constants.LuetMtreePlugin}))
		})
	})
	Describe("BuildConfig", func() {
		It("runs sanitize method", func() {
			cfg := config.NewBuildConfig(config.WithMounter(v1mocks.NewErrorMounter()))
			cfg.Config.Verify = true

			// Sets the luet mtree pluing
			err := cfg.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Luet.GetPlugins()).To(Equal([]string{constants.LuetMtreePlugin}))
		})
	})
	Describe("InstallSpec", func() {
		It("runs sanitize method", func() {
			cfg := config.NewConfig(config.WithMounter(v1mocks.NewErrorMounter()))
			spec := config.NewInstallSpec(*cfg)
			Expect(spec.Partitions.EFI).To(BeNil())
			Expect(spec.Active.Source.IsEmpty()).To(BeTrue())

			// Creates firmware partitions
			spec.Active.Source = v1.NewDirSrc("/dir")
			spec.Firmware = v1.EFI
			err := spec.Sanitize()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(spec.Partitions.EFI).NotTo(BeNil())

			// Fails without state partition
			spec.Partitions.State = nil
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())

			// Fails without an install source
			spec.Active.Source = v1.NewEmptySrc()
			err = spec.Sanitize()
			Expect(err).Should(HaveOccurred())
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
		})
	})
	Describe("RawDisk", func() {
		It("runs sanitize method", func() {
			disk := &v1.RawDisk{}
			Expect(disk.Sanitize()).ShouldNot(HaveOccurred())
		})
	})
})
