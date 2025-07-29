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

package config

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/twpayne/go-vfs"
	"k8s.io/mount-utils"

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

func WithMounter(mounter mount.Interface) func(r *v1.Config) error {
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
		c.Mounter = mount.New(constants.MountBinary)
	}

	return c
}

func NewRunConfig(opts ...GenericOptions) *v1.RunConfig {
	config := NewConfig(opts...)
	r := &v1.RunConfig{
		Config: *config,
	}
	return r
}

// NewInstallSpec returns an InstallSpec struct all based on defaults and basic host checks (e.g. EFI vs BIOS)
func NewInstallSpec(cfg v1.Config) *v1.InstallSpec {
	var firmware string
	var recoveryImg, activeImg, passiveImg v1.Image

	// Check if current host has EFI firmware
	efiExists, _ := utils.Exists(cfg.Fs, constants.EfiDevice)
	// Check the default ISO installation media is available
	isoRootExists, _ := utils.Exists(cfg.Fs, constants.ISOBaseTree)

	if efiExists {
		firmware = v1.EFI
	} else {
		firmware = v1.BIOS
	}

	activeImg.Label = constants.ActiveLabel
	activeImg.Size = constants.ImgSize
	activeImg.File = filepath.Join(constants.StateDir, "cOS", constants.ActiveImgFile)
	activeImg.FS = constants.LinuxImgFs
	activeImg.MountPoint = constants.ActiveDir
	if isoRootExists {
		activeImg.Source = v1.NewDirSrc(constants.ISOBaseTree)
	} else {
		activeImg.Source = v1.NewEmptySrc()
	}

	recoveryImg.Source = v1.NewFileSrc(activeImg.File)
	recoveryImg.FS = constants.LinuxImgFs
	recoveryImg.Label = constants.SystemLabel
	recoveryImg.File = filepath.Join(constants.RecoveryDir, "cOS", constants.RecoveryImgFile)

	passiveImg = v1.Image{
		File:   filepath.Join(constants.StateDir, "cOS", constants.PassiveImgFile),
		Label:  constants.PassiveLabel,
		Source: v1.NewFileSrc(activeImg.File),
		FS:     constants.LinuxImgFs,
	}

	return &v1.InstallSpec{
		Firmware:   firmware,
		PartTable:  v1.GPT,
		Partitions: NewInstallElementalPartitions(),
		GrubConf:   constants.GrubConf,
		Active:     activeImg,
		Recovery:   recoveryImg,
		Passive:    passiveImg,
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
	return partitions
}

// getActivePassiveAndRecovery returns active, passive and recovery states from a given install state. It
// returns default values for any missing field.
func getActivePassiveAndRecoveryState(state *v1.InstallState) (active, passive, recovery *v1.ImageState) {
	recovery = &v1.ImageState{
		FS:    constants.LinuxImgFs,
		Label: constants.SystemLabel,
	}
	passive = &v1.ImageState{
		FS:    constants.LinuxImgFs,
		Label: constants.PassiveLabel,
	}
	active = &v1.ImageState{
		FS:    constants.LinuxImgFs,
		Label: constants.ActiveLabel,
	}

	if state != nil {
		sPart := state.Partitions[constants.StatePartName]
		if sPart != nil {
			if sPart.Images[constants.ActiveImgName] != nil {
				active = sPart.Images[constants.ActiveImgName]
			}
			if sPart.Images[constants.PassiveImgName] != nil {
				passive = sPart.Images[constants.PassiveImgName]
			}
		}
		rPart := state.Partitions[constants.RecoveryPartName]
		if rPart != nil {
			if rPart.Images[constants.RecoveryImgName] != nil {
				recovery = rPart.Images[constants.RecoveryImgName]
			}
		}
	}

	return active, passive, recovery
}

// NewUpgradeSpec returns an UpgradeSpec struct all based on defaults and current host state
func NewUpgradeSpec(cfg v1.Config) (*v1.UpgradeSpec, error) {
	var aState, pState, rState *v1.ImageState
	var active, passive, recovery v1.Image

	installState, err := cfg.LoadInstallState()
	if err != nil {
		cfg.Logger.Warnf("failed reading installation state: %s", err.Error())
	}

	aState, pState, rState = getActivePassiveAndRecoveryState(installState)

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
			File:       filepath.Join(ep.Recovery.MountPoint, "cOS", constants.TransitionImgFile),
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

		active = v1.Image{
			File:       filepath.Join(ep.State.MountPoint, "cOS", constants.TransitionImgFile),
			Size:       constants.ImgSize,
			Label:      aState.Label,
			FS:         aState.FS,
			MountPoint: constants.TransitionDir,
			Source:     v1.NewEmptySrc(),
		}

		passive = v1.Image{
			File:   filepath.Join(ep.State.MountPoint, "cOS", constants.PassiveImgFile),
			Label:  pState.Label,
			Source: v1.NewFileSrc(active.File),
			FS:     active.FS,
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
		Active:     active,
		Recovery:   recovery,
		Passive:    passive,
		Partitions: ep,
		State:      installState,
	}, nil
}

// NewResetSpec returns a ResetSpec struct all based on defaults and current host state
func NewResetSpec(cfg v1.Config) (*v1.ResetSpec, error) {
	var imgSource *v1.ImageSource
	var aState, pState *v1.ImageState

	if !utils.BootedFrom(cfg.Runner, constants.RecoveryImgFile) {
		return nil, fmt.Errorf("reset can only be called from the recovery system")
	}

	efiExists, _ := utils.Exists(cfg.Fs, constants.EfiDevice)

	installState, err := cfg.LoadInstallState()
	if err != nil {
		cfg.Logger.Warnf("failed reading installation state: %s", err.Error())
	}
	aState, pState, _ = getActivePassiveAndRecoveryState(installState)

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

	recoveryImg := filepath.Join(constants.RunningStateDir, "cOS", constants.RecoveryImgFile)

	if exists, _ := utils.Exists(cfg.Fs, recoveryImg); exists {
		imgSource = v1.NewFileSrc(recoveryImg)
	} else {
		imgSource = v1.NewEmptySrc()
	}

	activeFile := filepath.Join(ep.State.MountPoint, "cOS", constants.ActiveImgFile)
	return &v1.ResetSpec{
		Target:       target,
		Partitions:   ep,
		Efi:          efiExists,
		GrubDefEntry: constants.GrubDefEntry,
		GrubConf:     constants.GrubConf,
		Active: v1.Image{
			Label:      aState.Label,
			Size:       constants.ImgSize,
			File:       activeFile,
			FS:         aState.FS,
			Source:     imgSource,
			MountPoint: constants.ActiveDir,
		},
		Passive: v1.Image{
			File:   filepath.Join(ep.State.MountPoint, "cOS", constants.PassiveImgFile),
			Label:  pState.Label,
			Source: v1.NewFileSrc(activeFile),
			FS:     aState.FS,
		},
		State: installState,
	}, nil
}

func NewDiskElementalParitions(workdir string) v1.ElementalPartitions {
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
	var recoveryImg, activeImg, passiveImg v1.Image

	workdir = filepath.Join(cfg.OutDir, constants.DiskWorkDir)

	recoveryImg.Size = constants.ImgSize
	recoveryImg.File = filepath.Join(workdir, constants.RecoveryPartName, "cOS", constants.RecoveryImgFile)
	recoveryImg.FS = constants.SquashFs
	recoveryImg.Source = v1.NewEmptySrc()
	recoveryImg.MountPoint = filepath.Join(
		workdir, strings.TrimSuffix(
			constants.RecoveryImgFile, filepath.Ext(constants.RecoveryImgFile),
		)+mountSuffix,
	)

	activeImg.Size = constants.ImgSize
	activeImg.File = filepath.Join(workdir, constants.StatePartName, "cOS", constants.ActiveImgFile)
	activeImg.FS = constants.SquashFs
	activeImg.Source = v1.NewEmptySrc()
	activeImg.MountPoint = filepath.Join(
		workdir, strings.TrimSuffix(
			constants.ActiveImgFile, filepath.Ext(constants.ActiveImgFile),
		)+mountSuffix,
	)

	passiveImg.Size = constants.ImgSize
	passiveImg.File = filepath.Join(workdir, constants.StatePartName, "cOS", constants.PassiveImgFile)
	passiveImg.FS = constants.SquashFs
	passiveImg.Source = v1.NewEmptySrc()
	activeImg.MountPoint = filepath.Join(
		workdir, strings.TrimSuffix(
			constants.PassiveImgFile, filepath.Ext(constants.PassiveImgFile),
		)+mountSuffix,
	)

	return &v1.DiskSpec{
		Partitions: NewDiskElementalParitions(workdir),
		GrubConf:   constants.GrubConf,
		Active:     activeImg,
		Recovery:   recoveryImg,
		Passive:    passiveImg,
		Type:       constants.RawType,
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
	}
	return b
}

// ReconcileUpgradeSpec will check current mounts which may differ from elemental disovery from /sys/block tree
// as this skips multipathed devices which may be in use.
func ReconcileUpgradeSpec(r *v1.RunConfig, spec *v1.UpgradeSpec) error {
	if spec.Partitions.State != nil {
		if err := reconcilePartition(r, spec.Partitions.State); err != nil {
			return err
		}
	}
	if spec.Partitions.Recovery != nil {
		if err := reconcilePartition(r, spec.Partitions.Recovery); err != nil {
			return err
		}
	}

	if spec.Partitions.Persistent != nil {
		if err := reconcilePartition(r, spec.Partitions.Persistent); err != nil {
			return err
		}
	}

	if spec.Partitions.OEM != nil {
		if err := reconcilePartition(r, spec.Partitions.OEM); err != nil {
			return err
		}
	}
	return nil
}

func reconcilePartition(r *v1.RunConfig, part *v1.Partition) error {
	discoveredMountDiskBytes, err := execBlkid(r, part.FilesystemLabel)
	if err != nil {
		return fmt.Errorf("error discovering current partition using label %s: %w", part.FilesystemLabel, err)
	}

	// trim space since `blkid` output has a newline in result
	discoveredMount := strings.TrimSpace(string(discoveredMountDiskBytes))
	if part.Path != discoveredMount {
		part.Path = discoveredMount
	}
	return nil
}
func execBlkid(r *v1.RunConfig, name string) ([]byte, error) {
	if ok := r.Config.Runner.CommandExists("blkid"); ok {
		return r.Config.Runner.Run("blkid", "-L", name)
	}

	return []byte{}, fmt.Errorf("blkid not found")
}
