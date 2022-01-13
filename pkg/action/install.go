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
	cnst "github.com/rancher-sandbox/elemental-cli/pkg/constants"
	"github.com/rancher-sandbox/elemental-cli/pkg/elemental"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
)

// InstallAction represents the struct that will run the full install from start to finish
type InstallAction struct {
	Config *v1.RunConfig
}

func NewInstallAction(config *v1.RunConfig) *InstallAction {
	return &InstallAction{Config: config}
}

func (i InstallAction) installHook(hook string, chroot bool) error {
	var out []byte
	var err error
	if chroot {
		chroot := utils.NewChroot(i.Config.ActiveImage.MountPoint, i.Config)
		chroot.SetExtraMounts(map[string]string{
			cnst.PersistentDir: "/usr/local",
			cnst.OEMDir:        "/oem",
		})
		out, err = chroot.Run(cnst.CosSetup, hook)
	} else {
		i.Config.Logger.Infof("Running %s hook", hook)
		out, err = i.Config.Runner.Run(cnst.CosSetup, hook)
	}
	i.Config.Logger.Debugf("%s output: %s", hook, string(out))
	if err != nil && i.Config.Strict {
		return err
	}
	return nil
}

// Run will install the cos system to a device by following several steps
func (i InstallAction) Run() (err error) {
	newElemental := elemental.NewElemental(i.Config)

	disk := part.NewDisk(
		i.Config.Target,
		part.WithRunner(i.Config.Runner),
		part.WithFS(i.Config.Fs),
		part.WithLogger(i.Config.Logger),
	)

	err = i.installHook(cnst.BeforeInstallHook, false)
	if err != nil {
		return err
	}

	if i.Config.Iso != "" {
		tmpDir, err := newElemental.GetIso()
		if err != nil {
			return err
		}
		defer func() {
			i.Config.Logger.Infof("Unmounting downloaded ISO")
			if tmpErr := i.Config.Mounter.Unmount(i.Config.IsoMnt); tmpErr != nil && err == nil {
				err = tmpErr
			}
			i.Config.Fs.RemoveAll(tmpDir)
		}()
	}

	// Check device valid
	if !disk.Exists() {
		i.Config.Logger.Errorf("Disk %s does not exist", i.Config.Target)
		return errors.New(fmt.Sprintf("Disk %s does not exist", i.Config.Target))
	}

	// Check no-format flag
	if i.Config.NoFormat {
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
		i.Config.Logger.Infof("Unmounting partitions")
		if tmpErr := newElemental.UnmountPartitions(); tmpErr != nil && err == nil {
			err = tmpErr
		}
	}()

	// create active file system image
	err = newElemental.CreateFileSystemImage(i.Config.ActiveImage)
	if err != nil {
		return err
	}

	//mount file system image
	err = newElemental.MountImage(&i.Config.ActiveImage)
	if err != nil {
		return err
	}
	defer func() {
		i.Config.Logger.Infof("Unmounting Active image")
		if tmpErr := newElemental.UnmountImage(&i.Config.ActiveImage); tmpErr != nil && err == nil {
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
	grub := utils.NewGrub(i.Config)
	err = grub.Install()
	if err != nil {
		return err
	}
	// Relabel SELinux
	_ = newElemental.SelinuxRelabel(false)

	err = i.installHook(cnst.AfterInstallChrootHook, true)
	if err != nil {
		newElemental.UnmountImage(&i.Config.ActiveImage)
		return err
	}
	//TODO rebrand here is really needed? see cos-installer script

	// Unmount active image
	err = newElemental.UnmountImage(&i.Config.ActiveImage)
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

	err = i.installHook(cnst.AfterInstallHook, false)
	if err != nil {
		return err
	}
	// TODO Rebrand
	// cos.Rebrand()
	// ????
	// profit!
	// TODO poweroff or reboot or nothing
	return err
}
