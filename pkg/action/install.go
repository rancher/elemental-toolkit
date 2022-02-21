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
	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/elemental"
	"github.com/rancher-sandbox/elemental/pkg/partitioner"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	"github.com/spf13/afero"
	"path/filepath"
)

func installHook(config *v1.RunConfig, hook string, chroot bool) error {
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

// InstallImagesSetup defines the parameters of active, passive and recovery
// images of an installation
func InstallImagesSetup(config *v1.RunConfig) error {
	partState := config.Partitions.GetByName(cnst.StatePartName)
	if partState == nil {
		config.Logger.Errorf("State partition not configured")
		return errors.New("Error setting Active image")
	}
	// Set active image
	activeImg := v1.Image{
		Label:      config.ActiveLabel,
		Size:       cnst.ImgSize,
		File:       filepath.Join(partState.MountPoint, "cOS", cnst.ActiveImgFile),
		FS:         cnst.LinuxImgFs,
		MountPoint: cnst.ActiveDir,
	}

	//TODO add installation from channel
	if config.DockerImg != "" {
		activeImg.Source = v1.NewDockerSrc(config.DockerImg)
	} else if config.Directory != "" {
		activeImg.Source = v1.NewDirSrc(config.Directory)
	} else {
		activeImg.Source = v1.NewDirSrc(cnst.IsoBaseTree)
	}

	// Set passive image
	passiveImg := v1.Image{
		File:   filepath.Join(partState.MountPoint, "cOS", cnst.PassiveImgFile),
		Label:  config.PassiveLabel,
		Source: v1.NewFileSrc(activeImg.File),
		FS:     cnst.LinuxImgFs,
	}

	// Set recovery image
	partRecovery := config.Partitions.GetByName(cnst.RecoveryPartName)
	if partState == nil {
		config.Logger.Errorf("Recovery partition not configured")
		return errors.New("Error setting Recovery image")
	}

	// TODO use iso for all images? Formerly it was only for recovery, but
	// I think this was a regression from former cos.sh script refactor
	isoMnt := cnst.IsoMnt
	if config.Iso != "" {
		isoMnt = cnst.DownloadedIsoMnt
	}
	recoveryDirCos := filepath.Join(partRecovery.MountPoint, "cOS")
	squashedImgSource := filepath.Join(isoMnt, cnst.RecoverySquashFile)

	recoveryImg := v1.Image{}
	if exists, _ := afero.Exists(config.Fs, squashedImgSource); exists {
		recoveryImg.File = filepath.Join(recoveryDirCos, cnst.RecoverySquashFile)
		recoveryImg.Source = v1.NewFileSrc(squashedImgSource)
		recoveryImg.FS = cnst.SquashFs
	} else {
		recoveryImg.File = filepath.Join(recoveryDirCos, cnst.RecoveryImgFile)
		recoveryImg.Source = v1.NewFileSrc(activeImg.File)
		recoveryImg.FS = cnst.LinuxImgFs
		recoveryImg.Label = config.SystemLabel
	}

	// Add images to config
	config.Images.SetActive(&activeImg)
	config.Images.SetPassive(&passiveImg)
	config.Images.SetRecovery(&recoveryImg)

	return nil
}

// InstallSetup will set installation parameters according to
// the given configuration flags
func InstallSetup(config *v1.RunConfig) error {
	SetPartitionsFromScratch(config)
	InstallImagesSetup(config)
	SetupLuet(config)

	return nil
}

// Run will install the system from a given configuration
func InstallRun(config *v1.RunConfig) (err error) {
	newElemental := elemental.NewElemental(config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	disk := partitioner.NewDisk(
		config.Target,
		partitioner.WithRunner(config.Runner),
		partitioner.WithFS(config.Fs),
		partitioner.WithLogger(config.Logger),
	)

	err = installHook(config, cnst.BeforeInstallHook, false)
	if err != nil {
		return err
	}

	// Make config.Iso to also install active from downloaded ISO
	if config.Iso != "" {
		tmpDir, err := newElemental.GetIso()
		if err != nil {
			return err
		}
		cleanup.Push(func() error {
			config.Logger.Infof("Unmounting downloaded ISO")
			config.Fs.RemoveAll(tmpDir)
			return config.Mounter.Unmount(cnst.DownloadedIsoMnt)
		})
	}

	// Check device valid
	if !disk.Exists() {
		config.Logger.Errorf("Disk %s does not exist", config.Target)
		return errors.New(fmt.Sprintf("Disk %s does not exist", config.Target))
	}

	// Check no-format flag
	if config.NoFormat {
		// Check force flag against current device
		err = newElemental.CheckNoFormat()
		if err != nil {
			return err
		}
	} else {
		// Partition device
		err = newElemental.PartitionAndFormatDevice(disk)
		if err != nil {
			return err
		}
	}

	err = newElemental.MountPartitions()
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return newElemental.UnmountPartitions() })

	// Deploy active image
	err = newElemental.DeployImage(config.Images.GetActive(), true)
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return newElemental.UnmountImage(config.Images.GetActive()) })

	// Copy cloud-init if any
	err = newElemental.CopyCloudConfig()
	if err != nil {
		return err
	}
	// Install grub
	grub := utils.NewGrub(config)
	err = grub.Install()
	if err != nil {
		return err
	}
	// Relabel SELinux
	_ = newElemental.SelinuxRelabel(cnst.ActiveDir, false)

	err = installHook(config, cnst.AfterInstallChrootHook, true)
	if err != nil {
		return err
	}

	// Unmount active image
	err = newElemental.UnmountImage(config.Images.GetActive())
	if err != nil {
		return err
	}
	// Install Recovery
	err = newElemental.DeployImage(config.Images.GetRecovery(), false)
	if err != nil {
		return err
	}
	// Install Passive
	err = newElemental.DeployImage(config.Images.GetPassive(), false)
	if err != nil {
		return err
	}

	err = installHook(config, cnst.AfterInstallHook, false)
	if err != nil {
		return err
	}

	// Installation rebrand (only grub for now)
	err = newElemental.Rebrand()
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
