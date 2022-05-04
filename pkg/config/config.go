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
	"github.com/rancher-sandbox/elemental/pkg/cloudinit"
	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/http"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
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
	c := &v1.Config{
		Fs:      vfs.OSFS,
		Logger:  log,
		Syscall: &v1.RealSyscall{},
		Client:  http.NewClient(),
		Repos:   []v1.Repository{},
		Arch:    "x86_64",
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
		c.Mounter = mount.New(cnst.MountBinary)
	}
	return c
}

func NewRunConfig(opts ...GenericOptions) *v1.RunConfig {
	r := &v1.RunConfig{
		Config: *NewConfig(opts...),
	}
	// Set defaults if empty
	if r.GrubConf == "" {
		r.GrubConf = cnst.GrubConf
	}

	if r.ActiveLabel == "" {
		r.ActiveLabel = cnst.ActiveLabel
	}

	if r.PassiveLabel == "" {
		r.PassiveLabel = cnst.PassiveLabel
	}

	if r.SystemLabel == "" {
		r.SystemLabel = cnst.SystemLabel
	}

	if r.RecoveryLabel == "" {
		r.RecoveryLabel = cnst.RecoveryLabel
	}

	if r.PersistentLabel == "" {
		r.PersistentLabel = cnst.PersistentLabel
	}

	if r.OEMLabel == "" {
		r.OEMLabel = cnst.OEMLabel
	}

	if r.StateLabel == "" {
		r.StateLabel = cnst.StateLabel
	}

	r.Partitions = v1.PartitionList{}
	r.Images = v1.ImageMap{}

	if r.GrubDefEntry == "" {
		r.GrubDefEntry = cnst.GrubDefEntry
	}

	if r.ImgSize == 0 {
		r.ImgSize = cnst.ImgSize
	}
	return r
}

func NewISO() *v1.LiveISO {
	return &v1.LiveISO{
		Label:       cnst.ISOLabel,
		UEFI:        cnst.GetDefaultISOUEFI(),
		Image:       cnst.GetDefaultISOImage(),
		HybridMBR:   cnst.IsoHybridMBR,
		BootFile:    cnst.IsoBootFile,
		BootCatalog: cnst.IsoBootCatalog,
	}
}

func NewBuildConfig(opts ...GenericOptions) *v1.BuildConfig {
	b := &v1.BuildConfig{
		Config: *NewConfig(opts...),
		ISO:    NewISO(),
		Name:   cnst.BuildImgName,
	}
	return b
}
