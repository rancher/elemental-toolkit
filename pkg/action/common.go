/*
Copyright © 2022 - 2026 SUSE LLC

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
	"github.com/sirupsen/logrus"

	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

// Hook is RunStage wrapper that only adds logic to ignore errors
// in case types.RunConfig.Strict is set to false
func Hook(config *types.Config, hook string, strict bool, cloudInitPaths ...string) error {
	config.Logger.Infof("Running %s hook", hook)
	oldLevel := config.Logger.GetLevel()
	config.Logger.SetLevel(logrus.ErrorLevel)
	err := utils.RunStage(config, hook, strict, cloudInitPaths...)
	config.Logger.SetLevel(oldLevel)
	if !strict {
		err = nil
	}
	return err
}

// ChrootHook executes Hook inside a chroot environment
func ChrootHook(config *types.Config, hook string, strict bool, chrootDir string, bindMounts map[string]string, cloudInitPaths ...string) (err error) {
	callback := func() error {
		return Hook(config, hook, strict, cloudInitPaths...)
	}
	return utils.ChrootedCallback(config, chrootDir, bindMounts, callback)
}

// PowerAction executes a power-action (Reboot/PowerOff) after completed
// install or upgrade and returns any encountered error.
func PowerAction(cfg *types.RunConfig) error {
	// Reboot, poweroff or nothing
	var (
		err  error
		code int
	)

	if cfg.Reboot {
		cfg.Logger.Infof("Rebooting in 5 seconds")
		if err = utils.Reboot(cfg.Runner, 5); err != nil {
			code = elementalError.Reboot
		}
	} else if cfg.PowerOff {
		cfg.Logger.Infof("Shutting down in 5 seconds")
		if err = utils.Shutdown(cfg.Runner, 5); err != nil {
			code = elementalError.PowerOff
		}
	}

	return elementalError.NewFromError(err, code)
}
