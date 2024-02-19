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

	"github.com/rancher/elemental-toolkit/v2/pkg/cloudinit"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/features"
	"github.com/rancher/elemental-toolkit/v2/pkg/http"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

const (
	partSuffix  = ".part"
	mountSuffix = ".mount"
)

type GenericOptions func(a *v2.Config) error

func WithFs(fs v2.FS) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.Fs = fs
		return nil
	}
}

func WithLogger(logger v2.Logger) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.Logger = logger
		return nil
	}
}

func WithSyscall(syscall v2.SyscallInterface) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.Syscall = syscall
		return nil
	}
}

func WithMounter(mounter v2.Mounter) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.Mounter = mounter
		return nil
	}
}

func WithRunner(runner v2.Runner) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.Runner = runner
		return nil
	}
}

func WithClient(client v2.HTTPClient) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.Client = client
		return nil
	}
}

func WithCloudInitRunner(ci v2.CloudInitRunner) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.CloudInitRunner = ci
		return nil
	}
}

func WithPlatform(platform string) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		p, err := v2.ParsePlatform(platform)
		r.Platform = p
		return err
	}
}

func WithOCIImageExtractor() func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.ImageExtractor = v2.OCIImageExtractor{}
		return nil
	}
}

func WithImageExtractor(extractor v2.ImageExtractor) func(r *v2.Config) error {
	return func(r *v2.Config) error {
		r.ImageExtractor = extractor
		return nil
	}
}

func NewConfig(opts ...GenericOptions) *v2.Config {
	log := v2.NewLogger()

	defaultPlatform, err := v2.NewPlatformFromArch(runtime.GOARCH)
	if err != nil {
		log.Errorf("error parsing default platform (%s): %s", runtime.GOARCH, err.Error())
		return nil
	}

	c := &v2.Config{
		Fs:                        vfs.OSFS,
		Logger:                    log,
		Syscall:                   &v2.RealSyscall{},
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
		c.Runner = &v2.RealRunner{Logger: c.Logger}
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
		c.Mounter = v2.NewMounter(constants.MountBinary)
	}

	return c
}

func NewRunConfig(opts ...GenericOptions) *v2.RunConfig {
	config := NewConfig(opts...)

	snapshotter := v1.NewLoopDevice()

	// Load snapshotter setup from state.yaml for reset and upgrade
	installState, _ := config.LoadInstallState()
	if installState != nil {
		snapshotter = installState.Snapshotter
	}

	r := &v2.RunConfig{
		Snapshotter: snapshotter,
		Config:      *config,
	}
	return r
}

// NewInstallSpec returns an InstallSpec struct all based on defaults and basic host checks (e.g. EFI vs BIOS)
func NewInstallSpec(cfg v2.Config) *v2.InstallSpec {
	var system *v2.ImageSource
	var recoverySystem v2.Image

	// Check the default ISO installation media is available
	isoRootExists, _ := utils.Exists(cfg.Fs, constants.ISOBaseTree)

	if isoRootExists {
		system = v2.NewDirSrc(constants.ISOBaseTree)
	} else {
		system = v2.NewEmptySrc()
	}

	recoverySystem.Source = system
	recoverySystem.FS = constants.LinuxImgFs
	recoverySystem.Label = constants.SystemLabel
	recoverySystem.File = filepath.Join(constants.RecoveryDir, constants.RecoveryImgFile)
	recoverySystem.MountPoint = constants.TransitionDir

	return &v2.InstallSpec{
		Firmware:       v2.EFI,
		PartTable:      v2.GPT,
		Partitions:     NewInstallElementalPartitions(),
		System:         system,
		RecoverySystem: recoverySystem,
	}
}

// NewInitSpec returns an InitSpec struct all based on defaults
func NewInitSpec() *v2.InitSpec {
	return &v2.InitSpec{
		Mkinitrd: true,
		Force:    false,
		Features: features.All,
	}
}

func NewMountSpec() *v2.MountSpec {
	return &v2.MountSpec{
		Sysroot:    "/sysroot",
		WriteFstab: true,
		Volumes: []*v2.VolumeMount{
			{
				Mountpoint: constants.OEMPath,
				Device:     fmt.Sprintf("PARTLABEL=%s", constants.OEMPartName),
				Options:    []string{"rw", "defaults"},
			}, {
				Mountpoint: constants.EfiDir,
				Device:     fmt.Sprintf("PARTLABEL=%s", constants.EfiPartName),
				Options:    []string{"ro", "defaults"},
			},
		},
		Ephemeral: v2.EphemeralMounts{
			Type:  constants.Tmpfs,
			Size:  "25%",
			Paths: []string{"/var", "/etc", "/srv"},
		},
		Persistent: v2.PersistentMounts{
			Mode:  constants.OverlayMode,
			Paths: []string{"/etc/systemd", "/etc/ssh", "/home", "/opt", "/root", "/var/log"},
			Volume: v2.VolumeMount{
				Mountpoint: constants.PersistentDir,
				Device:     fmt.Sprintf("PARTLABEL=%s", constants.PersistentPartName),
				Options:    []string{"rw", "defaults"},
			},
		},
	}
}

func NewInstallElementalPartitions() v2.ElementalPartitions {
	partitions := v2.ElementalPartitions{}
	partitions.OEM = &v2.Partition{
		FilesystemLabel: constants.OEMLabel,
		Size:            constants.OEMSize,
		Name:            constants.OEMPartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.OEMDir,
		Flags:           []string{},
	}

	partitions.Recovery = &v2.Partition{
		FilesystemLabel: constants.RecoveryLabel,
		Size:            constants.RecoverySize,
		Name:            constants.RecoveryPartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.RecoveryDir,
		Flags:           []string{},
	}

	partitions.State = &v2.Partition{
		FilesystemLabel: constants.StateLabel,
		Size:            constants.StateSize,
		Name:            constants.StatePartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.StateDir,
		Flags:           []string{},
	}

	partitions.Persistent = &v2.Partition{
		FilesystemLabel: constants.PersistentLabel,
		Size:            constants.PersistentSize,
		Name:            constants.PersistentPartName,
		FS:              constants.LinuxFs,
		MountPoint:      constants.PersistentDir,
		Flags:           []string{},
	}

	_ = partitions.SetFirmwarePartitions(v2.EFI, v2.GPT)

	return partitions
}

// getRecoveryState returns recovery state from a given install state. It
// returns default values for any missing field.
func getRecoveryState(state *v2.InstallState) (recovery *v2.SystemState) {
	recovery = &v2.SystemState{
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
func NewUpgradeSpec(cfg v2.Config) (*v2.UpgradeSpec, error) {
	var rState *v2.SystemState
	var recovery v2.Image

	installState, err := cfg.LoadInstallState()
	if err != nil {
		cfg.Logger.Warnf("failed reading installation state: %s", err.Error())
	}

	rState = getRecoveryState(installState)

	parts, err := utils.GetAllPartitions()
	if err != nil {
		return nil, fmt.Errorf("could not read host partitions")
	}
	ep := v2.NewElementalPartitionsFromList(parts, installState)

	if ep.Recovery != nil {
		if ep.Recovery.MountPoint == "" {
			ep.Recovery.MountPoint = constants.RecoveryDir
		}

		recovery = v2.Image{
			File:       filepath.Join(ep.Recovery.MountPoint, constants.TransitionImgFile),
			Size:       constants.ImgSize,
			Label:      rState.Label,
			FS:         rState.FS,
			MountPoint: constants.TransitionDir,
			Source:     v2.NewEmptySrc(),
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

	return &v2.UpgradeSpec{
		System:         v2.NewEmptySrc(),
		RecoverySystem: recovery,
		Partitions:     ep,
		State:          installState,
	}, nil
}

// NewResetSpec returns a ResetSpec struct all based on defaults and current host state
func NewResetSpec(cfg v2.Config) (*v2.ResetSpec, error) {
	var imgSource *v2.ImageSource

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
	ep := v2.NewElementalPartitionsFromList(parts, installState)

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
		imgSource = v2.NewFileSrc(recoveryImg)
	} else {
		imgSource = v2.NewEmptySrc()
	}

	return &v2.ResetSpec{
		Target:       target,
		Partitions:   ep,
		Efi:          efiExists,
		GrubDefEntry: constants.GrubDefEntry,
		System:       imgSource,
		State:        installState,
	}, nil
}

func NewDiskElementalPartitions(workdir string) v2.ElementalPartitions {
	partitions := v2.ElementalPartitions{}

	// does not return error on v2.EFI use case
	_ = partitions.SetFirmwarePartitions(v2.EFI, v2.GPT)
	partitions.EFI.Path = filepath.Join(workdir, constants.EfiPartName+partSuffix)

	partitions.OEM = &v2.Partition{
		FilesystemLabel: constants.OEMLabel,
		Size:            constants.OEMSize,
		Name:            constants.OEMPartName,
		FS:              constants.LinuxFs,
		MountPoint:      filepath.Join(workdir, constants.OEMPartName+mountSuffix),
		Path:            filepath.Join(workdir, constants.OEMPartName+partSuffix),
		Flags:           []string{},
	}

	partitions.Recovery = &v2.Partition{
		FilesystemLabel: constants.RecoveryLabel,
		Size:            constants.RecoverySize,
		Name:            constants.RecoveryPartName,
		FS:              constants.LinuxFs,
		MountPoint:      filepath.Join(workdir, constants.RecoveryPartName+mountSuffix),
		Path:            filepath.Join(workdir, constants.RecoveryPartName+partSuffix),
		Flags:           []string{},
	}

	partitions.State = &v2.Partition{
		FilesystemLabel: constants.StateLabel,
		Size:            constants.StateSize,
		Name:            constants.StatePartName,
		FS:              constants.LinuxFs,
		MountPoint:      filepath.Join(workdir, constants.StatePartName+mountSuffix),
		Path:            filepath.Join(workdir, constants.StatePartName+partSuffix),
		Flags:           []string{},
	}

	partitions.Persistent = &v2.Partition{
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

func NewDisk(cfg *v2.BuildConfig) *v2.DiskSpec {
	var workdir string
	var recoveryImg v2.Image

	workdir = filepath.Join(cfg.OutDir, constants.DiskWorkDir)

	recoveryImg.Size = constants.ImgSize
	recoveryImg.File = filepath.Join(workdir, constants.RecoveryPartName, constants.RecoveryImgFile)
	recoveryImg.FS = constants.LinuxImgFs
	recoveryImg.Label = constants.SystemLabel
	recoveryImg.Source = v2.NewEmptySrc()
	recoveryImg.MountPoint = filepath.Join(
		workdir, strings.TrimSuffix(
			constants.RecoveryImgFile, filepath.Ext(constants.RecoveryImgFile),
		)+mountSuffix,
	)

	return &v2.DiskSpec{
		Partitions:     NewDiskElementalPartitions(workdir),
		GrubConf:       filepath.Join(constants.GrubCfgPath, constants.GrubCfg),
		System:         v2.NewEmptySrc(),
		RecoverySystem: recoveryImg,
		Type:           constants.RawType,
		DeployCmd:      []string{"elemental", "--debug", "reset", "--reboot"},
	}
}

func NewISO() *v2.LiveISO {
	return &v2.LiveISO{
		Label:     constants.ISOLabel,
		GrubEntry: constants.GrubDefEntry,
		UEFI:      []*v2.ImageSource{},
		Image:     []*v2.ImageSource{},
		Firmware:  v2.EFI,
	}
}

func NewBuildConfig(opts ...GenericOptions) *v2.BuildConfig {
	b := &v1.BuildConfig{
		Config:      *NewConfig(opts...),
		Name:        constants.BuildImgName,
		Snapshotter: v2.NewLoopDevice(),
	}
	return b
}
