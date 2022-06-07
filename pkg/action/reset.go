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
	cnst "github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/elemental"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

func (r *ResetAction) resetHook(hook string, chroot bool) error {
	if chroot {
		extraMounts := map[string]string{}
		persistent := r.spec.Partitions.Persistent
		if persistent != nil && persistent.MountPoint != "" {
			extraMounts[persistent.MountPoint] = cnst.UsrLocalPath
		}
		oem := r.spec.Partitions.OEM
		if oem != nil && oem.MountPoint != "" {
			extraMounts[oem.MountPoint] = cnst.OEMPath
		}
		return ChrootHook(&r.cfg.Config, hook, r.cfg.Strict, r.spec.Active.MountPoint, extraMounts, r.cfg.CloudInitPaths...)
	}
	return Hook(&r.cfg.Config, hook, r.cfg.Strict, r.cfg.CloudInitPaths...)
}

type ResetAction struct {
	cfg  *v1.RunConfig
	spec *v1.ResetSpec
}

func NewResetAction(cfg *v1.RunConfig, spec *v1.ResetSpec) *ResetAction {
	return &ResetAction{cfg: cfg, spec: spec}
}

// ResetRun will reset the cos system to by following several steps
func (r ResetAction) Run() (err error) {
	e := elemental.NewElemental(&r.cfg.Config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	err = r.resetHook(cnst.BeforeResetHook, false)
	if err != nil {
		return err
	}

	// Unmount partitions if any is already mounted before formatting
	err = e.UnmountPartitions(r.spec.Partitions.PartitionsByMountPoint(true))
	if err != nil {
		return err
	}

	// Reformat state partition
	err = e.FormatPartition(r.spec.Partitions.State)
	if err != nil {
		return err
	}

	// Reformat persistent partitions
	if r.spec.FormatPersistent {
		persistent := r.spec.Partitions.Persistent
		if persistent != nil {
			err = e.FormatPartition(persistent)
			if err != nil {
				return err
			}
		}
		oem := r.spec.Partitions.OEM
		if oem != nil {
			err = e.FormatPartition(oem)
			if err != nil {
				return err
			}
		}
	}

	// Mount configured partitions
	err = e.MountPartitions(r.spec.Partitions.PartitionsByMountPoint(false))
	if err != nil {
		return err
	}
	cleanup.Push(func() error {
		return e.UnmountPartitions(r.spec.Partitions.PartitionsByMountPoint(true))
	})

	// Deploy active image
	err = e.DeployImage(&r.spec.Active, true)
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return e.UnmountImage(&r.spec.Active) })

	// install grub
	grub := utils.NewGrub(&r.cfg.Config)
	err = grub.Install(
		r.spec.Target,
		r.spec.Active.MountPoint,
		r.spec.Partitions.State.MountPoint,
		r.spec.GrubConf,
		r.spec.Tty,
		r.spec.Efi,
	)
	if err != nil {
		return err
	}

	// Relabel SELinux
	// TODO probably relabelling persistent volumes should be an opt in feature, it could
	// have undesired effects in case of failures
	binds := map[string]string{}
	if mnt, _ := utils.IsMounted(&r.cfg.Config, r.spec.Partitions.Persistent); mnt {
		binds[r.spec.Partitions.Persistent.MountPoint] = cnst.UsrLocalPath
	}
	if mnt, _ := utils.IsMounted(&r.cfg.Config, r.spec.Partitions.OEM); mnt {
		binds[r.spec.Partitions.OEM.MountPoint] = cnst.OEMPath
	}
	err = utils.ChrootedCallback(
		&r.cfg.Config, r.spec.Active.MountPoint, binds,
		func() error { return e.SelinuxRelabel("/", true) },
	)
	if err != nil {
		return err
	}

	err = r.resetHook(cnst.AfterResetChrootHook, true)
	if err != nil {
		return err
	}

	// installation rebrand (only grub for now)
	err = e.SetDefaultGrubEntry(
		r.spec.Partitions.State.MountPoint,
		r.spec.Active.MountPoint,
		r.spec.GrubDefEntry,
	)
	if err != nil {
		return err
	}

	// Unmount active image
	err = e.UnmountImage(&r.spec.Active)
	if err != nil {
		return err
	}

	// Install Passive
	err = e.DeployImage(&r.spec.Passive, false)
	if err != nil {
		return err
	}

	err = r.resetHook(cnst.AfterResetHook, false)
	if err != nil {
		return err
	}

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return err
	}

	// Reboot, poweroff or nothing
	if r.cfg.Reboot {
		r.cfg.Logger.Infof("Rebooting in 5 seconds")
		return utils.Reboot(r.cfg.Runner, 5)
	} else if r.cfg.PowerOff {
		r.cfg.Logger.Infof("Shutting down in 5 seconds")
		return utils.Shutdown(r.cfg.Runner, 5)
	}
	return err
}
