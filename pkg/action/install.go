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
	"github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
)

type InstallAction struct {
	Config *v1.RunConfig
}

func NewInstallAction(config *v1.RunConfig) *InstallAction {
	return &InstallAction{Config: config}
}

func (i InstallAction) Run() error {
	i.Config.Logger.Infof("InstallAction called")
	i.Config.SetupStyle()
	// Rough steps (then they have multisteps inside)
	disk := partitioner.NewDisk(i.Config.Target, i.Config.Runner)
	// get_iso: _COS_INSTALL_ISO_URL -> download -> mount
	// get_image: _UPGRADE_IMAGE -> create_rootfs -> NOT NECESSARY FOR INSTALL!
	// Remember to hook the yip hooks (before-install, after-install-chroot, after-install)
	// Check device valid
	if !disk.Exists() {
		i.Config.Logger.Errorf("Disk %s does not exist", i.Config.Target)
		return errors.New(fmt.Sprintf("Disk %s does not exist", i.Config.Target))
	}
	if i.Config.NoFormat != "" {
		// User asked for no format, lets check if there is already those labeled partitions in the disk
		for _, label := range []string{i.Config.ActiveLabel, i.Config.PassiveLabel} {
			found, err := utils.FindLabel(i.Config.Runner, label)
			if err != nil {
				return err
			}
			if found != "" {
				if i.Config.Force {
					msg := fmt.Sprintf("Forcing overwrite of existing partitions due to `force` flag")
					i.Config.Logger.Infof(msg)
					break
				} else {
					msg := fmt.Sprintf("There is already an active deployment in the system, use '--force' flag to overwrite it")
					i.Config.Logger.Error(msg)
					return errors.New(msg)
				}
			}
		}
	}
	// partition device
	// install Active
	err := utils.DoCopy(i.Config)
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
	_ = utils.SelinuxRelabel(i.Config.Target, i.Config.Fs, false)
	// Unmount everything
	// install Recovery
	// install Secondary
	// Rebrand
	// ????
	// profit!
	return nil
}
