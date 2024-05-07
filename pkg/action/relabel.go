/*
Copyright © 2022 - 2024 SUSE LLC

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
	"fmt"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

func SelinuxRelabel(cfg *types.RunConfig, spec *types.MountSpec) error {
	if !spec.SelinuxRelabel {
		cfg.Logger.Debug("SELinux relabeling disabled, skipping")
		return nil
	}

	if exists, _ := utils.Exists(cfg.Fs, constants.SELinuxTargetedContextFile); !exists {
		cfg.Logger.Debug("Could not find selinux policy context file")
		return nil
	}

	if !cfg.Runner.CommandExists("setfiles") {
		cfg.Logger.Debug("Could not find selinux setfiles utility")
		return nil
	}

	paths := []string{}
	paths = append(paths, spec.Ephemeral.Paths...)
	paths = append(paths, spec.Persistent.Paths...)

	// Some extended attributes are lost on copy-up bsc#1210690.
	// Workaround visit children first, then parents
	cfg.Logger.Debugf("Running setfiles on depth-sorted files in %s chroot", spec.Sysroot)
	for _, path := range paths {
		out, err := cfg.Config.Runner.Run(fmt.Sprintf("find %s -depth -exec setfiles -i -F -v %s {} +", path, constants.SELinuxTargetedContextFile))
		cfg.Logger.Debugf("setfiles output: %s", string(out))
		if err != nil {
			cfg.Logger.Errorf("Error running setfiles in %s: %s", path, err.Error())
			return err
		}
	}

	return nil
}
