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

package config

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/rancher/elemental-cli/pkg/cloudinit"
	"github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/http"
	"github.com/rancher/elemental-cli/pkg/luet"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
	"github.com/twpayne/go-vfs"
	"k8s.io/mount-utils"
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

func WithLuet(luet v1.LuetInterface) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Luet = luet
		return nil
	}
}

func WithArch(arch string) func(r *v1.Config) error {
	return func(r *v1.Config) error {
		r.Arch = arch
		return nil
	}
}

func NewConfig(opts ...GenericOptions) *v1.Config {
	log := v1.NewLogger()
	arch := func() string {
		switch runtime.GOARCH {
		case "amd64":
			return "x86_64"
		}
		return runtime.GOARCH
	}()
	c := &v1.Config{
		Fs:                        vfs.OSFS,
		Logger:                    log,
		Syscall:                   &v1.RealSyscall{},
		Client:                    http.NewClient(),
		Repos:                     []v1.Repository{},
		Arch:                      arch,
		SquashFsCompressionConfig: constants.GetDefaultSquashfsCompressionOptions(),
	}
	for _, o := range opts {
		err := o(c)
		if err != nil {
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

	if c.Luet == nil {
		tmpDir := utils.GetTempDir(c, "")
		c.Luet = luet.NewLuet(luet.WithFs(c.Fs), luet.WithLogger(log), luet.WithLuetTempDir(tmpDir))
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

// CoOccurrenceConfig sets further configurations once config files and other
// runtime configurations are read. This is mostly a method to call once the
// mapstructure unmarshal already took place.
func CoOccurrenceConfig(cfg *v1.Config) {
	// Set Luet plugins, we only use the mtree plugin for now
	if cfg.Verify {
		cfg.Luet.SetPlugins(constants.LuetMtreePlugin)
	}
}

// NewInstallSpec returns an InstallSpec struct all based on defaults and basic host checks (e.g. EFI vs BIOS)
func NewInstallSpec(cfg v1.Config) *v1.InstallSpec {
	var firmware string
	var recoveryImg, activeImg, passiveImg v1.Image

	recoveryImgFile := filepath.Join(constants.LiveDir, constants.RecoverySquashFile)

	// Check if current host has EFI firmware
	efiExists, _ := utils.Exists(cfg.Fs, constants.EfiDevice)
	// Check the default ISO installation media is available
	isoRootExists, _ := utils.Exists(cfg.Fs, constants.IsoBaseTree)
	// Check the default ISO recovery installation media is available)
	recoveryExists, _ := utils.Exists(cfg.Fs, recoveryImgFile)

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
		activeImg.Source = v1.NewDirSrc(constants.IsoBaseTree)
	} else {
		activeImg.Source = v1.NewEmptySrc()
	}

	if recoveryExists {
		recoveryImg.Source = v1.NewFileSrc(recoveryImgFile)
		recoveryImg.FS = constants.SquashFs
		recoveryImg.File = filepath.Join(constants.RecoveryDir, "cOS", constants.RecoverySquashFile)
	} else {
		recoveryImg.Source = v1.NewFileSrc(activeImg.File)
		recoveryImg.FS = constants.LinuxImgFs
		recoveryImg.Label = constants.SystemLabel
		recoveryImg.File = filepath.Join(constants.RecoveryDir, "cOS", constants.RecoveryImgFile)
	}

	passiveImg = v1.Image{
		File:   filepath.Join(constants.StateDir, "cOS", constants.PassiveImgFile),
		Label:  constants.PassiveLabel,
		Source: v1.NewFileSrc(activeImg.File),
		FS:     constants.LinuxImgFs,
	}

	return &v1.InstallSpec{
		Firmware:     firmware,
		PartTable:    v1.GPT,
		Partitions:   NewInstallElementalParitions(),
		GrubDefEntry: constants.GrubDefEntry,
		GrubConf:     constants.GrubConf,
		Tty:          constants.DefaultTty,
		Active:       activeImg,
		Recovery:     recoveryImg,
		Passive:      passiveImg,
	}
}

func NewInstallElementalParitions() v1.ElementalPartitions {
	partitions := v1.ElementalPartitions{}
	partitions.OEM = &v1.Partition{
		Label:      constants.OEMLabel,
		Size:       constants.OEMSize,
		Name:       constants.OEMPartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.OEMDir,
		Flags:      []string{},
	}

	partitions.Recovery = &v1.Partition{
		Label:      constants.RecoveryLabel,
		Size:       constants.RecoverySize,
		Name:       constants.RecoveryPartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.RecoveryDir,
		Flags:      []string{},
	}

	partitions.State = &v1.Partition{
		Label:      constants.StateLabel,
		Size:       constants.StateSize,
		Name:       constants.StatePartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.StateDir,
		Flags:      []string{},
	}

	partitions.Persistent = &v1.Partition{
		Label:      constants.PersistentLabel,
		Size:       constants.PersistentSize,
		Name:       constants.PersistentPartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.PersistentDir,
		Flags:      []string{},
	}
	return partitions
}

// NewUpgradeSpec returns an UpgradeSpec struct all based on defaults and current host state
func NewUpgradeSpec(cfg v1.Config) (*v1.UpgradeSpec, error) {
	var recLabel, recFs, recMnt string
	var active, passive, recovery v1.Image

	parts, err := utils.GetAllPartitions()
	if err != nil {
		return nil, fmt.Errorf("could not read host partitions")
	}
	ep := v1.NewElementalPartitionsFromList(parts)

	if ep.Recovery != nil {
		if ep.Recovery.MountPoint == "" {
			ep.Recovery.MountPoint = constants.RecoveryDir
		}

		squashedRec, err := utils.HasSquashedRecovery(&cfg, ep.Recovery)
		if err != nil {
			return nil, fmt.Errorf("failed checking for squashed recovery")
		}

		if squashedRec {
			recFs = constants.SquashFs
		} else {
			recLabel = constants.SystemLabel
			recFs = constants.LinuxImgFs
			recMnt = constants.TransitionDir
		}

		recovery = v1.Image{
			File:       filepath.Join(ep.Recovery.MountPoint, "cOS", constants.TransitionImgFile),
			Size:       constants.ImgSize,
			Label:      recLabel,
			FS:         recFs,
			MountPoint: recMnt,
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
			Label:      constants.ActiveLabel,
			FS:         constants.LinuxImgFs,
			MountPoint: constants.TransitionDir,
			Source:     v1.NewEmptySrc(),
		}

		passive = v1.Image{
			File:   filepath.Join(ep.State.MountPoint, "cOS", constants.PassiveImgFile),
			Label:  constants.PassiveLabel,
			Source: v1.NewFileSrc(active.File),
			FS:     constants.LinuxImgFs,
		}
	}

	return &v1.UpgradeSpec{
		Active:     active,
		Recovery:   recovery,
		Passive:    passive,
		Partitions: ep,
	}, nil
}

// NewResetSpec returns a ResetSpec struct all based on defaults and current host state
func NewResetSpec(cfg v1.Config) (*v1.ResetSpec, error) {
	var imgSource *v1.ImageSource

	//TODO find a way to pre-load current state values such as labels
	if !utils.BootedFrom(cfg.Runner, constants.RecoverySquashFile) &&
		!utils.BootedFrom(cfg.Runner, constants.SystemLabel) {
		return nil, fmt.Errorf("reset can only be called from the recovery system")
	}

	efiExists, _ := utils.Exists(cfg.Fs, constants.EfiDevice)

	parts, err := utils.GetAllPartitions()
	if err != nil {
		return nil, fmt.Errorf("could not read host partitions")
	}
	ep := v1.NewElementalPartitionsFromList(parts)

	// We won't do anything with the recovery partition
	// removing it so we can easily loop to mount and unmount
	ep.Recovery = nil

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
	} else if exists, _ = utils.Exists(cfg.Fs, constants.IsoBaseTree); exists {
		imgSource = v1.NewDirSrc(constants.IsoBaseTree)
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
		Tty:          constants.DefaultTty,
		Active: v1.Image{
			Label:      constants.ActiveLabel,
			Size:       constants.ImgSize,
			File:       activeFile,
			FS:         constants.LinuxImgFs,
			Source:     imgSource,
			MountPoint: constants.ActiveDir,
		},
		Passive: v1.Image{
			File:   filepath.Join(ep.State.MountPoint, "cOS", constants.PassiveImgFile),
			Label:  constants.PassiveLabel,
			Source: v1.NewFileSrc(activeFile),
			FS:     constants.LinuxImgFs,
		},
	}, nil
}

func NewRawDisk() *v1.RawDisk {
	var packages []v1.RawDiskPackage
	defaultPackages := constants.GetBuildDiskDefaultPackages()

	for pkg, target := range defaultPackages {
		packages = append(packages, v1.RawDiskPackage{Name: pkg, Target: target})
	}

	return &v1.RawDisk{
		X86_64: &v1.RawDiskArchEntry{Packages: packages},
		Arm64:  &v1.RawDiskArchEntry{Packages: packages},
	}
}

func NewISO() *v1.LiveISO {
	var uefiSrc, imageSrc []*v1.ImageSource

	defaultUEFI := constants.GetDefaultISOUEFI()
	for _, imgSrc := range defaultUEFI {
		src, _ := v1.NewSrcFromURI(imgSrc)
		uefiSrc = append(uefiSrc, src)
	}

	defaultImage := constants.GetDefaultISOImage()
	for _, imgSrc := range defaultImage {
		src, _ := v1.NewSrcFromURI(imgSrc)
		imageSrc = append(imageSrc, src)
	}

	return &v1.LiveISO{
		Label:       constants.ISOLabel,
		UEFI:        uefiSrc,
		Image:       imageSrc,
		HybridMBR:   constants.IsoHybridMBR,
		BootFile:    constants.IsoBootFile,
		BootCatalog: constants.IsoBootCatalog,
	}
}

func NewBuildConfig(opts ...GenericOptions) *v1.BuildConfig {
	b := &v1.BuildConfig{
		Config: *NewConfig(opts...),
		Name:   constants.BuildImgName,
	}
	if len(b.Repos) == 0 {
		repo := constants.LuetDefaultRepoURI
		if b.Arch != "x86_64" {
			repo = fmt.Sprintf("%s-%s", constants.LuetDefaultRepoURI, b.Arch)
		}
		b.Repos = []v1.Repository{{
			Name:     "cos",
			Type:     "docker",
			URI:      repo,
			Priority: constants.LuetDefaultRepoPrio,
		}}
	}
	return b
}
