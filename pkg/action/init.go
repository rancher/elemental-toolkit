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
	"fmt"
	"strings"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/features"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

func RunInit(cfg *types.RunConfig, spec *types.InitSpec) error {
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

	cfg.Config.Logger.Infof("Find Kernel")
	kernel, version, err := utils.FindKernel(cfg.Fs, "/")
	if err != nil {
		cfg.Config.Logger.Errorf("could not find kernel or kernel version")
		return err
	}

	if kernel != constants.KernelPath {
		cfg.Config.Logger.Debugf("Creating kernel symlink from %s to %s", kernel, constants.KernelPath)
		_ = cfg.Fs.Remove(constants.KernelPath)
		err = cfg.Fs.Symlink(kernel, constants.KernelPath)
		if err != nil {
			cfg.Config.Logger.Errorf("failed creating kernel symlink")
			return err
		}
	}

	cfg.Config.Logger.Info("Remove any pre-existing initrd")
	err = removeElementalInitrds(cfg)
	if err != nil {
		cfg.Config.Logger.Errorf("failed removing any pre-existing Elemental initrd: %s", err.Error())
		return err
	}

	cfg.Config.Logger.Infof("Generate initrd.")
	output, err := cfg.Runner.Run("dracut", "-f", fmt.Sprintf("%s-%s", constants.ElementalInitrd, version), version)
	if err != nil {
		cfg.Config.Logger.Errorf("dracut failed with output: %s", output)
	}

	cfg.Config.Logger.Debugf("darcut output: %s", output)

	initrd, err := utils.FindInitrd(cfg.Fs, "/")
	if err != nil || !strings.HasPrefix(initrd, constants.ElementalInitrd) {
		cfg.Config.Logger.Errorf("could not find generated initrd")
		return err
	}

	cfg.Config.Logger.Debugf("Creating initrd symlink from %s to %s", initrd, constants.InitrdPath)
	_ = cfg.Fs.Remove(constants.InitrdPath)
	err = cfg.Fs.Symlink(initrd, constants.InitrdPath)
	if err != nil {
		cfg.Config.Logger.Errorf("failed creating initrd symlink")
	}

	return err
}

func removeElementalInitrds(cfg *types.RunConfig) error {
	initImgs, err := utils.FindFiles(cfg.Fs, "/", fmt.Sprintf("%s-*", constants.ElementalInitrd))
	if err != nil {
		return err
	}
	for _, img := range initImgs {
		cfg.Config.Logger.Debugf("Removing pre-existing elemental initrd file: %s", img)
		err = cfg.Fs.Remove(img)
		if err != nil {
			return err
		}
	}
	return nil
}
