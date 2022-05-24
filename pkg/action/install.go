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
	"fmt"

	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/elemental"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
)

func (i *InstallAction) installHook(hook string, chroot bool) error {
	if chroot {
		extraMounts := map[string]string{}
		persistent := i.spec.Partitions.Persistent
		if persistent != nil && persistent.MountPoint != "" {
			extraMounts[persistent.MountPoint] = cnst.UsrLocalPath
		}
		oem := i.spec.Partitions.OEM
		if oem != nil && oem.MountPoint != "" {
			extraMounts[oem.MountPoint] = cnst.OEMPath
		}
		return ChrootHook(&i.cfg.Config, hook, i.cfg.Strict, i.spec.Active.MountPoint, extraMounts, i.cfg.CloudInitPaths...)
	}
	return Hook(&i.cfg.Config, hook, i.cfg.Strict, i.cfg.CloudInitPaths...)
}

type InstallAction struct {
	cfg  *v1.RunConfig
	spec *v1.InstallSpec
}

func NewInstallAction(cfg *v1.RunConfig, spec *v1.InstallSpec) *InstallAction {
	return &InstallAction{cfg: cfg, spec: spec}
}

// InstallRun will install the system from a given configuration
func (i InstallAction) Run() (err error) {
	e := elemental.NewElemental(&i.cfg.Config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	err = i.installHook(cnst.BeforeInstallHook, false)
	if err != nil {
		return err
	}

	// Set installation sources from a downloaded ISO
	if i.spec.Iso != "" {
		tmpDir, err := e.GetIso(i.spec.Iso)
		if err != nil {
			return err
		}
		cleanup.Push(func() error { return i.cfg.Fs.RemoveAll(tmpDir) })
		err = e.UpdateSourcesFormDownloadedISO(tmpDir, &i.spec.Active, &i.spec.Recovery)
		if err != nil {
			return err
		}
	}

	// Check no-format flag
	if i.spec.NoFormat {
		// Check force flag against current device
		labels := []string{i.spec.Active.Label, i.spec.Recovery.Label}
		if e.CheckActiveDeployment(labels) && !i.spec.Force {
			return fmt.Errorf("use `force` flag to run an installation over the current running deployment")
		}
	} else {
		// Deactivate any active volume on target
		err = e.DeactivateDevices()
		if err != nil {
			return err
		}
		// Partition device
		err = e.PartitionAndFormatDevice(i.spec)
		if err != nil {
			return err
		}
	}

	err = e.MountPartitions(i.spec.Partitions.PartitionsByMountPoint(false))
	if err != nil {
		return err
	}
	cleanup.Push(func() error {
		return e.UnmountPartitions(i.spec.Partitions.PartitionsByMountPoint(true))
	})

	// Deploy active image
	err = e.DeployImage(&i.spec.Active, true)
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return e.UnmountImage(&i.spec.Active) })

	// Copy cloud-init if any
	err = e.CopyCloudConfig(i.spec.CloudInit)
	if err != nil {
		return err
	}
	// Install grub
	grub := utils.NewGrub(&i.cfg.Config)
	err = grub.Install(
		i.spec.Target,
		i.spec.Active.MountPoint,
		i.spec.Partitions.State.MountPoint,
		i.spec.GrubConf,
		i.spec.Tty,
		i.spec.Firmware == v1.EFI,
	)
	if err != nil {
		return err
	}
	// Relabel SELinux
	_ = e.SelinuxRelabel(cnst.ActiveDir, false)

	err = i.installHook(cnst.AfterInstallChrootHook, true)
	if err != nil {
		return err
	}

	// Installation rebrand (only grub for now)
	err = e.SetDefaultGrubEntry(
		i.spec.Partitions.State.MountPoint,
		i.spec.Active.MountPoint,
		i.spec.GrubDefEntry,
	)
	if err != nil {
		return err
	}

	// Unmount active image
	err = e.UnmountImage(&i.spec.Active)
	if err != nil {
		return err
	}
	// Install Recovery
	err = e.DeployImage(&i.spec.Recovery, false)
	if err != nil {
		return err
	}
	// Install Passive
	err = e.DeployImage(&i.spec.Passive, false)
	if err != nil {
		return err
	}

	err = i.installHook(cnst.AfterInstallHook, false)
	if err != nil {
		return err
	}

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return err
	}

	// If we want to eject the cd, create the required executable so the cd is ejected at shutdown
	if i.cfg.EjectCD && utils.BootedFrom(i.cfg.Runner, "cdroot") {
		i.cfg.Logger.Infof("Writing eject script")
		err = i.cfg.Fs.WriteFile("/usr/lib/systemd/system-shutdown/eject", []byte(cnst.EjectScript), 0744)
		if err != nil {
			i.cfg.Logger.Warnf("Could not write eject script, cdrom wont be ejected automatically: %s", err)
		}
	}

	// Reboot, poweroff or nothing
	if i.cfg.Reboot {
		i.cfg.Logger.Infof("Rebooting in 5 seconds")
		return utils.Reboot(i.cfg.Runner, 5)
	} else if i.cfg.PowerOff {
		i.cfg.Logger.Infof("Shutting down in 5 seconds")
		return utils.Shutdown(i.cfg.Runner, 5)
	}
	return err
}
