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
	"errors"
	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/elemental"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	"github.com/spf13/afero"
	"path/filepath"
)

func resetHook(config *v1.RunConfig, hook string, chroot bool) error {
	if chroot {
		extraMounts := map[string]string{}
		persistent := config.Partitions.GetByName(cnst.PersistentPartName)
		if persistent != nil {
			extraMounts[persistent.MountPoint] = "/usr/local"
		}
		oem := config.Partitions.GetByName(cnst.OEMPartName)
		if oem != nil {
			extraMounts[oem.MountPoint] = "/oem"
		}
		return ActionChrootHook(config, hook, config.Images.GetActive().MountPoint, extraMounts)
	}
	return ActionHook(config, hook)
}

// ResetSetup will set installation parameters according to
// the given configuration flags
func ResetSetup(config *v1.RunConfig) error {
	if !utils.BootedFrom(config.Runner, cnst.RecoverySquashFile) &&
		!utils.BootedFrom(config.Runner, config.SystemLabel) {
		return errors.New("Reset can only be called from the recovery system")
	}

	SetupLuet(config)

	efiExists, _ := afero.Exists(config.Fs, cnst.EfiDevice)

	if efiExists {
		partEfi, err := utils.GetFullDeviceByLabel(config.Runner, cnst.EfiLabel, 1)
		if err != nil {
			return err
		}
		if partEfi.MountPoint == "" {
			partEfi.MountPoint = cnst.EfiDir
		}
		partEfi.Name = cnst.EfiPartName
		config.Partitions = append(config.Partitions, &partEfi)
	}

	// Only add it if it exists, not a hard requirement
	partOEM, err := utils.GetFullDeviceByLabel(config.Runner, cnst.OEMLabel, 1)
	if err == nil {
		if partOEM.MountPoint == "" {
			partOEM.MountPoint = cnst.OEMDir
		}
		partOEM.Name = cnst.OEMPartName
		config.Partitions = append(config.Partitions, &partOEM)
	} else {
		config.Logger.Warnf("No OEM partition found")
	}

	partState, err := utils.GetFullDeviceByLabel(config.Runner, cnst.StateLabel, 1)
	if err != nil {
		return err
	}
	if partState.MountPoint == "" {
		partState.MountPoint = cnst.StateDir
	}
	partState.Name = cnst.StatePartName
	config.Partitions = append(config.Partitions, &partState)
	config.Target = partState.Disk

	// Only add it if it exists, not a hard requirement
	partPersistent, err := utils.GetFullDeviceByLabel(config.Runner, cnst.PersistentLabel, 1)
	if err == nil {
		if partPersistent.MountPoint == "" {
			partPersistent.MountPoint = cnst.PersistentDir
		}
		partPersistent.Name = cnst.PersistentPartName
		config.Partitions = append(config.Partitions, &partPersistent)
	} else {
		config.Logger.Warnf("No Persistent partition found")
	}

	ResetImagesSetup(config)

	return nil
}

// ResetImagesSetup defines the parameters of active and passive images
// as they are used during the reset.
func ResetImagesSetup(config *v1.RunConfig) error {
	var imgSource v1.ImageSource
	// TODO add reset from channel
	// TODO execute rootTree sanity checks?
	if config.Directory != "" {
		imgSource = v1.NewDirSrc(config.Directory)
	} else if config.DockerImg != "" {
		imgSource = v1.NewDockerSrc(config.DockerImg)
	} else if utils.BootedFrom(config.Runner, cnst.RecoverySquashFile) {
		imgSource = v1.NewDirSrc(cnst.IsoBaseTree)
	} else {
		imgSource = v1.NewFileSrc(filepath.Join(cnst.RunningStateDir, "cOS", cnst.RecoveryImgFile))
	}

	// Set Active Image
	partState := config.Partitions.GetByName(cnst.StatePartName)
	if partState == nil {
		config.Logger.Errorf("State partition not configured")
		return errors.New("Error setting Active image")
	}
	config.Images.SetActive(&v1.Image{
		Label:      config.ActiveLabel,
		Size:       cnst.ImgSize,
		File:       filepath.Join(partState.MountPoint, "cOS", cnst.ActiveImgFile),
		FS:         cnst.LinuxImgFs,
		Source:     imgSource,
		MountPoint: cnst.ActiveDir,
	})

	// Set Passive image
	config.Images.SetPassive(&v1.Image{
		File:   filepath.Join(partState.MountPoint, "cOS", cnst.PassiveImgFile),
		Label:  config.PassiveLabel,
		Source: v1.NewFileSrc(config.Images.GetActive().File),
		FS:     cnst.LinuxImgFs,
	})

	return nil
}

// ResetRun will reset the cos system to by following several steps
func ResetRun(config *v1.RunConfig) (err error) {
	ele := elemental.NewElemental(config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	err = resetHook(config, cnst.BeforeResetHook, false)
	if err != nil {
		return err
	}

	// Unmount partitions if any is already mounted before formatting
	err = ele.UnmountPartitions()
	if err != nil {
		return err
	}

	// Reformat state partition
	err = ele.FormatPartition(config.Partitions.GetByName(cnst.StatePartName))
	if err != nil {
		return err
	}
	// Reformat persistent partitions
	if config.ResetPersistent {
		persistent := config.Partitions.GetByName(cnst.PersistentPartName)
		if persistent != nil {
			err = ele.FormatPartition(persistent)
			if err != nil {
				return err
			}
		}
		oem := config.Partitions.GetByName(cnst.OEMPartName)
		if oem != nil {
			err = ele.FormatPartition(oem)
			if err != nil {
				return err
			}
		}
	}

	// Mount configured partitions
	err = ele.MountPartitions()
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return ele.UnmountPartitions() })

	// Depoly active image
	err = ele.DeployImage(config.Images.GetActive(), true)
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return ele.UnmountImage(config.Images.GetActive()) })

	// install grub
	grub := utils.NewGrub(config)
	err = grub.Install()
	if err != nil {
		return err
	}
	// Relabel SELinux
	_ = ele.SelinuxRelabel(cnst.ActiveDir, false)

	err = resetHook(config, cnst.AfterResetChrootHook, true)
	if err != nil {
		return err
	}

	// Unmount active image
	err = ele.UnmountImage(config.Images.GetActive())
	if err != nil {
		return err
	}

	// Install Passive
	err = ele.DeployImage(config.Images.GetPassive(), false)
	if err != nil {
		return err
	}

	err = resetHook(config, cnst.AfterResetHook, false)
	if err != nil {
		return err
	}

	// installation rebrand (only grub for now)
	err = ele.Rebrand()
	if err != nil {
		return err
	}

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return err
	}

	// Reboot, poweroff or nothing
	if config.Reboot {
		config.Logger.Infof("Rebooting in 5 seconds")
		return utils.Reboot(config.Runner, 5)
	} else if config.PowerOff {
		config.Logger.Infof("Shutting down in 5 seconds")
		return utils.Shutdown(config.Runner, 5)
	}
	return err
}
