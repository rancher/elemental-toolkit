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
	"github.com/rancher-sandbox/elemental-cli/pkg/constants"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
)

const (
	GPT   = "gpt"
	ESP   = "esp"
	BIOS  = "bios_grub"
	MSDOS = "msdos"
	BOOT  = "boot"
	VFAT  = "vfat"
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

func NewRunConfig(opts ...RunConfigOptions) *RunConfig {
	r := &RunConfig{
		Fs:      afero.NewOsFs(),
		Logger:  logrus.New(),
		Runner:  &RealRunner{},
		Syscall: &RealSyscall{},
	}
	for _, o := range opts {
		err := o(r)
		if err != nil {
			return nil
		}
	}

	if r.Mounter == nil {
		r.Mounter = mount.New(constants.MountBinary)
	}

	// Set defaults if empty
	if r.GrubConf == "" {
		r.GrubConf = constants.GrubConf
	}
	if r.StateDir == "" {
		r.StateDir = constants.StateDir
	}

	if r.ActiveLabel == "" {
		r.ActiveLabel = constants.ActiveLabel
	}

	if r.PassiveLabel == "" {
		r.PassiveLabel = constants.PassiveLabel
	}

	if r.SystemLabel == "" {
		r.SystemLabel = constants.SystemLabel
	}

	r.RecoveryPart = Partition{
		Label:  constants.RecoveryLabel,
		Size:   constants.RecoverySize,
		PLabel: constants.RecoveryPLabel,
		FS:     constants.LinuxFs,
	}
	if r.recoveryLabel != "" {
		r.RecoveryPart.Label = r.recoveryLabel
	}

	r.PersistentPart = Partition{
		Label:  constants.PersistentLabel,
		Size:   constants.PersistentSize,
		PLabel: constants.PersistentPLabel,
		FS:     constants.LinuxFs,
	}
	if r.persistentLabel != "" {
		r.PersistentPart.Label = r.persistentLabel
	}

	r.OEMPart = Partition{
		Label:  constants.OEMLabel,
		Size:   constants.OEMSize,
		PLabel: constants.OEMPLabel,
		FS:     constants.LinuxFs,
	}
	if r.oEMLabel != "" {
		r.OEMPart.Label = r.oEMLabel
	}

	r.StatePart = Partition{
		Label:  constants.StateLabel,
		Size:   constants.StateSize,
		PLabel: constants.StatePLabel,
		FS:     constants.LinuxFs,
	}
	if r.stateLabel != "" {
		r.StatePart.Label = r.stateLabel
	}
	return r
}

// RunConfig is the struct that represents the full configuration needed for install, upgrade, reset, rebrand.
// Basically everything needed to know for all operations in a running system, not related to builds
type RunConfig struct {
	//Interanlly used to compute RunConfig state
	recoveryLabel   string `yaml:"RECOVERY_LABEL,omitempty" mapstructure:"RECOVERY_LABEL"`
	persistentLabel string `yaml:"PERSISTENT_LABEL,omitempty" mapstructure:"PERSISTENT_LABEL"`
	stateLabel      string `yaml:"STATE_LABEL,omitempty" mapstructure:"STATE_LABEL"`
	oEMLabel        string `yaml:"OEM_LABEL,omitempty" mapstructure:"OEM_LABEL"`

	Device       string `yaml:"device,omitempty" mapstructure:"device"`
	Target       string `yaml:"target,omitempty" mapstructure:"target"`
	Source       string `yaml:"source,omitempty" mapstructure:"source"`
	CloudInit    string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	ForceEfi     bool   `yaml:"force-efi,omitempty" mapstructure:"force-efi"`
	ForceGpt     bool   `yaml:"force-gpt,omitempty" mapstructure:"force-gpt"`
	PartLayout   string `yaml:"partition-layout,omitempty" mapstructure:"partition-layout"`
	Tty          string `yaml:"tty,omitempty" mapstructure:"tty"`
	NoFormat     bool   `yaml:"no-format,omitempty" mapstructure:"no-format"`
	ActiveLabel  string `yaml:"ACTIVE_LABEL,omitempty" mapstructure:"ACTIVE_LABEL"`
	PassiveLabel string `yaml:"PASSIVE_LABEL,omitempty" mapstructure:"PASSIVE_LABEL"`
	SystemLabel  string `yaml:"SYSTEM_LABEL,omitempty" mapstructure:"SYSTEM_LABEL"`
	Force        bool   `yaml:"force,omitempty" mapstructure:"force"`

	PartTable      string
	BootFlag       string
	StateDir       string
	GrubConf       string
	Logger         Logger
	Fs             afero.Fs
	Mounter        mount.Interface
	Runner         Runner
	Syscall        SyscallInterface
	RecoveryPart   Partition
	PersistentPart Partition
	StatePart      Partition
	OEMPart        Partition
}

// Partition struct represents a partition with its commonly configurable values, size in MiB
type Partition struct {
	Label  string
	Size   uint
	PLabel string
	FS     string
}

// SetupStyle will gather what partition table and bootflag we need for the current system
func (r *RunConfig) SetupStyle() {
	var part, boot string

	_, err := r.Fs.Stat(constants.EfiDevice)
	efiExists := err == nil

	if r.ForceEfi || efiExists {
		part = GPT
		boot = ESP
	} else if r.ForceGpt {
		part = GPT
		boot = BIOS
	} else {
		part = MSDOS
		boot = BOOT
	}

	r.PartTable = part
	r.BootFlag = boot
}

// BuildConfig represents the config we need for building isos, raw images, artifacts
type BuildConfig struct {
	Label string `yaml:"label,omitempty" mapstructure:"label"`
}
