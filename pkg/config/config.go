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

package config

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/elemental-toolkit/pkg/cloudinit"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/features"
	"github.com/rancher/elemental-toolkit/pkg/http"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const (
	partSuffix  = ".part"
	mountSuffix = ".mount"
)

type GenericOptions func(a *v1.Config) error

func WithFs(fs v1.FS) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Fs = fs
		return nil
	}
}

func WithLogger(logger v1.Logger) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Logger = logger
		return nil
	}
}

func WithSyscall(syscall v1.SyscallInterface) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Syscall = syscall
		return nil
	}
}

func WithMounter(mounter v1.Mounter) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Mounter = mounter
		return nil
	}
}

func WithRunner(runner v1.Runner) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Runner = runner
		return nil
	}
}

func WithClient(client v1.HTTPClient) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Client = client
		return nil
	}
}

func WithCloudInitRunner(ci v1.CloudInitRunner) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.CloudInitRunner = ci
		return nil
	}
}

func WithPlatform(platform string) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		p, err := v1.ParsePlatform(platform)
		r.Platform = p
		return err
	}
}

func WithOCIImageExtractor() func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.ImageExtractor = v1.OCIImageExtractor{}
		return nil
	}
}

func WithImageExtractor(extractor v1.ImageExtractor) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.ImageExtractor = extractor
		return nil
	}
}

func NewConfig(opts ...GenericOptions) *v1.Config {
	log := v1.NewLogger()

	defaultPlatform, err := v1.NewPlatformFromArch(runtime.GOARCH)
	if err != nil {
		log.Errorf("error parsing default platform (%s): %s", runtime.GOARCH, err.Error())
		return nil
	}

	c := &v1.Config{
		Fs:                        vfs.OSFS,
		Logger:                    log,
		Syscall:                   &v1.RealSyscall{},
		Client:                    http.NewClient(),
		Platform:                  defaultPlatform,
		SquashFsCompressionConfig: constants.GetDefaultSquashfsCompressionOptions(),
	}
	for _, o := range opts {
		err := o(c)
		if err != nil {
			log.Errorf("error applying config option: %s", err.Error())
			return nil
		}
	}

	// delay runner creation after we have run over the options in case we use WithRunner
	if c.Runner == nil {
		c.Runner = &v1.RealRunner{Logger: c.Logger}
	}

	// Now check if the runner has a logger inside, otherwise point our logger into it
	// This can happen if we set the WithRunner option as that doesn't set a logger
	if c.Runner.GetLogger() == nil {
		c.Runner.SetLogger(c.Logger)
	}

	// Delay the yip runner creation, so we set the proper logger instead of blindly setting it to the logger we create
	// at the start of NewRunConfig, as WithLogger can be passed on init, and that would result in 2 different logger
	// instances, on the config.Logger and the other on config.CloudInitRunner
	if c.CloudInitRunner == nil {
		c.CloudInitRunner = cloudinit.NewYipCloudInitRunner(c.Logger, c.Runner, vfs.OSFS)
	}

	if c.Mounter == nil {
		c.Mounter = v1.NewMounter(constants.MountBinary)
	}

	return c
}

func NewRunConfig(opts ...GenericOptions) *v1.RunConfig {
	config := NewConfig(opts...)

	snapshotter := v1.SnapshotterConfig{
		Type:     constants.LoopDeviceSnapshotterType,
		MaxSnaps: constants.MaxSnaps,
		Config:   v1.NewLoopDeviceConfig(),
	}

	// Load snapshotter setup from state.yaml for reset and upgrade
	installState, _ := config.LoadInstallState()
	if installState != nil {
		snapshotter = installState.Snapshotter
	}

	r := &v1.RunConfig{
		Snapshotter: snapshotter,
		Config:      *config,
	}
	return r
}

// NewInstallSpec returns an InstallSpec struct all based on defaults and basic host checks (e.g. EFI vs BIOS)
func NewInstallSpec(cfg v1.Config) *v1.InstallSpec {
	var system *v1.ImageSource
	var recoverySystem v1.Image

	// Check the default ISO installation media is available
	isoRootExists, _ := utils.Exists(cfg.Fs, constants.ISOBaseTree)

	if isoRootExists {
		system = v1.NewDirSrc(constants.ISOBaseTree)
	} else {
		system = v1.NewEmptySrc()
	}

	recoverySystem.Source = system
	recoverySystem.FS = constants.LinuxImgFs
	recoverySystem.Label = constants.SystemLabel
	recoverySystem.File = filepath.Join(constants.RecoveryDir, constants.RecoveryImgFile)
	recoverySystem.MountPoint = constants.TransitionDir

	return &v1.InstallSpec{
		Firmware:       v1.EFI,
		PartTable:      v1.GPT,
		Partitions:     NewInstallElementalPartitions(),
		System:         system,
		RecoverySystem: recoverySystem,
	}
}

// NewInitSpec returns an InitSpec struct all based on defaults
func NewInitSpec() *v1.InitSpec {
	return &v1.InitSpec{
		Mkinitrd: true,
		Force:    false,
		Features: features.All,
	}
}

func NewMountSpec(cfg v1.Config) (*v1.MountSpec, error) {
	// Check current installed system setup and discover partitions
	installState, err := cfg.LoadInstallState()
	if err != nil {
		cfg.Logger.Warnf("failed reading installation state: %s", err.Error())
	}

	// Lists detected partitions on current system including mountpoint if mounted
	parts, err := utils.GetAllPartitions()
	if err != nil {
		return nil, fmt.Errorf("could not read host partitions")
	}

	ep := v1.NewElementalPartitionsFromList(parts, installState)

	if ep.EFI != nil && ep.EFI.MountPoint == "" {
		ep.EFI.MountPoint = constants.EfiDir
		ep.EFI.Flags = []string{"ro", "defaults"}
	}
	if ep.OEM != nil && ep.OEM.MountPoint == "" {
		ep.OEM.MountPoint = constants.OEMDir
	}
	if ep.Persistent != nil && ep.Persistent.MountPoint == "" {
		ep.Persistent.MountPoint = constants.PersistentDir
	}
	if ep.Recovery != nil {
		ep.Recovery.Flags = []string{"ro", "defaults"}
	}
	if ep.State != nil {
		ep.State.Flags = []string{"ro", "defaults"}
	}
	if (ep.Recovery == nil || ep.Recovery.MountPoint == "") &&
		(ep.State == nil || ep.State.MountPoint == "") {
		return nil, fmt.Errorf("neither state or recovery partitions are mounted")
	}

	return &v1.MountSpec{
		Sysroot:    "/sysroot",
		WriteFstab: true,
		Partitions: ep,
		Ephemeral: v1.EphemeralMounts{
			Type:  constants.Tmpfs,
			Size:  "25%",
			Paths: []string{"/var", "/etc", "/srv"},
		},
		Persistent: v1.PersistentMounts{
			Mode:  constants.OverlayMode,
			Paths: []string{"/etc/systemd", "/etc/ssh", "/home", "/opt", "/root", "/var/log"},
		},
	}, nil
}

func NewInstallElementalPartitions() v1.ElementalPartitions {
	partitions := v1.ElementalPartitions{}
	partitions.OEM = &v1.Partition{
		FilesystemLabel: constants.OEMLabel,
		Size:            constants.OEMSize,
		Name:            constants.OEMPartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.OEMDir,
		Flags:           []string{},
	}

	partitions.Recovery = &v1.Partition{
		FilesystemLabel: constants.RecoveryLabel,
		Size:            constants.RecoverySize,
		Name:            constants.RecoveryPartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.RecoveryDir,
		Flags:           []string{},
	}

	partitions.State = &v1.Partition{
		FilesystemLabel: constants.StateLabel,
		Size:            constants.StateSize,
		Name:            constants.StatePartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.StateDir,
		Flags:           []string{},
	}

	partitions.Persistent = &v1.Partition{
		FilesystemLabel: constants.PersistentLabel,
		Size:            constants.PersistentSize,
		Name:            constants.PersistentPartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.PersistentDir,
		Flags:           []string{},
	}

	_ = partitions.SetFirmwarePartitions(v1.EFI, v1.GPT)

	return partitions
}

// getRecoveryState returns recovery state from a given install state. It
// returns default values for any missing field.
func getRecoveryState(state *v1.InstallState) (recovery *v1.SystemState) {
	recovery = &v1.SystemState{
		FS:    constants.SquashFs,
		Label: constants.SystemLabel,
	}

	if state != nil {
		rPart := state.Partitions[constants.RecoveryPartName]
		if rPart != nil {
			if rPart.RecoveryImage != nil {
				recovery = rPart.RecoveryImage
			}
		}
	}

	return recovery
}

// NewUpgradeSpec returns an UpgradeSpec struct all based on defaults and current host state
func NewUpgradeSpec(cfg v1.Config) (*v1.UpgradeSpec, error) {
	var rState *v1.SystemState
	var recovery v1.Image

	installState, err := cfg.LoadInstallState()
	if err != nil {
		cfg.Logger.Warnf("failed reading installation state: %s", err.Error())
	}

	rState = getRecoveryState(installState)

	parts, err := utils.GetAllPartitions()
	if err != nil {
		return nil, fmt.Errorf("could not read host partitions")
	}
	ep := v1.NewElementalPartitionsFromList(parts, installState)

	if ep.Recovery != nil {
		if ep.Recovery.MountPoint == "" {
			ep.Recovery.MountPoint = constants.RecoveryDir
		}

		recovery = v1.Image{
			File:       filepath.Join(ep.Recovery.MountPoint, constants.TransitionImgFile),
			Size:       constants.ImgSize,
			Label:      rState.Label,
			FS:         rState.FS,
			MountPoint: constants.TransitionDir,
			Source:     v1.NewEmptySrc(),
		}
	}

	if ep.State != nil {
		if ep.State.MountPoint == "" {
			ep.State.MountPoint = constants.StateDir
		}
	}

	if ep.EFI != nil {
		if ep.EFI.MountPoint == "" {
			ep.EFI.MountPoint = constants.EfiDir
		}
	}

	// This is needed if we want to use the persistent as tmpdir for the upgrade images
	// as tmpfs is 25% of the total RAM, we cannot rely on the tmp dir having enough space for our image
	// This enables upgrades on low ram devices
	if ep.Persistent != nil {
		if ep.Persistent.MountPoint == "" {
			ep.Persistent.MountPoint = constants.PersistentDir
		}
	}

	return &v1.UpgradeSpec{
		System:         v1.NewEmptySrc(),
		RecoverySystem: recovery,
		Partitions:     ep,
		State:          installState,
	}, nil
}

// NewResetSpec returns a ResetSpec struct all based on defaults and current host state
func NewResetSpec(cfg v1.Config) (*v1.ResetSpec, error) {
	var imgSource *v1.ImageSource

	if !utils.BootedFrom(cfg.Runner, constants.RecoveryImgName) {
		return nil, fmt.Errorf("reset can only be called from the recovery system")
	}

	efiExists, _ := utils.Exists(cfg.Fs, constants.EfiDevice)

	installState, err := cfg.LoadInstallState()
	if err != nil {
		cfg.Logger.Warnf("failed reading installation state: %s", err.Error())
	}

	parts, err := utils.GetAllPartitions()
	if err != nil {
		return nil, fmt.Errorf("could not read host partitions")
	}
	ep := v1.NewElementalPartitionsFromList(parts, installState)

	if efiExists {
		if ep.EFI == nil {
			return nil, fmt.Errorf("EFI partition not found")
		}
		if ep.EFI.MountPoint == "" {
			ep.EFI.MountPoint = constants.EfiDir
		}
		ep.EFI.Name = constants.EfiPartName
	}

	if ep.State == nil {
		return nil, fmt.Errorf("state partition not found")
	}
	if ep.State.MountPoint == "" {
		ep.State.MountPoint = constants.StateDir
	}
	ep.State.Name = constants.StatePartName

	if ep.Recovery == nil {
		return nil, fmt.Errorf("recovery partition not found")
	}
	if ep.Recovery.MountPoint == "" {
		ep.Recovery.MountPoint = constants.RecoveryDir
	}

	target := ep.State.Disk

	// OEM partition is not a hard requirement
	if ep.OEM != nil {
		if ep.OEM.MountPoint == "" {
			ep.OEM.MountPoint = constants.OEMDir
		}
		ep.OEM.Name = constants.OEMPartName
	} else {
		cfg.Logger.Warnf("no OEM partition found")
	}

	// Persistent partition is not a hard requirement
	if ep.Persistent != nil {
		if ep.Persistent.MountPoint == "" {
			ep.Persistent.MountPoint = constants.PersistentDir
		}
		ep.Persistent.Name = constants.PersistentPartName
	} else {
		cfg.Logger.Warnf("no Persistent partition found")
	}

	recoveryImg := filepath.Join(constants.RunningStateDir, constants.RecoveryImgFile)

	if exists, _ := utils.Exists(cfg.Fs, recoveryImg); exists {
		imgSource = v1.NewFileSrc(recoveryImg)
	} else {
		imgSource = v1.NewEmptySrc()
	}

	return &v1.ResetSpec{
		Target:       target,
		Partitions:   ep,
		Efi:          efiExists,
		GrubDefEntry: constants.GrubDefEntry,
		System:       imgSource,
		State:        installState,
	}, nil
}

func NewDiskElementalPartitions(workdir string) v1.ElementalPartitions {
	partitions := v1.ElementalPartitions{}

	// does not return error on v1.EFI use case
	_ = partitions.SetFirmwarePartitions(v1.EFI, v1.GPT)
	partitions.EFI.Path = filepath.Join(workdir, constants.EfiPartName+partSuffix)

	partitions.OEM = &v1.Partition{
		FilesystemLabel: constants.OEMLabel,
		Size:            constants.OEMSize,
		Name:            constants.OEMPartName,
		FS:              constants.LinuxFs,
		MountPoint:      filepath.Join(workdir, constants.OEMPartName+mountSuffix),
		Path:            filepath.Join(workdir, constants.OEMPartName+partSuffix),
		Flags:           []string{},
	}

	partitions.Recovery = &v1.Partition{
		FilesystemLabel: constants.RecoveryLabel,
		Size:            constants.RecoverySize,
		Name:            constants.RecoveryPartName,
		FS:              constants.LinuxFs,
		MountPoint:      filepath.Join(workdir, constants.RecoveryPartName+mountSuffix),
		Path:            filepath.Join(workdir, constants.RecoveryPartName+partSuffix),
		Flags:           []string{},
	}

	partitions.State = &v1.Partition{
		FilesystemLabel: constants.StateLabel,
		Size:            constants.StateSize,
		Name:            constants.StatePartName,
		FS:              constants.LinuxFs,
		MountPoint:      filepath.Join(workdir, constants.StatePartName+mountSuffix),
		Path:            filepath.Join(workdir, constants.StatePartName+partSuffix),
		Flags:           []string{},
	}

	partitions.Persistent = &v1.Partition{
		FilesystemLabel: constants.PersistentLabel,
		Size:            constants.PersistentSize,
		Name:            constants.PersistentPartName,
		FS:              constants.LinuxFs,
		MountPoint:      filepath.Join(workdir, constants.PersistentPartName+mountSuffix),
		Path:            filepath.Join(workdir, constants.PersistentPartName+partSuffix),
		Flags:           []string{},
	}
	return partitions
}

func NewDisk(cfg *v1.BuildConfig) *v1.DiskSpec {
	var workdir string
	var recoveryImg v1.Image

	workdir = filepath.Join(cfg.OutDir, constants.DiskWorkDir)

	recoveryImg.Size = constants.ImgSize
	recoveryImg.File = filepath.Join(workdir, constants.RecoveryPartName, constants.RecoveryImgFile)
	recoveryImg.FS = constants.SquashFs
	recoveryImg.Source = v1.NewEmptySrc()
	recoveryImg.MountPoint = filepath.Join(
		workdir, strings.TrimSuffix(
			constants.RecoveryImgFile, filepath.Ext(constants.RecoveryImgFile),
		)+mountSuffix,
	)

	return &v1.DiskSpec{
		Partitions:     NewDiskElementalPartitions(workdir),
		GrubConf:       filepath.Join(constants.GrubCfgPath, constants.GrubCfg),
		System:         v1.NewEmptySrc(),
		RecoverySystem: recoveryImg,
		Type:           constants.RawType,
		DeployCmd:      []string{"elemental", "--debug", "reset", "--reboot"},
	}
}

func NewISO() *v1.LiveISO {
	return &v1.LiveISO{
		Label:     constants.ISOLabel,
		GrubEntry: constants.GrubDefEntry,
		UEFI:      []*v1.ImageSource{},
		Image:     []*v1.ImageSource{},
		Firmware:  v1.EFI,
	}
}

func NewBuildConfig(opts ...GenericOptions) *v1.BuildConfig {
	b := &v1.BuildConfig{
		Config: *NewConfig(opts...),
		Name:   constants.BuildImgName,
		Snapshotter: v1.SnapshotterConfig{
			Type:     constants.LoopDeviceSnapshotterType,
			MaxSnaps: constants.MaxSnaps,
			Config:   v1.NewLoopDeviceConfig(),
		},
	}
	return b
}
