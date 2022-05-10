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

package action

import (
	"github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/luet"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	"github.com/sirupsen/logrus"
)

// Hook is RunStage wrapper that only adds logic to ignore errors
// in case v1.RunConfig.Strict is set to false
func Hook(config *v1.RunConfig, hook string) error {
	config.Logger.Infof("Running %s hook", hook)
	oldLevel := config.Logger.GetLevel()
	config.Logger.SetLevel(logrus.ErrorLevel)
	err := utils.RunStage(hook, config)
	config.Logger.SetLevel(oldLevel)
	if !config.Strict {
		err = nil
	}
	return err
}

// ChrootHook executes Hook inside a chroot environment
func ChrootHook(config *v1.RunConfig, hook string, chrootDir string, bindMounts map[string]string) (err error) {
	chroot := utils.NewChroot(chrootDir, config)
	chroot.SetExtraMounts(bindMounts)
	callback := func() error {
		return Hook(config, hook)
	}
	return chroot.RunCallback(callback)
}

// SetupLuet sets the Luet object with the appropriate plugins
func SetupLuet(config *v1.RunConfig) {
	var plugins []string
	if config.DockerImg != "" {
		if !config.NoVerify {
			plugins = append(plugins, constants.LuetMtreePlugin)
		}
	}
	tmpDir := utils.GetTempDir(config, "")
	config.Luet = luet.NewLuet(luet.WithLogger(config.Logger), luet.WithPlugins(plugins...), luet.WithLuetTempDir(tmpDir))
}

// SetPartitionsFromScratch initiates all defaults partitions in order is they
// would be on a fresh installation. It does not run any kind of block device analysis
// it only populates partitions from defaults or configurations.
func SetPartitionsFromScratch(config *v1.RunConfig) {
	_, err := config.Fs.Stat(constants.EfiDevice)
	efiExists := err == nil
	var statePartFlags []string
	var part *v1.Partition

	if config.ForceEfi || efiExists {
		config.PartTable = v1.GPT
		config.BootFlag = v1.ESP
		part = &v1.Partition{
			Label:      constants.EfiLabel,
			Size:       constants.EfiSize,
			Name:       constants.EfiPartName,
			FS:         constants.EfiFs,
			MountPoint: constants.EfiDir,
			Flags:      []string{v1.ESP},
		}
		config.Partitions = append(config.Partitions, part)
	} else if config.ForceGpt {
		config.PartTable = v1.GPT
		config.BootFlag = v1.BIOS
		part = &v1.Partition{
			Label:      "",
			Size:       constants.BiosSize,
			Name:       constants.BiosPartName,
			FS:         "",
			MountPoint: "",
			Flags:      []string{v1.BIOS},
		}
		config.Partitions = append(config.Partitions, part)
	} else {
		config.PartTable = v1.MSDOS
		config.BootFlag = v1.BOOT
		statePartFlags = []string{v1.BOOT}
	}

	part = &v1.Partition{
		Label:      config.OEMLabel,
		Size:       constants.OEMSize,
		Name:       constants.OEMPartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.OEMDir,
		Flags:      []string{},
	}
	config.Partitions = append(config.Partitions, part)

	part = &v1.Partition{
		Label:      config.StateLabel,
		Size:       constants.StateSize,
		Name:       constants.StatePartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.StateDir,
		Flags:      statePartFlags,
	}
	config.Partitions = append(config.Partitions, part)

	part = &v1.Partition{
		Label:      config.RecoveryLabel,
		Size:       constants.RecoverySize,
		Name:       constants.RecoveryPartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.RecoveryDir,
		Flags:      []string{},
	}
	config.Partitions = append(config.Partitions, part)

	part = &v1.Partition{
		Label:      config.PersistentLabel,
		Size:       constants.PersistentSize,
		Name:       constants.PersistentPartName,
		FS:         constants.LinuxFs,
		MountPoint: constants.PersistentDir,
		Flags:      []string{},
	}
	config.Partitions = append(config.Partitions, part)
}
