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
	"path/filepath"
)

func installHook(config *v1.RunConfig, hook string, chroot bool) error {
	if chroot {
		return actionChrootHook(
			config, hook, config.ActiveImage.MountPoint,
			map[string]string{
				cnst.PersistentDir: "/usr/local",
				cnst.OEMDir:        "/oem",
			},
		)
	}
	return actionHook(config, hook)
}

// InstallSetup will set installation parameters according to
// the given configuration flags
func InstallSetup(config *v1.RunConfig) error {
	_, err := config.Fs.Stat(cnst.EfiDevice)
	efiExists := err == nil
	statePartFlags := []string{}
	var part *v1.Partition

	if config.ForceEfi || efiExists {
		config.PartTable = v1.GPT
		config.BootFlag = v1.ESP
		part = &v1.Partition{
			Label:      cnst.EfiLabel,
			Size:       cnst.EfiSize,
			Name:       cnst.EfiPartName,
			FS:         cnst.EfiFs,
			MountPoint: cnst.EfiDir,
			Flags:      []string{v1.ESP},
		}
		config.Partitions = append(config.Partitions, part)
	} else if config.ForceGpt {
		config.PartTable = v1.GPT
		config.BootFlag = v1.BIOS
		part = &v1.Partition{
			Label:      "",
			Size:       cnst.BiosSize,
			Name:       cnst.BiosPartName,
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
		Size:       cnst.OEMSize,
		Name:       cnst.OEMPartName,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.OEMDir,
		Flags:      []string{},
	}
	config.Partitions = append(config.Partitions, part)

	part = &v1.Partition{
		Label:      config.StateLabel,
		Size:       cnst.StateSize,
		Name:       cnst.StatePartName,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.StateDir,
		Flags:      statePartFlags,
	}
	config.Partitions = append(config.Partitions, part)

	part = &v1.Partition{
		Label:      config.RecoveryLabel,
		Size:       cnst.RecoverySize,
		Name:       cnst.RecoveryPartName,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.RecoveryDir,
		Flags:      []string{},
	}
	config.Partitions = append(config.Partitions, part)

	part = &v1.Partition{
		Label:      config.PersistentLabel,
		Size:       cnst.PersistentSize,
		Name:       cnst.PersistentPartName,
		FS:         cnst.LinuxFs,
		MountPoint: cnst.PersistentDir,
		Flags:      []string{},
	}
	config.Partitions = append(config.Partitions, part)

	config.ActiveImage = v1.Image{
		Label:      config.ActiveLabel,
		Size:       cnst.ImgSize,
		File:       filepath.Join(cnst.StateDir, "cOS", cnst.ActiveImgFile),
		FS:         cnst.LinuxImgFs,
		RootTree:   cnst.IsoBaseTree,
		MountPoint: cnst.ActiveDir,
	}

	setupLuet(config)

	return nil
}

// Run will install the system from a given configuration
func InstallRun(config *v1.RunConfig) (err error) {
	newElemental := elemental.NewElemental(config)

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

	if config.Iso != "" {
		tmpDir, err := newElemental.GetIso()
		if err != nil {
			return err
		}
		defer func() {
			config.Logger.Infof("Unmounting downloaded ISO")
			if tmpErr := config.Mounter.Unmount(config.IsoMnt); tmpErr != nil && err == nil {
				err = tmpErr
			}
			config.Fs.RemoveAll(tmpDir)
		}()
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
	defer func() {
		if tmpErr := newElemental.UnmountPartitions(); tmpErr != nil && err == nil {
			err = tmpErr
		}
	}()

	// create active file system image
	err = newElemental.CreateFileSystemImage(config.ActiveImage)
	if err != nil {
		return err
	}

	//mount file system image
	err = newElemental.MountImage(&config.ActiveImage, "rw")
	if err != nil {
		return err
	}
	defer func() {
		if tmpErr := newElemental.UnmountImage(&config.ActiveImage); tmpErr != nil && err == nil {
			err = tmpErr
		}
	}()

	// install Active
	err = newElemental.CopyActive()
	if err != nil {
		return err
	}
	// Copy cloud-init if any
	err = newElemental.CopyCloudConfig()
	if err != nil {
		return err
	}
	// install grub
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
	err = newElemental.UnmountImage(&config.ActiveImage)
	if err != nil {
		return err
	}
	// install Recovery
	err = newElemental.CopyRecovery()
	if err != nil {
		return err
	}
	// install Passive
	err = newElemental.CopyPassive()
	if err != nil {
		return err
	}

	err = installHook(config, cnst.AfterInstallHook, false)
	if err != nil {
		return err
	}

	// installation rebrand (only grub for now)
	err = newElemental.Rebrand()
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
