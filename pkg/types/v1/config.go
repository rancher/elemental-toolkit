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

package v1

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"

	"gopkg.in/yaml.v3"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/pkg/constants"
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
	ImageExtractor            ImageExtractor
	Client                    HTTPClient
	Platform                  *Platform `yaml:"platform,omitempty" mapstructure:"platform"`
	Cosign                    bool      `yaml:"cosign,omitempty" mapstructure:"cosign"`
	Verify                    bool      `yaml:"verify,omitempty" mapstructure:"verify"`
	CosignPubKey              string    `yaml:"cosign-key,omitempty" mapstructure:"cosign-key"`
	LocalImage                bool      `yaml:"local,omitempty" mapstructure:"local"`
	Arch                      string    `yaml:"arch,omitempty" mapstructure:"arch"`
	SquashFsCompressionConfig []string  `yaml:"squash-compression,omitempty" mapstructure:"squash-compression"`
	SquashFsNoCompression     bool      `yaml:"squash-no-compression,omitempty" mapstructure:"squash-no-compression"`
	CloudInitPaths            []string  `yaml:"cloud-init-paths,omitempty" mapstructure:"cloud-init-paths"`
	Strict                    bool      `yaml:"strict,omitempty" mapstructure:"strict"`
}

// WriteInstallState writes the state.yaml file to the given state and recovery paths
func (c Config) WriteInstallState(i *InstallState, statePath, recoveryPath string) error {
	data, err := yaml.Marshal(i)
	if err != nil {
		return err
	}

	data = append([]byte("# Autogenerated file by elemental client, do not edit\n\n"), data...)

	err = c.Fs.WriteFile(statePath, data, constants.FilePerm)
	if err != nil {
		return err
	}

	err = c.Fs.WriteFile(recoveryPath, data, constants.FilePerm)
	if err != nil {
		return err
	}

	return nil
}

// LoadInstallState loads the state.yaml file and unmarshals it to an InstallState object
func (c Config) LoadInstallState() (*InstallState, error) {
	installState := &InstallState{}
	data, err := c.Fs.ReadFile(filepath.Join(constants.RunningStateDir, constants.InstallStateFile))
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, installState)
	if err != nil {
		return nil, err
	}

	// Set default filesystem labels if missing, see racnher/elemental-toolkit#1827
	if installState.Partitions[constants.EfiPartName] != nil && installState.Partitions[constants.EfiPartName].FSLabel == "" {
		installState.Partitions[constants.EfiPartName].FSLabel = constants.EfiLabel
	}
	if installState.Partitions[constants.OEMPartName] != nil && installState.Partitions[constants.OEMPartName].FSLabel == "" {
		installState.Partitions[constants.OEMPartName].FSLabel = constants.OEMLabel
	}
	if installState.Partitions[constants.RecoveryPartName] != nil && installState.Partitions[constants.RecoveryPartName].FSLabel == "" {
		installState.Partitions[constants.RecoveryPartName].FSLabel = constants.RecoveryLabel
		recovery := installState.Partitions[constants.RecoveryPartName]
		if recovery.Images[constants.RecoveryImgName] != nil && recovery.Images[constants.RecoveryImgName].Label == "" {
			recovery.Images[constants.RecoveryImgName].Label = constants.SystemLabel
		}
	}
	if installState.Partitions[constants.StatePartName] != nil && installState.Partitions[constants.StatePartName].FSLabel == "" {
		installState.Partitions[constants.StatePartName].FSLabel = constants.StateLabel
		state := installState.Partitions[constants.StatePartName]
		if state.Images[constants.ActiveImgName] != nil && state.Images[constants.ActiveImgName].Label == "" {
			state.Images[constants.ActiveImgName].Label = constants.ActiveLabel
		}
		if state.Images[constants.PassiveImgName] != nil && state.Images[constants.PassiveImgName].Label == "" {
			state.Images[constants.PassiveImgName].Label = constants.PassiveLabel
		}
	}
	if installState.Partitions[constants.PersistentPartName] != nil && installState.Partitions[constants.PersistentPartName].FSLabel == "" {
		installState.Partitions[constants.PersistentPartName].FSLabel = constants.PersistentLabel
	}

	return installState, nil
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (c *Config) Sanitize() error {
	// If no squashcompression is set, zero the compression parameters
	// By default on NewConfig the SquashFsCompressionConfig is set to the default values, and then override
	// on config unmarshall.
	if c.SquashFsNoCompression {
		c.SquashFsCompressionConfig = []string{}
	}

	if c.Arch != "" {
		p, err := NewPlatformFromArch(c.Arch)
		if err != nil {
			return err
		}
		c.Platform = p
	}

	if c.Platform == nil {
		p, err := NewPlatformFromArch(runtime.GOARCH)
		if err != nil {
			return err
		}
		c.Platform = p
	}

	return nil
}

type RunConfig struct {
	Reboot   bool `yaml:"reboot,omitempty" mapstructure:"reboot"`
	PowerOff bool `yaml:"poweroff,omitempty" mapstructure:"poweroff"`
	EjectCD  bool `yaml:"eject-cd,omitempty" mapstructure:"eject-cd"`

	// 'inline' and 'squash' labels ensure config fields
	// are embedded from a yaml and map PoV
	Config `yaml:",inline" mapstructure:",squash"`
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (r *RunConfig) Sanitize() error {
	// Always include default cloud-init paths
	r.CloudInitPaths = append(constants.GetCloudInitPaths(), r.CloudInitPaths...)
	return r.Config.Sanitize()
}

// InstallSpec struct represents all the installation action details
type InstallSpec struct {
	Target           string              `yaml:"target,omitempty" mapstructure:"target"`
	Firmware         string              `yaml:"firmware,omitempty" mapstructure:"firmware"`
	PartTable        string              `yaml:"part-table,omitempty" mapstructure:"part-table"`
	Partitions       ElementalPartitions `yaml:"partitions,omitempty" mapstructure:"partitions"`
	ExtraPartitions  PartitionList       `yaml:"extra-partitions,omitempty" mapstructure:"extra-partitions"`
	NoFormat         bool                `yaml:"no-format,omitempty" mapstructure:"no-format"`
	Force            bool                `yaml:"force,omitempty" mapstructure:"force"`
	CloudInit        []string            `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	Iso              string              `yaml:"iso,omitempty" mapstructure:"iso"`
	GrubDefEntry     string              `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	Active           Image               `yaml:"system,omitempty" mapstructure:"system"`
	Recovery         Image               `yaml:"recovery-system,omitempty" mapstructure:"recovery-system"`
	Passive          Image
	GrubConf         string
	DisableBootEntry bool `yaml:"disable-boot-entry,omitempty" mapstructure:"disable-boot-entry"`
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

	// Unset labels for squashfs filesystem
	if i.Active.FS == constants.SquashFs {
		i.Active.Label = ""
		i.Passive.Label = ""
	}
	if i.Recovery.FS == constants.SquashFs {
		i.Recovery.Label = ""
	}

	// Check for extra partitions having set its size to 0
	extraPartsSizeCheck := 0
	for _, p := range i.ExtraPartitions {
		if p.Size == 0 {
			extraPartsSizeCheck++
		}
	}

	if extraPartsSizeCheck > 1 {
		return fmt.Errorf("more than one extra partition has its size set to 0. Only one partition can have its size set to 0 which means that it will take all the available disk space in the device")
	}
	// Check for both an extra partition and the persistent partition having size set to 0
	if extraPartsSizeCheck == 1 && i.Partitions.Persistent.Size == 0 {
		return fmt.Errorf("both persistent partition and extra partitions have size set to 0. Only one partition can have its size set to 0 which means that it will take all the available disk space in the device")
	}
	return i.Partitions.SetFirmwarePartitions(i.Firmware, i.PartTable)
}

// InitSpec struct represents all the init action details
type InitSpec struct {
	Mkinitrd bool `yaml:"mkinitrd,omitempty" mapstructure:"mkinitrd"`
	Force    bool `yaml:"force,omitempty" mapstructure:"force"`

	Features []string `yaml:"features,omitempty" mapstructure:"features"`
}

// MountSpec struct represents all the mount action details
type MountSpec struct {
	ReadKernelCmdline bool   `yaml:"read-kernel-cmdline,omitempty" mapstructure:"read-kernel-cmdline"`
	WriteFstab        bool   `yaml:"write-fstab,omitempty" mapstructure:"write-fstab"`
	RunCloudInit      bool   `yaml:"run-cloud-init,omitempty" mapstructure:"run-cloud-init"`
	Disable           bool   `yaml:"disable,omitempty" mapstructure:"disable"`
	Sysroot           string `yaml:"sysroot,omitempty" mapstructure:"sysroot"`
	Mode              string `yaml:"mode,omitempty" mapstructure:"mode"`
	Image             *Image `yaml:"image,omitempty" mapstructure:"image"`
	Partitions        ElementalPartitions
	Overlay           OverlayMounts    `yaml:"overlay,omitempty" mapstructure:"overlay"`
	Persistent        PersistentMounts `yaml:"persistent,omitempty" mapstructure:"persistent"`
}

// PersistentMounts struct contains settings for which paths to mount as
// persistent
type PersistentMounts struct {
	Mode  string   `yaml:"mode,omitempty" mapstructure:"mode"`
	Paths []string `yaml:"paths,omitempty" mapstructure:"paths"`
}

// OverlayMounts contains information about the RW overlay mounted over the
// immutable system.
type OverlayMounts struct {
	Type   string   `yaml:"type,omitempty" mapstructure:"type"`
	Device string   `yaml:"device,omitempty" mapstructure:"device"`
	Size   string   `yaml:"size,omitempty" mapstructure:"size"`
	Paths  []string `yaml:"paths,omitempty" mapstructure:"paths"`
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (spec *MountSpec) Sanitize() error {
	switch spec.Persistent.Mode {
	case constants.BindMode, constants.OverlayMode:
		break
	default:
		return fmt.Errorf("unknown persistent mode: '%s'", spec.Persistent.Mode)
	}

	switch spec.Overlay.Type {
	case constants.Tmpfs, constants.Block:
		break
	default:
		return fmt.Errorf("unknown overlay type: '%s'", spec.Overlay.Type)
	}

	// If the Mode is set as an image path we convert it to just say
	// active|passive|recovery and calculate the path below.
	switch spec.Mode {
	case constants.ActiveImgPath:
		spec.Mode = constants.ActiveImgName
	case constants.PassiveImgPath:
		spec.Mode = constants.PassiveImgName
	case constants.RecoveryImgPath:
		spec.Mode = constants.RecoveryImgName
	}

	// Mode should be active|passive|recovery here.
	switch spec.Mode {
	case constants.ActiveImgName:
		spec.Image.Label = constants.ActiveLabel
		spec.Image.File = filepath.Join(constants.RunningStateDir, constants.ActiveImgPath)
	case constants.PassiveImgName:
		spec.Image.Label = constants.PassiveLabel
		spec.Image.File = filepath.Join(constants.RunningStateDir, constants.PassiveImgPath)
	case constants.RecoveryImgName:
		spec.Image.Label = constants.ActiveLabel
		spec.Image.File = filepath.Join(constants.RunningStateDir, constants.RecoveryImgPath)

		spec.Partitions.State.MountPoint = ""
		spec.Partitions.Recovery.MountPoint = constants.RunningStateDir
	default:
		return fmt.Errorf("unknown mode '%s'", spec.Mode)
	}

	spec.Image.Source = NewFileSrc(spec.Image.File)

	return nil
}

// ResetSpec struct represents all the reset action details
type ResetSpec struct {
	FormatPersistent bool `yaml:"reset-persistent,omitempty" mapstructure:"reset-persistent"`
	FormatOEM        bool `yaml:"reset-oem,omitempty" mapstructure:"reset-oem"`

	CloudInit        []string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	GrubDefEntry     string   `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	Active           Image    `yaml:"system,omitempty" mapstructure:"system"`
	Passive          Image
	Partitions       ElementalPartitions
	Target           string
	Efi              bool
	GrubConf         string
	State            *InstallState
	DisableBootEntry bool `yaml:"disable-boot-entry,omitempty" mapstructure:"disable-boot-entry"`
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
	// Unset labels for squashfs filesystem
	if r.Active.FS == constants.SquashFs {
		r.Active.Label = ""
		r.Passive.Label = ""
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
	State           *InstallState
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
	// Unset labels for squashfs filesystem
	if u.Active.FS == constants.SquashFs {
		u.Active.Label = ""
		u.Passive.Label = ""
	}
	if u.Recovery.FS == constants.SquashFs {
		u.Recovery.Label = ""
	}
	return nil
}

// Partition struct represents a partition with its commonly configurable values, size in MiB
type Partition struct {
	Name            string
	FilesystemLabel string   `yaml:"label,omitempty" mapstructure:"label"`
	Size            uint     `yaml:"size,omitempty" mapstructure:"size"`
	FS              string   `yaml:"fs,omitempty" mapstructure:"fs"`
	Flags           []string `yaml:"flags,omitempty" mapstructure:"flags"`
	MountPoint      string
	Path            string
	Disk            string
}

type PartitionList []*Partition

// ToImage returns an image object that matches the partition. This is helpful if the partition
// is managed as an image.
func (p Partition) ToImage() *Image {
	return &Image{
		File:       p.Path,
		Label:      p.FilesystemLabel,
		Size:       p.Size,
		FS:         p.FS,
		Source:     NewEmptySrc(),
		MountPoint: p.MountPoint,
	}
}

// GetByName gets a partitions by its name from the PartitionList
func (pl PartitionList) GetByName(name string) *Partition {
	var part *Partition

	for _, p := range pl {
		if p.Name == name {
			part = p
			if part.MountPoint != "" {
				return part
			}
		}
	}
	return part
}

// GetByLabel gets a partition by its label from the PartitionList
func (pl PartitionList) GetByLabel(label string) *Partition {
	var part *Partition

	for _, p := range pl {
		if p.FilesystemLabel == label {
			part = p
			if part.MountPoint != "" {
				return part
			}
		}
	}
	return part
}

// GetByNameOrLabel gets a partition by its name or label. It tries by name first
func (pl PartitionList) GetByNameOrLabel(name, label string) *Partition {
	part := pl.GetByName(name)
	if part == nil {
		part = pl.GetByLabel(label)
	}
	return part
}

type ElementalPartitions struct {
	BIOS       *Partition
	EFI        *Partition
	OEM        *Partition `yaml:"oem,omitempty" mapstructure:"oem"`
	Recovery   *Partition `yaml:"recovery,omitempty" mapstructure:"recovery"`
	State      *Partition `yaml:"state,omitempty" mapstructure:"state"`
	Persistent *Partition `yaml:"persistent,omitempty" mapstructure:"persistent"`
}

// GetConfigStorage returns the path, usually a mountpoint, of the configuration partition
func (ep ElementalPartitions) GetConfigStorage() string {
	if ep.OEM != nil {
		return ep.OEM.MountPoint
	}
	return ""
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
// it tries to match partitions by filesystem label
func NewElementalPartitionsFromList(pl PartitionList, state *InstallState) ElementalPartitions {
	ep := ElementalPartitions{}

	lm := map[string]string{
		constants.EfiPartName:        constants.EfiLabel,
		constants.OEMPartName:        constants.OEMLabel,
		constants.RecoveryPartName:   constants.RecoveryLabel,
		constants.StatePartName:      constants.StateLabel,
		constants.PersistentPartName: constants.PersistentLabel,
	}
	if state != nil {
		for k := range lm {
			if state.Partitions[k] != nil {
				lm[k] = state.Partitions[k].FSLabel
			}
		}
	}

	ep.BIOS = pl.GetByName(constants.BiosPartName)
	ep.EFI = pl.GetByNameOrLabel(constants.EfiPartName, lm[constants.EfiPartName])
	ep.OEM = pl.GetByNameOrLabel(constants.OEMPartName, lm[constants.OEMPartName])
	ep.Recovery = pl.GetByNameOrLabel(constants.RecoveryPartName, lm[constants.RecoveryPartName])
	ep.State = pl.GetByNameOrLabel(constants.StatePartName, lm[constants.StatePartName])
	ep.Persistent = pl.GetByNameOrLabel(constants.PersistentPartName, lm[constants.PersistentPartName])

	return ep
}

// PartitionsByInstallOrder sorts partitions according to the default layout
// nil partitons are ignored
// partition with 0 size is set last
func (ep ElementalPartitions) PartitionsByInstallOrder(extraPartitions PartitionList, excludes ...*Partition) PartitionList {
	partitions := PartitionList{}
	var lastPartition *Partition

	inExcludes := func(part *Partition, list ...*Partition) bool {
		for _, p := range list {
			if part == p {
				return true
			}
		}
		return false
	}

	if ep.BIOS != nil && !inExcludes(ep.BIOS, excludes...) {
		partitions = append(partitions, ep.BIOS)
	}
	if ep.EFI != nil && !inExcludes(ep.EFI, excludes...) {
		partitions = append(partitions, ep.EFI)
	}
	if ep.OEM != nil && !inExcludes(ep.OEM, excludes...) {
		partitions = append(partitions, ep.OEM)
	}
	if ep.Recovery != nil && !inExcludes(ep.Recovery, excludes...) {
		partitions = append(partitions, ep.Recovery)
	}
	if ep.State != nil && !inExcludes(ep.State, excludes...) {
		partitions = append(partitions, ep.State)
	}
	if ep.Persistent != nil && !inExcludes(ep.Persistent, excludes...) {
		// Check if we have to set this partition the latest due size == 0
		if ep.Persistent.Size == 0 {
			lastPartition = ep.Persistent
		} else {
			partitions = append(partitions, ep.Persistent)
		}
	}
	for _, p := range extraPartitions {
		// Check if we have to set this partition the latest due size == 0
		// Also check that we didn't set already the persistent to last in which case ignore this
		// InstallConfig.Sanitize should have already taken care of failing if this is the case, so this is extra protection
		if p.Size == 0 {
			if lastPartition != nil {
				// Ignore this part, we are not setting 2 parts to have 0 size!
				continue
			}
			lastPartition = p
		} else {
			partitions = append(partitions, p)
		}
	}

	// Set the last partition in the list the partition which has 0 size, so it grows to use the rest of free space
	if lastPartition != nil {
		partitions = append(partitions, lastPartition)
	}

	return partitions
}

// PartitionsByMountPoint sorts partitions according to its mountpoint, ignores nil
// partitions or partitions with an empty mountpoint
func (ep ElementalPartitions) PartitionsByMountPoint(descending bool, excludes ...*Partition) PartitionList {
	mountPointKeys := map[string]*Partition{}
	mountPoints := []string{}
	partitions := PartitionList{}

	for _, p := range ep.PartitionsByInstallOrder([]*Partition{}, excludes...) {
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
	RootFS             []*ImageSource `yaml:"rootfs,omitempty" mapstructure:"rootfs"`
	UEFI               []*ImageSource `yaml:"uefi,omitempty" mapstructure:"uefi"`
	Image              []*ImageSource `yaml:"image,omitempty" mapstructure:"image"`
	Label              string         `yaml:"label,omitempty" mapstructure:"label"`
	GrubEntry          string         `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	BootloaderInRootFs bool           `yaml:"bootloader-in-rootfs" mapstructure:"bootloader-in-rootfs"`
	Firmware           string         `yaml:"firmware,omitempty" mapstructure:"firmware"`
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
// if unsolvable inconsistencies are found
func (b *BuildConfig) Sanitize() error {
	return b.Config.Sanitize()
}

type DiskSpec struct {
	Size         uint                `yaml:"size,omitempty" mapstructure:"size"`
	Partitions   ElementalPartitions `yaml:"partitions,omitempty" mapstructure:"partitions"`
	Expandable   bool                `yaml:"expandable,omitempty" mapstructure:"expandable"`
	Unprivileged bool                `yaml:"unprivileged,omitempty" mapstructure:"unprivileged"`
	Active       Image               `yaml:"system,omitempty" mapstructure:"system"`
	Recovery     Image               `yaml:"recovery-system,omitempty" mapstructure:"recovery-system"`
	Passive      Image
	GrubConf     string
	CloudInit    []string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	GrubDefEntry string   `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	Type         string   `yaml:"type,omitempty" mapstructure:"type"`
	DeployCmd    []string `yaml:"deploy-command,omitempty" mapstructure:"deploy-command"`
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (d *DiskSpec) Sanitize() error {
	// Set passive filesystem as active
	d.Passive.FS = d.Active.FS

	// Unset default label for squashed images
	if d.Active.FS == constants.SquashFs {
		d.Active.Label = ""
		d.Passive.Label = ""
	}
	if d.Recovery.FS == constants.SquashFs {
		d.Recovery.Label = ""
	}

	// The disk size is enough for all partitions
	minSize := d.MinDiskSize()
	if d.Size != 0 && !d.Expandable && d.Size <= minSize {
		return fmt.Errorf("Requested disk size (%dMB) is not enough, it should be, at least, of %d", d.Size, minSize)
	}

	return nil
}

// minDiskSize counts the minimum size (MB) required for the disk given the partitions setup
func (d *DiskSpec) MinDiskSize() uint {
	var minDiskSize uint

	// First partition is aligned at the first 1MB and the last one ends at -1MB
	minDiskSize = 2
	for _, part := range d.Partitions.PartitionsByInstallOrder(PartitionList{}) {
		if part.Size == 0 {
			minDiskSize += constants.MinPartSize
		} else {
			minDiskSize += part.Size
		}
	}

	return minDiskSize
}

// InstallState tracks the installation data of the whole system
type InstallState struct {
	Date       string                     `yaml:"date,omitempty"`
	Partitions map[string]*PartitionState `yaml:",omitempty,inline"`
}

// PartState tracks installation data of a partition
type PartitionState struct {
	FSLabel string                 `yaml:"label,omitempty"`
	Images  map[string]*ImageState `yaml:",omitempty,inline"`
}

// ImageState represents data of a deployed image
type ImageState struct {
	Source         *ImageSource `yaml:"source,omitempty"`
	SourceMetadata interface{}  `yaml:"source-metadata,omitempty"`
	Label          string       `yaml:"label,omitempty"`
	FS             string       `yaml:"fs,omitempty"`
}

func (i *ImageState) UnmarshalYAML(value *yaml.Node) error {
	type iState ImageState
	var srcMeta *yaml.Node
	var err error

	err = value.Decode((*iState)(i))
	if err != nil {
		return err
	}

	if i.SourceMetadata != nil {
		for i, n := range value.Content {
			if n.Value == "source-metadata" && n.Kind == yaml.ScalarNode {
				if len(value.Content) >= i+1 && value.Content[i+1].Kind == yaml.MappingNode {
					srcMeta = value.Content[i+1]
				}
				break
			}
		}
	}

	i.SourceMetadata = nil
	if srcMeta != nil {
		d := &DockerImageMeta{}
		err = srcMeta.Decode(d)
		if err == nil && (d.Digest != "" || d.Size != 0) {
			i.SourceMetadata = d
			return nil
		}
		c := &ChannelImageMeta{}
		err = srcMeta.Decode(c)
		if err == nil && c.Name != "" {
			i.SourceMetadata = c
		}
	}

	return err
}

// DockerImageMeta represents metadata of a docker container image type
type DockerImageMeta struct {
	Digest string `yaml:"digest,omitempty"`
	Size   int64  `yaml:"size,omitempty"`
}

// ChannelImageMeta represents metadata of a channel image type
type ChannelImageMeta struct {
	Category    string       `yaml:"category,omitempty"`
	Name        string       `yaml:"name,omitempty"`
	Version     string       `yaml:"version,omitempty"`
	FingerPrint string       `yaml:"finger-print,omitempty"`
	Repos       []Repository `yaml:"repositories,omitempty"`
}
