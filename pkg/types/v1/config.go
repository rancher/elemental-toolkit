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
	"github.com/spf13/afero"
)

const (
	GPT = "gpt"
	ESP = "esp"
	BIOS = "bios_grub"
	MSDOS = "msdos"
	BOOT = "boot"
)

type RunConfigOptions func(a *RunConfig) error

func WithFs(fs afero.Fs) func(r *RunConfig) error {
	return func(r *RunConfig) error {
		r.fs = fs
		return nil
	}
}

func NewRunConfig(opts...RunConfigOptions) *RunConfig {
	r := &RunConfig{
		fs: afero.NewOsFs(),
	}
	for _, o := range opts {
		err := o(r)
		if err != nil {
			return nil
		}
	}
	return r
}

type RunConfig struct {
	Device string `yaml:"device,omitempty" mapstructure:"device"`
	Target string `yaml:"target,omitempty" mapstructure:"target"`
	Source string `yaml:"source,omitempty" mapstructure:"source"`
	CloudInit string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	ForceEfi bool `yaml:"force-efi,omitempty" mapstructure:"force-efi"`
	ForceGpt bool `yaml:"force-gpt,omitempty" mapstructure:"force-gpt"`
	PartTable string
	BootFlag string
	fs afero.Fs
	logger Logger
}

func (r *RunConfig) SetupStyle() {
	var part,boot string

	_, err := r.fs.Stat("/sys/firmware/efi")
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