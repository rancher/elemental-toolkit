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
		extraMounts := map[string]string{}
		persistent := config.Partitions.GetByName(cnst.PersistentPartName)
		if persistent != nil {
			extraMounts[persistent.MountPoint] = "/usr/local"
		}
		oem := config.Partitions.GetByName(cnst.OEMPartName)
		if oem != nil {
			extraMounts[oem.MountPoint] = "/oem"
		}
		return ActionChrootHook(config, hook, config.ActiveImage.MountPoint, extraMounts)
	}
	return ActionHook(config, hook)
}

// InstallSetup will set installation parameters according to
// the given configuration flags
func InstallSetup(config *v1.RunConfig) error {
	SetPartitionsFromScratch(config)
	SetupLuet(config)

	config.ActiveImage = v1.Image{
		Label:      config.ActiveLabel,
		Size:       cnst.ImgSize,
		File:       filepath.Join(cnst.StateDir, "cOS", cnst.ActiveImgFile),
		FS:         cnst.LinuxImgFs,
		MountPoint: cnst.ActiveDir,
	}

	//TODO add installation from channel
	if config.DockerImg != "" {
		config.ActiveImage.Source.Source = config.DockerImg
		config.ActiveImage.Source.IsDocker = true
	} else if config.Directory != "" {
		config.ActiveImage.Source.Source = config.Directory
		config.ActiveImage.Source.IsDir = true
	} else {
		config.ActiveImage.Source.Source = cnst.IsoBaseTree
		config.ActiveImage.Source.IsDir = true
	}

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

	if config.Iso != "" {
		tmpDir, err := newElemental.GetIso()
		if err != nil {
			return err
		}
		cleanup.Push(func() error {
			config.Logger.Infof("Unmounting downloaded ISO")
			config.Fs.RemoveAll(tmpDir)
			return config.Mounter.Unmount(config.IsoMnt)
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
	err = newElemental.DeployImage(&config.ActiveImage, true)
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return newElemental.UnmountImage(&config.ActiveImage) })

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
