/*
Copyright © 2021 SUSE LLC

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
	dockTypes "github.com/docker/docker/api/types"
	"github.com/mudler/luet/pkg/api/core/context"
	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"net/http"
)

const (
	GPT   = "gpt"
	ESP   = "esp"
	BIOS  = "bios_grub"
	MSDOS = "msdos"
	BOOT  = "boot"
)

type RunConfigOptions func(a *RunConfig) error

func WithFs(fs afero.Fs) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.Fs = fs
		return nil
	}
}

func WithLogger(logger Logger) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.Logger = logger
		return nil
	}
}

func WithSyscall(syscall SyscallInterface) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.Syscall = syscall
		return nil
	}
}

func WithMounter(mounter mount.Interface) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.Mounter = mounter
		return nil
	}
}

func WithRunner(runner Runner) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.Runner = runner
		return nil
	}
}

func WithClient(client HTTPClient) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.Client = client
		return nil
	}
}

func WithCloudInitRunner(ci CloudInitRunner) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.CloudInitRunner = ci
		return nil
	}
}

func WithLuet(luet LuetInterface) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.Luet = luet
		return nil
	}
}

func NewRunConfig(opts ...RunConfigOptions) *RunConfig {
	log := NewLogger()
	r := &RunConfig{
		Fs:      afero.NewOsFs(),
		Logger:  log,
		Runner:  &RealRunner{},
		Syscall: &RealSyscall{},
		Client:  &http.Client{},
	}
	for _, o := range opts {
		err := o(r)
		if err != nil {
			return nil
		}
	}

	// Delay the yip runner creation, so we set the proper logger instead of blindly setting it to the logger we create
	// at the start of NewRunConfig, as WithLogger can be passed on init, and that would result in 2 different logger
	// instances, on on the config.Logger and the other on config.CloudInitRunner
	if r.CloudInitRunner == nil {
		r.CloudInitRunner = NewYipCloudInitRunner(r.Logger)
	}

	if r.Mounter == nil {
		r.Mounter = mount.New(cnst.MountBinary)
	}

	if r.CloudInitRunner == nil {
		r.CloudInitRunner = NewYipCloudInitRunner(r.Logger)
	}

	// Set defaults if empty
	if r.GrubConf == "" {
		r.GrubConf = cnst.GrubConf
	}

	r.ActiveImage = Image{
		Label:      cnst.ActiveLabel,
		Size:       cnst.ImgSize,
		File:       fmt.Sprintf("%s/cOS/%s", cnst.StateDir, cnst.ActiveImgFile),
		FS:         cnst.LinuxImgFs,
		RootTree:   cnst.IsoBaseTree,
		MountPoint: cnst.ActiveDir,
	}

	if r.ActiveLabel != "" {
		r.ActiveImage.Label = r.ActiveLabel
	}

	if r.PassiveLabel == "" {
		r.PassiveLabel = cnst.PassiveLabel
	}

	if r.SystemLabel == "" {
		r.SystemLabel = cnst.SystemLabel
	}

	r.Partitions = PartitionList{}

	if r.IsoMnt == "" {
		r.IsoMnt = cnst.IsoMnt
	}

	if r.GrubDefEntry == "" {
		r.GrubDefEntry = cnst.GrubDefEntry
	}
	return r
}

// RunConfig is the struct that represents the full configuration needed for install, upgrade, reset, rebrand.
// Basically everything needed to know for all operations in a running system, not related to builds
type RunConfig struct {
	// Can come from config, env var or flags
	RecoveryLabel   string `yaml:"RECOVERY_LABEL,omitempty" mapstructure:"RECOVERY_LABEL"`
	PersistentLabel string `yaml:"PERSISTENT_LABEL,omitempty" mapstructure:"PERSISTENT_LABEL"`
	StateLabel      string `yaml:"STATE_LABEL,omitempty" mapstructure:"STATE_LABEL"`
	OEMLabel        string `yaml:"OEM_LABEL,omitempty" mapstructure:"OEM_LABEL"`
	SystemLabel     string `yaml:"SYSTEM_LABEL,omitempty" mapstructure:"SYSTEM_LABEL"`
	ActiveLabel     string `yaml:"ACTIVE_LABEL,omitempty" mapstructure:"ACTIVE_LABEL"`
	PassiveLabel    string `yaml:"PASSIVE_LABEL,omitempty" mapstructure:"PASSIVE_LABEL"`
	Target          string `yaml:"target,omitempty" mapstructure:"target"`
	Source          string `yaml:"source,omitempty" mapstructure:"source"`
	CloudInit       string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	ForceEfi        bool   `yaml:"force-efi,omitempty" mapstructure:"force-efi"`
	ForceGpt        bool   `yaml:"force-gpt,omitempty" mapstructure:"force-gpt"`
	PartLayout      string `yaml:"partition-layout,omitempty" mapstructure:"partition-layout"`
	Tty             string `yaml:"tty,omitempty" mapstructure:"tty"`
	NoFormat        bool   `yaml:"no-format,omitempty" mapstructure:"no-format"`
	Force           bool   `yaml:"force,omitempty" mapstructure:"force"`
	Strict          bool   `yaml:"strict,omitempty" mapstructure:"strict"`
	Iso             string `yaml:"iso,omitempty" mapstructure:"iso"`
	DockerImg       string `yaml:"docker-image,omitempty" mapstructure:"docker-image"`
	Cosign          bool   `yaml:"cosign,omitempty" mapstructure:"cosign"`
	CosignPubKey    string `yaml:"cosign-key,omitempty" mapstructure:"cosign-key"`
	NoVerify        bool   `yaml:"no-verify,omitempty" mapstructure:"no-verify"`
	CloudInitPaths  string `yaml:"CLOUD_INIT_PATHS,omitempty" mapstructure:"CLOUD_INIT_PATHS"`
	GrubDefEntry    string `yaml:"GRUB_ENTRY_NAME,omitempty" mapstructure:"GRUB_ENTRY_NAME"`
	// Internally used to track stuff around
	PartTable string
	BootFlag  string
	GrubConf  string
	IsoMnt    string // /run/initramfs/live by default, can be set to a different dir if --iso flag is set
	// Interfaces used around by methods
	Logger          Logger
	Fs              afero.Fs
	Mounter         mount.Interface
	Runner          Runner
	Syscall         SyscallInterface
	CloudInitRunner CloudInitRunner
	Luet            LuetInterface
	Partitions      PartitionList
	Client          HTTPClient
	ActiveImage     Image
}

// Partition struct represents a partition with its commonly configurable values, size in MiB
type Partition struct {
	Label      string
	Size       uint
	PLabel     string
	FS         string
	Flags      []string
	MountPoint string
}

type PartitionList []*Partition

// Image struct represents a file system image with its commonly configurable values, size in MiB
type Image struct {
	File  string
	Label string
	Size  uint
	FS    string
	// Path of the root tree
	RootTree   string
	MountPoint string
	LoopDevice string
}

func (pl PartitionList) GetByPLabel(label string) *Partition {
	for _, p := range pl {
		if p.PLabel == label {
			return p
		}
	}
	return nil
}

// setupStyle will gather what partition table and bootflag we need for the current system
func (r *RunConfig) setupStyle() {
	_, err := r.Fs.Stat(cnst.EfiDevice)
	efiExists := err == nil
	statePartFlags := []string{}
	var part *Partition

	if r.ForceEfi || efiExists {
		r.PartTable = GPT
		r.BootFlag = ESP
		part = &Partition{
			Label:      cnst.EfiLabel,
			Size:       cnst.EfiSize,
			PLabel:     cnst.EfiPLabel,
			FS:         cnst.EfiFs,
			MountPoint: cnst.EfiDir,
			Flags:      []string{ESP},
		}
		r.Partitions = append(r.Partitions, part)
	} else if r.ForceGpt {
		r.PartTable = GPT
		r.BootFlag = BIOS
		part = &Partition{
			Label:      "",
			Size:       cnst.BiosSize,
			PLabel:     cnst.BiosPLabel,
			FS:         "",
			MountPoint: "",
			Flags:      []string{BIOS},
		}
		r.Partitions = append(r.Partitions, part)
	} else {
		r.PartTable = MSDOS
		r.BootFlag = BOOT
		statePartFlags = []string{BOOT}
	}

	part = &Partition{
		Label:      cnst.OEMLabel,
		Size:       cnst.OEMSize,
		PLabel:     cnst.OEMPLabel,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.OEMDir,
		Flags:      []string{},
	}
	if r.OEMLabel != "" {
		part.Label = r.OEMLabel
	}
	r.Partitions = append(r.Partitions, part)

	part = &Partition{
		Label:      cnst.StateLabel,
		Size:       cnst.StateSize,
		PLabel:     cnst.StatePLabel,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.StateDir,
		Flags:      statePartFlags,
	}
	if r.StateLabel != "" {
		part.Label = r.StateLabel
	}
	r.Partitions = append(r.Partitions, part)

	part = &Partition{
		Label:      cnst.RecoveryLabel,
		Size:       cnst.RecoverySize,
		PLabel:     cnst.RecoveryPLabel,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.RecoveryDir,
		Flags:      []string{},
	}
	if r.RecoveryLabel != "" {
		part.Label = r.RecoveryLabel
	}
	r.Partitions = append(r.Partitions, part)

	part = &Partition{
		Label:      cnst.PersistentLabel,
		Size:       cnst.PersistentSize,
		PLabel:     cnst.PersistentPLabel,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.PersistentDir,
		Flags:      []string{},
	}
	if r.PersistentLabel != "" {
		part.Label = r.PersistentLabel
	}
	r.Partitions = append(r.Partitions, part)
}

// setupLuet will initialize Luet interface if required
func (r *RunConfig) setupLuet() {
	if r.DockerImg != "" {
		plugins := []string{}
		if r.Cosign && r.CosignPubKey == "" {
			r.Logger.Warnf("Keyless cosign verification is experimental, consider setting a public key")
		}
		if !r.NoVerify {
			plugins = append(plugins, cnst.LuetMtreePlugin)
		}
		r.Luet = NewLuet(r.Logger, context.NewContext(), &dockTypes.AuthConfig{}, plugins...)
	}
}

// DigestSetup will gather what partition table and bootflag we need for the current system
func (r *RunConfig) DigestSetup() {
	r.setupStyle()
	r.setupLuet()
}

// BuildConfig represents the config we need for building isos, raw images, artifacts
type BuildConfig struct {
	Label string `yaml:"label,omitempty" mapstructure:"label"`
}
