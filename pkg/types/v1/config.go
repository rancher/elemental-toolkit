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

package v1

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/rancher/elemental-cli/pkg/constants"
	"k8s.io/mount-utils"
)

const (
	GPT   = "gpt"
	BIOS  = "bios"
	MSDOS = "msdos"
	EFI   = "efi"
	esp   = "esp"
	bios  = "bios_grub"
	boot  = "boot"
)

// Config is the struct that includes basic and generic configuration of elemental binary runtime.
// It mostly includes the interfaces used around many methods in elemental code
type Config struct {
	Logger                    Logger
	Fs                        FS
	Mounter                   mount.Interface
	Runner                    Runner
	Syscall                   SyscallInterface
	CloudInitRunner           CloudInitRunner
	Luet                      LuetInterface
	Client                    HTTPClient
	Cosign                    bool         `yaml:"cosign,omitempty" mapstructure:"cosign"`
	Verify                    bool         `yaml:"verify,omitempty" mapstructure:"verify"`
	CosignPubKey              string       `yaml:"cosign-key,omitempty" mapstructure:"cosign-key"`
	LocalImage                bool         `yaml:"local,omitempty" mapstructure:"local"`
	Repos                     []Repository `yaml:"repositories,omitempty" mapstructure:"repositories"`
	Arch                      string       `yaml:"arch,omitempty" mapstructure:"arch"`
	SquashFsCompressionConfig []string     `yaml:"squash-compression,omitempty" mapstructure:"squash-compression"`
	SquashFsNoCompression     bool         `yaml:"squash-no-compression,omitempty" mapstructure:"squash-no-compression"`
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (c *Config) Sanitize() error {
	// Set Luet plugins, we only use the mtree plugin for now
	if c.Verify {
		c.Luet.SetPlugins(constants.LuetMtreePlugin)
	}
	// If no squashcompression is set, zero the compression parameters
	// By default on NewConfig the SquashFsCompressionConfig is set to the default values, and then override
	// on config unmarshall.
	if c.SquashFsNoCompression {
		c.SquashFsCompressionConfig = []string{}
	}
	// Ensure luet arch matches Config.Arch
	c.Luet.SetArch(c.Arch)
	return nil
}

type RunConfig struct {
	Strict         bool     `yaml:"strict,omitempty" mapstructure:"strict"`
	Reboot         bool     `yaml:"reboot,omitempty" mapstructure:"reboot"`
	PowerOff       bool     `yaml:"poweroff,omitempty" mapstructure:"poweroff"`
	CloudInitPaths []string `yaml:"cloud-init-paths,omitempty" mapstructure:"cloud-init-paths"`
	EjectCD        bool     `yaml:"eject-cd,omitempty" mapstructure:"eject-cd"`

	// 'inline' and 'squash' labels ensure config fields
	// are embedded from a yaml and map PoV
	Config `yaml:",inline" mapstructure:",squash"`
}

// Sanitize checks the consistency of the struct, returns error
//if unsolvable inconsistencies are found
func (r *RunConfig) Sanitize() error {
	return r.Config.Sanitize()
}

// InstallSpec struct represents all the installation action details
type InstallSpec struct {
	Target       string              `yaml:"target,omitempty" mapstructure:"target"`
	Firmware     string              `yaml:"firmware,omitempty" mapstructure:"firmware"`
	PartTable    string              `yaml:"part-table,omitempty" mapstructure:"part-table"`
	Partitions   ElementalPartitions `yaml:"partitions,omitempty" mapstructure:"partitions"`
	NoFormat     bool                `yaml:"no-format,omitempty" mapstructure:"no-format"`
	Force        bool                `yaml:"force,omitempty" mapstructure:"force"`
	CloudInit    string              `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	Iso          string              `yaml:"iso,omitempty" mapstructure:"iso"`
	GrubDefEntry string              `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	Tty          string              `yaml:"tty,omitempty" mapstructure:"tty"`
	Active       Image               `yaml:"system,omitempty" mapstructure:"system"`
	Recovery     Image               `yaml:"recovery-system,omitempty" mapstructure:"recovery-system"`
	Passive      Image
	GrubConf     string
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (i *InstallSpec) Sanitize() error {
	if i.Active.Source.IsEmpty() && i.Iso == "" {
		return fmt.Errorf("undefined system source to install")
	}
	if i.Partitions.State == nil || i.Partitions.State.MountPoint == "" {
		return fmt.Errorf("undefined state partition")
	}
	// Set the image file name depending on the filesystem
	recoveryMnt := constants.RecoveryDir
	if i.Partitions.Recovery != nil && i.Partitions.Recovery.MountPoint != "" {
		recoveryMnt = i.Partitions.Recovery.MountPoint
	}
	if i.Recovery.FS == constants.SquashFs {
		i.Recovery.File = filepath.Join(recoveryMnt, "cOS", constants.RecoverySquashFile)
	} else {
		i.Recovery.File = filepath.Join(recoveryMnt, "cOS", constants.RecoveryImgFile)
	}
	return i.Partitions.SetFirmwarePartitions(i.Firmware, i.PartTable)
}

// ResetSpec struct represents all the reset action details
type ResetSpec struct {
	FormatPersistent bool `yaml:"reset-persistent,omitempty" mapstructure:"reset-persistent"`
	FormatOEM        bool `yaml:"reset-oem,omitempty" mapstructure:"reset-oem"`

	GrubDefEntry string `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	Tty          string `yaml:"tty,omitempty" mapstructure:"tty"`
	Active       Image  `yaml:"system,omitempty" mapstructure:"system"`
	Passive      Image
	Partitions   ElementalPartitions
	Target       string
	Efi          bool
	GrubConf     string
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (r *ResetSpec) Sanitize() error {
	if r.Active.Source.IsEmpty() {
		return fmt.Errorf("undefined system source to reset to")
	}
	if r.Partitions.State == nil || r.Partitions.State.MountPoint == "" {
		return fmt.Errorf("undefined state partition")
	}
	return nil
}

type UpgradeSpec struct {
	RecoveryUpgrade bool   `yaml:"recovery,omitempty" mapstructure:"recovery"`
	Active          Image  `yaml:"system,omitempty" mapstructure:"system"`
	Recovery        Image  `yaml:"recovery-system,omitempty" mapstructure:"recovery-system"`
	GrubDefEntry    string `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	Passive         Image
	Partitions      ElementalPartitions
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (u *UpgradeSpec) Sanitize() error {
	if u.RecoveryUpgrade {
		if u.Partitions.Recovery == nil || u.Partitions.Recovery.MountPoint == "" {
			return fmt.Errorf("undefined recovery partition")
		}
		if u.Recovery.Source.IsEmpty() {
			return fmt.Errorf("undefined upgrade source")
		}
	} else {
		if u.Partitions.State == nil || u.Partitions.State.MountPoint == "" {
			return fmt.Errorf("undefined state partition")
		}
		if u.Active.Source.IsEmpty() {
			return fmt.Errorf("undefined upgrade source")
		}
	}
	return nil
}

// Partition struct represents a partition with its commonly configurable values, size in MiB
type Partition struct {
	Name            string
	FilesystemLabel string   `yaml:"label,omitempty" mapstructure:"label"`
	Size            uint     `yaml:"size,omitempty" mapstructure:"size"`
	FS              string   `yaml:"fs,omitempty" mapstrcuture:"fs"`
	Flags           []string `yaml:"flags,omitempty" mapstrcuture:"flags"`
	MountPoint      string
	Path            string
	Disk            string
}

type PartitionList []*Partition

// GetByName gets a partitions by its name from the PartitionList
func (pl PartitionList) GetByName(name string) *Partition {
	for _, p := range pl {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// GetByLabel gets a partition by its label from the PartitionList
func (pl PartitionList) GetByLabel(label string) *Partition {
	for _, p := range pl {
		if p.FilesystemLabel == label {
			return p
		}
	}
	return nil
}

type ElementalPartitions struct {
	BIOS       *Partition
	EFI        *Partition
	OEM        *Partition `yaml:"oem,omitempty" mapstructure:"oem"`
	Recovery   *Partition `yaml:"recovery,omitempty" mapstructure:"recovery"`
	State      *Partition `yaml:"state,omitempty" mapstructure:"state"`
	Persistent *Partition `yaml:"persistent,omitempty" mapstructure:"persistent"`
}

// SetFirmwarePartitions sets firmware partitions for a given firmware and partition table type
func (ep *ElementalPartitions) SetFirmwarePartitions(firmware string, partTable string) error {
	if firmware == EFI && partTable == GPT {
		ep.EFI = &Partition{
			FilesystemLabel: constants.EfiLabel,
			Size:            constants.EfiSize,
			Name:            constants.EfiPartName,
			FS:              constants.EfiFs,
			MountPoint:      constants.EfiDir,
			Flags:           []string{esp},
		}
		ep.BIOS = nil
	} else if firmware == BIOS && partTable == GPT {
		ep.BIOS = &Partition{
			FilesystemLabel: "",
			Size:            constants.BiosSize,
			Name:            constants.BiosPartName,
			FS:              "",
			MountPoint:      "",
			Flags:           []string{bios},
		}
		ep.EFI = nil
	} else {
		if ep.State == nil {
			return fmt.Errorf("nil state partition")
		}
		ep.State.Flags = []string{boot}
		ep.EFI = nil
		ep.BIOS = nil
	}
	return nil
}

// NewElementalPartitionsFromList fills an ElementalPartitions instance from given
// partitions list. First tries to match partitions by partition label, if not,
// it tries to match partitions by default filesystem label
// TODO find a way to map custom labels when partition labels are not available
func NewElementalPartitionsFromList(pl PartitionList) ElementalPartitions {
	ep := ElementalPartitions{}
	ep.BIOS = pl.GetByName(constants.BiosPartName)
	ep.EFI = pl.GetByName(constants.EfiPartName)
	if ep.EFI == nil {
		ep.EFI = pl.GetByLabel(constants.EfiLabel)
	}
	ep.OEM = pl.GetByName(constants.OEMPartName)
	if ep.OEM == nil {
		ep.OEM = pl.GetByLabel(constants.OEMLabel)
	}
	ep.Recovery = pl.GetByName(constants.RecoveryPartName)
	if ep.Recovery == nil {
		ep.Recovery = pl.GetByLabel(constants.RecoveryLabel)
	}
	ep.State = pl.GetByName(constants.StatePartName)
	if ep.State == nil {
		ep.State = pl.GetByLabel(constants.StateLabel)
	}
	ep.Persistent = pl.GetByName(constants.PersistentPartName)
	if ep.Persistent == nil {
		ep.Persistent = pl.GetByLabel(constants.PersistentLabel)
	}
	return ep
}

// PartitionsByInstallOrder sorts partitions according to the default layout
// nil partitons are ignored
func (ep ElementalPartitions) PartitionsByInstallOrder() PartitionList {
	partitions := PartitionList{}
	if ep.BIOS != nil {
		partitions = append(partitions, ep.BIOS)
	}
	if ep.EFI != nil {
		partitions = append(partitions, ep.EFI)
	}
	if ep.OEM != nil {
		partitions = append(partitions, ep.OEM)
	}
	if ep.Recovery != nil {
		partitions = append(partitions, ep.Recovery)
	}
	if ep.State != nil {
		partitions = append(partitions, ep.State)
	}
	if ep.Persistent != nil {
		partitions = append(partitions, ep.Persistent)
	}
	return partitions
}

// PartitionsByMountPoint sorts partitions according to its mountpoint, ignores nil
// partitions or partitions with an empty mountpoint
func (ep ElementalPartitions) PartitionsByMountPoint(descending bool) PartitionList {
	mountPointKeys := map[string]*Partition{}
	mountPoints := []string{}
	partitions := PartitionList{}

	for _, p := range ep.PartitionsByInstallOrder() {
		if p.MountPoint != "" {
			mountPointKeys[p.MountPoint] = p
			mountPoints = append(mountPoints, p.MountPoint)
		}
	}

	if descending {
		sort.Sort(sort.Reverse(sort.StringSlice(mountPoints)))
	} else {
		sort.Strings(mountPoints)
	}

	for _, mnt := range mountPoints {
		partitions = append(partitions, mountPointKeys[mnt])
	}
	return partitions
}

// Image struct represents a file system image with its commonly configurable values, size in MiB
type Image struct {
	File       string
	Label      string       `yaml:"label,omitempty" mapstructure:"label"`
	Size       uint         `yaml:"size,omitempty" mapstructure:"size"`
	FS         string       `yaml:"fs,omitempty" mapstructure:"fs"`
	Source     *ImageSource `yaml:"uri,omitempty" mapstructure:"uri"`
	MountPoint string
	LoopDevice string
}

// LiveISO represents the configurations needed for a live ISO image
type LiveISO struct {
	RootFS      []*ImageSource `yaml:"rootfs,omitempty" mapstructure:"rootfs"`
	UEFI        []*ImageSource `yaml:"uefi,omitempty" mapstructure:"uefi"`
	Image       []*ImageSource `yaml:"image,omitempty" mapstructure:"image"`
	Label       string         `yaml:"label,omitempty" mapstructure:"label"`
	BootCatalog string         `yaml:"boot-catalog,omitempty" mapstructure:"boot-catalog"`
	BootFile    string         `yaml:"boot-file,omitempty" mapstructure:"boot-file"`
	HybridMBR   string         `yaml:"hybrid-mbr,omitempty" mapstructure:"hybrid-mbr,omitempty"`
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (i *LiveISO) Sanitize() error {
	for _, src := range i.RootFS {
		if src == nil {
			return fmt.Errorf("wrong name of source package for rootfs")
		}
	}
	for _, src := range i.UEFI {
		if src == nil {
			return fmt.Errorf("wrong name of source package for uefi")
		}
	}
	for _, src := range i.Image {
		if src == nil {
			return fmt.Errorf("wrong name of source package for image")
		}
	}

	return nil
}

// Repository represents the basic configuration for a package repository
type Repository struct {
	Name        string `yaml:"name,omitempty" mapstructure:"name"`
	Priority    int    `yaml:"priority,omitempty" mapstructure:"priority"`
	URI         string `yaml:"uri,omitempty" mapstructure:"uri"`
	Type        string `yaml:"type,omitempty" mapstructure:"type"`
	Arch        string `yaml:"arch,omitempty" mapstructure:"arch"`
	ReferenceID string `yaml:"reference,omitempty" mapstructure:"reference"`
}

// BuildConfig represents the config we need for building isos, raw images, artifacts
type BuildConfig struct {
	Date   bool   `yaml:"date,omitempty" mapstructure:"date"`
	Name   string `yaml:"name,omitempty" mapstructure:"name"`
	OutDir string `yaml:"output,omitempty" mapstructure:"output"`

	// 'inline' and 'squash' labels ensure config fields
	// are embedded from a yaml and map PoV
	Config `yaml:",inline" mapstructure:",squash"`
}

// Sanitize checks the consistency of the struct, returns error
//if unsolvable inconsistencies are found
func (b *BuildConfig) Sanitize() error {
	return b.Config.Sanitize()
}

type RawDisk struct {
	X86_64 *RawDiskArchEntry `yaml:"x86_64,omitempty" mapstructure:"x86_64"` //nolint:revive
	Arm64  *RawDiskArchEntry `yaml:"arm64,omitempty" mapstructure:"arm64"`
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (d *RawDisk) Sanitize() error {
	// No checks for the time being
	return nil
}

// RawDiskArchEntry represents an arch entry in raw_disk
type RawDiskArchEntry struct {
	Packages []RawDiskPackage `yaml:"packages,omitempty"`
}

// RawDiskPackage represents a package entry for raw_disk, with a package name and a target to install to
type RawDiskPackage struct {
	Name   string `yaml:"name,omitempty"`
	Target string `yaml:"target,omitempty"`
}
