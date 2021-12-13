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
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
)

type InstallAction struct {
	Config *v1.RunConfig
}

func NewInstallAction(config *v1.RunConfig) *InstallAction {
	return &InstallAction{Config: config}
}

func (i InstallAction) Run() error {
	i.Config.Logger.Infof("InstallAction called")
	// Rough steps (then they have multisteps inside)
	// Remember to hook the yip hooks (before-install, after-install-chroot, after-install)
	// Check device valid
	// partition device
	// check source to install
	// install Active
	// install grub
	// Relabel SELinux
	// Unmount everything
	// install Recovery
	// install Secondary
	// Rebrand
	// ????
	// profit!
	return nil
}
