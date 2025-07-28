/*
Copyright © 2022 - 2025 SUSE LLC

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
	"strings"

	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	"github.com/rancher/elemental-toolkit/pkg/features"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

func RunInit(cfg *v1.RunConfig, spec *v1.InitSpec) error {
	if exists, _ := utils.Exists(cfg.Fs, "/.dockerenv"); !exists && !spec.Force {
		return elementalError.New("running outside of container, pass --force to run anyway", elementalError.StatFile)
	}

	features, err := features.Get(spec.Features)
	if err != nil {
		cfg.Config.Logger.Errorf("Error getting features: %s", err.Error())
		return err
	}

	if err := utils.CreateDirStructure(cfg.Config.Fs, "/"); err != nil {
		cfg.Config.Logger.Errorf("Error creating directories: %s", err.Error())
		return err
	}

	cfg.Config.Logger.Infof("Running init action with features: %s", strings.Join(spec.Features, ","))

	// Install enabled features
	for _, feature := range features {
		cfg.Config.Logger.Debugf("Installing feature: %s", feature.Name)
		if err := feature.Install(cfg.Config.Logger, cfg.Config.Fs, cfg.Config.Runner); err != nil {
			cfg.Config.Logger.Errorf("Error installing feature '%s': %v", feature.Name, err.Error())
			return err
		}
	}

	if !spec.Mkinitrd {
		cfg.Config.Logger.Debugf("Skipping initrd.")
		return nil
	}

	cfg.Config.Logger.Infof("Generate initrd.")
	output, err := cfg.Runner.Run("dracut", "-f", "--regenerate-all")
	if err != nil {
		cfg.Config.Logger.Errorf("dracut failed with output: %s", output)
	}

	return err
}
