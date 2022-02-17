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
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"net/http"
)

func NewRunConfig(opts ...v1.RunConfigOptions) *v1.RunConfig {
	log := v1.NewLogger()
	r := &v1.RunConfig{
		Fs:      afero.NewOsFs(),
		Logger:  log,
		Runner:  &v1.RealRunner{},
		Syscall: &v1.RealSyscall{},
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
		r.CloudInitRunner = cloudinit.NewYipCloudInitRunner(r.Logger, r.Runner)
	}

	if r.Mounter == nil {
		r.Mounter = mount.New(cnst.MountBinary)
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

	if r.IsoMnt == "" {
		r.IsoMnt = cnst.IsoMnt
	}

	if r.GrubDefEntry == "" {
		r.GrubDefEntry = cnst.GrubDefEntry
	}

	if r.ImgSize == 0 {
		r.ImgSize = cnst.ImgSize
	}
	return r
}
