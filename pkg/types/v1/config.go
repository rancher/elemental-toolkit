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
		r.Mounter = mount.New("/usr/bin/mount")
	}

	// Set defaults if empty
	if r.GrubConf == "" {
		r.GrubConf = "/etc/cos/grub.cfg"
	}
	if r.StateDir == "" {
		r.StateDir = "/run/initramfs/cos-state"
	}

	if r.ActiveLabel == "" {
		r.ActiveLabel = "COS_ACTIVE"
	}

	if r.PassiveLabel == "" {
		r.ActiveLabel = "COS_PASSIVE"
	}
	return r
}

type RunConfig struct {
	Device       string `yaml:"device,omitempty" mapstructure:"device"`
	Target       string `yaml:"target,omitempty" mapstructure:"target"`
	Source       string `yaml:"source,omitempty" mapstructure:"source"`
	CloudInit    string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	ForceEfi     bool   `yaml:"force-efi,omitempty" mapstructure:"force-efi"`
	ForceGpt     bool   `yaml:"force-gpt,omitempty" mapstructure:"force-gpt"`
	Tty          string `yaml:"tty,omitempty" mapstructure:"tty"`
	NoFormat     bool   `yaml:"no-format,omitempty" mapstructure:"no-format"`
	ActiveLabel  string `yaml:"ACTIVE_LABEL,omitempty" mapstructure:"ACTIVE_LABEL"`
	PassiveLabel string `yaml:"PASSIVE_LABEL,omitempty" mapstructure:"PASSIVE_LABEL"`
	Force        bool   `yaml:"force,omitempty" mapstructure:"force"`
	PartTable    string
	BootFlag     string
	StateDir     string
	GrubConf     string
	Logger       Logger
	Fs           afero.Fs
	Mounter      mount.Interface
	Runner       Runner
	Syscall      SyscallInterface
}

func (r *RunConfig) SetupStyle() {
	var part, boot string

	_, err := r.Fs.Stat("/sys/firmware/efi")
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

type BuildConfig struct {
	Label string `yaml:"label,omitempty" mapstructure:"label"`
}
