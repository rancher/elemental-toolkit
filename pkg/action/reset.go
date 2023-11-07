/*
Copyright © 2022 - 2023 SUSE LLC

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
	"path/filepath"
	"time"

	"github.com/rancher/elemental-toolkit/pkg/bootloader"
	cnst "github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

func (r *ResetAction) resetHook(hook string) error {
	return Hook(&r.cfg.Config, hook, r.cfg.Strict, r.cfg.CloudInitPaths...)
}

func (r *ResetAction) resetChrootHook(hook string, root string) error {
	extraMounts := map[string]string{}
	persistent := r.spec.Partitions.Persistent
	if persistent != nil && persistent.MountPoint != "" {
		extraMounts[persistent.MountPoint] = cnst.UsrLocalPath
	}
	oem := r.spec.Partitions.OEM
	if oem != nil && oem.MountPoint != "" {
		extraMounts[oem.MountPoint] = cnst.OEMPath
	}
	return ChrootHook(&r.cfg.Config, hook, r.cfg.Strict, root, extraMounts, r.cfg.CloudInitPaths...)
}

type ResetActionOption func(r *ResetAction) error

func WithResetBootloader(bootloader v1.Bootloader) func(r *ResetAction) error {
	return func(i *ResetAction) error {
		i.bootloader = bootloader
		return nil
	}
}

type ResetAction struct {
	cfg        *v1.RunConfig
	spec       *v1.ResetSpec
	bootloader v1.Bootloader
}

func NewResetAction(cfg *v1.RunConfig, spec *v1.ResetSpec, opts ...ResetActionOption) *ResetAction {
	r := &ResetAction{cfg: cfg, spec: spec}

	for _, o := range opts {
		err := o(r)
		if err != nil {
			cfg.Logger.Errorf("error applying config option: %s", err.Error())
			return nil
		}
	}

	if r.bootloader == nil {
		r.bootloader = bootloader.NewGrub(
			&cfg.Config, bootloader.WithGrubDisableBootEntry(r.spec.DisableBootEntry),
			bootloader.WithGrubClearBootEntry(false),
		)
	}

	return r
}

func (r *ResetAction) updateInstallState(e *elemental.Elemental, cleanup *utils.CleanStack, meta interface{}) error {
	if r.spec.Partitions.Recovery == nil || r.spec.Partitions.State == nil {
		return fmt.Errorf("undefined state or recovery partition")
	}

	installState := &v1.InstallState{
		Date: time.Now().Format(time.RFC3339),
		Partitions: map[string]*v1.PartitionState{
			cnst.StatePartName: {
				FSLabel: r.spec.Partitions.State.FilesystemLabel,
				Images: map[string]*v1.ImageState{
					cnst.ActiveImgName: {
						Source:         r.spec.Active.Source,
						SourceMetadata: meta,
						Label:          r.spec.Active.Label,
						FS:             r.spec.Active.FS,
					},
					cnst.PassiveImgName: {
						Source:         r.spec.Active.Source,
						SourceMetadata: meta,
						Label:          r.spec.Passive.Label,
						FS:             r.spec.Passive.FS,
					},
				},
			},
		},
	}
	if r.spec.Partitions.OEM != nil {
		installState.Partitions[cnst.OEMPartName] = &v1.PartitionState{
			FSLabel: r.spec.Partitions.OEM.FilesystemLabel,
		}
	}
	if r.spec.Partitions.Persistent != nil {
		installState.Partitions[cnst.PersistentPartName] = &v1.PartitionState{
			FSLabel: r.spec.Partitions.Persistent.FilesystemLabel,
		}
	}
	if r.spec.State != nil && r.spec.State.Partitions != nil {
		installState.Partitions[cnst.RecoveryPartName] = r.spec.State.Partitions[cnst.RecoveryPartName]
	}

	umount, err := e.MountRWPartition(r.spec.Partitions.Recovery)
	if err != nil {
		return err
	}
	cleanup.Push(umount)

	return r.cfg.WriteInstallState(
		installState,
		filepath.Join(r.spec.Partitions.State.MountPoint, cnst.InstallStateFile),
		filepath.Join(r.spec.Partitions.Recovery.MountPoint, cnst.InstallStateFile),
	)
}

// ResetRun will reset the cos system to by following several steps
func (r ResetAction) Run() (err error) {
	e := elemental.NewElemental(&r.cfg.Config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	// Unmount partitions if any is already mounted before formatting
	err = e.UnmountPartitions(r.spec.Partitions.PartitionsByMountPoint(true, r.spec.Partitions.Recovery))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.UnmountPartitions)
	}

	// Reformat state partition
	err = e.FormatPartition(r.spec.Partitions.State)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.FormatPartitions)
	}

	// Reformat persistent partition
	if r.spec.FormatPersistent {
		persistent := r.spec.Partitions.Persistent
		if persistent != nil {
			err = e.FormatPartition(persistent)
			if err != nil {
				return elementalError.NewFromError(err, elementalError.FormatPartitions)
			}
		}
	}

	// Reformat OEM
	if r.spec.FormatOEM {
		oem := r.spec.Partitions.OEM
		if oem != nil {
			err = e.FormatPartition(oem)
			if err != nil {
				return elementalError.NewFromError(err, elementalError.FormatPartitions)
			}
		}
	}
	// Mount configured partitions
	err = e.MountPartitions(r.spec.Partitions.PartitionsByMountPoint(false, r.spec.Partitions.Recovery))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountPartitions)
	}
	cleanup.Push(func() error {
		return e.UnmountPartitions(r.spec.Partitions.PartitionsByMountPoint(true, r.spec.Partitions.Recovery))
	})

	// Before reset hook happens once partitions are aready and before deploying the OS image
	err = r.resetHook(cnst.BeforeResetHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookBeforeReset)
	}

	// Deploy active image
	meta, treeCleaner, err := e.DeployImgTree(&r.spec.Active, cnst.WorkingImgDir)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.DeployImgTree)
	}
	cleanup.Push(func() error { return treeCleaner() })

	// Copy cloud-init if any
	err = e.CopyCloudConfig(r.spec.Partitions.GetConfigStorage(), r.spec.CloudInit)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}

	// install grub
	err = r.bootloader.Install(
		cnst.WorkingImgDir,
		r.spec.Partitions.State.MountPoint,
		r.spec.Partitions.State.FilesystemLabel,
	)

	if err != nil {
		return elementalError.NewFromError(err, elementalError.InstallGrub)
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
		&r.cfg.Config, cnst.WorkingImgDir, binds,
		func() error { return e.SelinuxRelabel("/", true) },
	)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SelinuxRelabel)
	}

	err = r.resetChrootHook(cnst.AfterResetChrootHook, cnst.WorkingImgDir)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookAfterResetChroot)
	}
	err = r.resetHook(cnst.AfterResetHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookAfterReset)
	}

	grubVars := r.spec.GetGrubLabels()
	err = r.bootloader.SetPersistentVariables(
		filepath.Join(r.spec.Partitions.State.MountPoint, cnst.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		r.cfg.Logger.Error("Error setting GRUB labels: %s", err)
		return elementalError.NewFromError(err, elementalError.SetGrubVariables)
	}

	// installation rebrand (only grub for now)
	err = r.bootloader.SetDefaultEntry(
		r.spec.Partitions.State.MountPoint,
		cnst.WorkingImgDir,
		r.spec.GrubDefEntry,
	)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}

	err = e.CreateImgFromTree(cnst.WorkingImgDir, &r.spec.Active, false, treeCleaner)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateImgFromTree)
	}

	// Install Passive
	err = e.CopyFileImg(&r.spec.Passive)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.DeployImage)
	}

	err = r.resetHook(cnst.PostResetHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookPostReset)
	}

	err = r.updateInstallState(e, cleanup, meta)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateFile)
	}

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.Cleanup)
	}

	return PowerAction(r.cfg)
}
