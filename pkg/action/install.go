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

package action

import (
	"errors"
	"fmt"
	"github.com/rancher-sandbox/elemental-cli/pkg/elemental"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
)

// InstallAction represents the struct that will run the full install from start to finish
type InstallAction struct {
	Config *v1.RunConfig
}

func NewInstallAction(config *v1.RunConfig) *InstallAction {
	return &InstallAction{Config: config}
}

// Run will install the cos system to a device by following several steps
func (i InstallAction) Run() error {
	var err error

	newElemental := elemental.NewElemental(i.Config)
	i.Config.Logger.Infof("InstallAction called")
	// Install steps really starts here
	i.Config.SetupStyle()
	disk := part.NewDisk(
		i.Config.Target,
		part.WithRunner(i.Config.Runner),
		part.WithFS(i.Config.Fs),
		part.WithLogger(i.Config.Logger),
	)
	// get_iso: _COS_INSTALL_ISO_URL -> download -> mount
	// cos.GetIso() ?

	// Remember to hook the yip hooks (before-install, after-install-chroot, after-install)
	// This will probably need the yip module to be used before being able?

	// Check device valid
	if !disk.Exists() {
		i.Config.Logger.Errorf("Disk %s does not exist", i.Config.Target)
		return errors.New(fmt.Sprintf("Disk %s does not exist", i.Config.Target))
	}

	// Check no-format flag and force flag against current device
	err = newElemental.CheckNoFormat()
	if err != nil {
		return err
	}
	// partition device
	// TODO handle non partitioning case
	err = newElemental.PartitionAndFormatDevice(disk)
	if err != nil {
		return err
	}
	// install Active
	err = newElemental.CopyCos()
	if err != nil {
		return err
	}
	// Copy cloud-init if any
	err = newElemental.CopyCloudConfig()
	if err != nil {
		return err
	}
	// install grub
	grub := utils.NewGrub(i.Config)
	err = grub.Install()
	if err != nil {
		return err
	}
	// Relabel SELinux
	_ = newElemental.SelinuxRelabel(false)
	// Unmount everything
	// cos.CleanupMounts()
	// install Recovery
	// cos.CopyRecovery()
	// install Secondary
	// cos.CopyPassive()
	// Rebrand
	// cos.Rebrand()
	// ????
	// profit!
	return nil
}
