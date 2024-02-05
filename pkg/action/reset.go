/*
Copyright © 2022 - 2024 SUSE LLC

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
	"strings"
	"time"

	"github.com/rancher/elemental-toolkit/pkg/bootloader"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	cnst "github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	"github.com/rancher/elemental-toolkit/pkg/snapshotter"
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
		extraMounts[persistent.MountPoint] = cnst.PersistentPath
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
	cfg         *v1.RunConfig
	spec        *v1.ResetSpec
	bootloader  v1.Bootloader
	snapshotter v1.Snapshotter
	snapshot    *v1.Snapshot
}

func NewResetAction(cfg *v1.RunConfig, spec *v1.ResetSpec, opts ...ResetActionOption) (*ResetAction, error) {
	var err error

	r := &ResetAction{cfg: cfg, spec: spec}

	for _, o := range opts {
		err = o(r)
		if err != nil {
			cfg.Logger.Errorf("error applying config option: %s", err.Error())
			return nil, err
		}
	}

	if r.bootloader == nil {
		r.bootloader = bootloader.NewGrub(
			&cfg.Config, bootloader.WithGrubDisableBootEntry(r.spec.DisableBootEntry),
			bootloader.WithGrubClearBootEntry(false),
		)
	}

	if r.snapshotter == nil {
		r.snapshotter, err = snapshotter.NewSnapshotter(cfg.Config, cfg.Snapshotter, r.bootloader)
		if err != nil {
			cfg.Logger.Errorf("error initializing snapshotter of type '%s'", cfg.Snapshotter.Type)
			return nil, err
		}
	}

	if r.cfg.Snapshotter.Type == constants.BtrfsSnapshotterType {
		if spec.Partitions.State.FS != constants.Btrfs {
			cfg.Logger.Warning("Btrfs snapshotter type, forcing btrfs filesystem on state partition")
			spec.Partitions.State.FS = constants.Btrfs
		}
	}

	return r, nil
}

func (r *ResetAction) updateInstallState(cleanup *utils.CleanStack) error {
	if r.spec.Partitions.Recovery == nil || r.spec.Partitions.State == nil {
		return fmt.Errorf("undefined state or recovery partition")
	}

	if r.snapshot == nil {
		return fmt.Errorf("undefined reset snapshot")
	}

	// Reuse recovery source and digest if system points to recovery
	src := r.spec.System
	if src.IsFile() && strings.HasSuffix(src.Value(), constants.RecoveryImgFile) {
		if r.spec.State != nil && r.spec.State.Partitions[constants.RecoveryPartName] != nil &&
			r.spec.State.Partitions[constants.RecoveryPartName].RecoveryImage != nil {
			src = r.spec.State.Partitions[constants.RecoveryPartName].RecoveryImage.Source
			src.SetDigest(r.spec.State.Partitions[constants.RecoveryPartName].RecoveryImage.Digest)
		}
	}

	installState := &v1.InstallState{
		Date:        time.Now().Format(time.RFC3339),
		Snapshotter: r.cfg.Snapshotter,
		Partitions: map[string]*v1.PartitionState{
			cnst.StatePartName: {
				FSLabel: r.spec.Partitions.State.FilesystemLabel,
				Snapshots: map[int]*v1.SystemState{
					r.snapshot.ID: {
						Source: src,
						Digest: src.GetDigest(),
						Active: true,
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

	umount, err := elemental.MountRWPartition(r.cfg.Config, r.spec.Partitions.Recovery)
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
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	// Unmount partitions if any is already mounted before formatting
	err = elemental.UnmountPartitions(r.cfg.Config, r.spec.Partitions.PartitionsByMountPoint(true, r.spec.Partitions.Recovery))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.UnmountPartitions)
	}

	// Reformat state partition
	err = elemental.FormatPartition(r.cfg.Config, r.spec.Partitions.State)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.FormatPartitions)
	}

	// Reformat persistent partition
	if r.spec.FormatPersistent {
		persistent := r.spec.Partitions.Persistent
		if persistent != nil {
			err = elemental.FormatPartition(r.cfg.Config, persistent)
			if err != nil {
				return elementalError.NewFromError(err, elementalError.FormatPartitions)
			}
		}
	}

	// Reformat OEM
	if r.spec.FormatOEM {
		oem := r.spec.Partitions.OEM
		if oem != nil {
			err = elemental.FormatPartition(r.cfg.Config, oem)
			if err != nil {
				return elementalError.NewFromError(err, elementalError.FormatPartitions)
			}
		}
	}
	// Mount configured partitions
	err = elemental.MountPartitions(r.cfg.Config, r.spec.Partitions.PartitionsByMountPoint(false, r.spec.Partitions.Recovery), "rw")
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountPartitions)
	}
	cleanup.Push(func() error {
		return elemental.UnmountPartitions(r.cfg.Config, r.spec.Partitions.PartitionsByMountPoint(true, r.spec.Partitions.Recovery))
	})

	// Init snapshotter
	err = r.snapshotter.InitSnapshotter(r.spec.Partitions.State.MountPoint)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SnapshotterInit)
	}

	// Before reset hook happens once partitions are aready and before deploying the OS image
	err = r.resetHook(cnst.BeforeResetHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookBeforeReset)
	}

	// Starting snapshotter transaction
	r.cfg.Logger.Info("Starting snapshotter transaction")
	r.snapshot, err = r.snapshotter.StartTransaction()
	if err != nil {
		r.cfg.Logger.Errorf("failed to start snapshotter transaction")
		return elementalError.NewFromError(err, elementalError.SnapshotterStart)
	}
	cleanup.PushErrorOnly(func() error { return r.snapshotter.CloseTransactionOnError(r.snapshot) })

	// Deploy system image
	err = elemental.DumpSource(r.cfg.Config, r.snapshot.WorkDir, r.spec.System)
	if err != nil {
		r.cfg.Logger.Errorf("failed deploying source: %s", r.spec.System.String())
		return elementalError.NewFromError(err, elementalError.DumpSource)
	}

	// Fine tune the dumped tree
	r.cfg.Logger.Info("Fine tune the dumped root tree")
	err = r.refineDeployment()
	if err != nil {
		r.cfg.Logger.Error("failed refining system root tree")
		return err
	}

	// Closing snapshotter transaction
	r.cfg.Logger.Info("Closing snapshotter transaction")
	err = r.snapshotter.CloseTransaction(r.snapshot)
	if err != nil {
		r.cfg.Logger.Errorf("failed closing snapshot transaction: %v", err)
		return err
	}

	err = r.resetHook(cnst.PostResetHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookPostReset)
	}

	err = r.updateInstallState(cleanup)
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

func (r *ResetAction) refineDeployment() error { //nolint:dupl
	// Copy cloud-init if any
	err := elemental.CopyCloudConfig(r.cfg.Config, r.spec.Partitions.GetConfigStorage(), r.spec.CloudInit)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}
	// Install grub
	err = r.bootloader.Install(
		r.snapshot.WorkDir,
		r.spec.Partitions.EFI.MountPoint,
	)
	if err != nil {
		r.cfg.Logger.Errorf("failed installing grub: %v", err)
		return elementalError.NewFromError(err, elementalError.InstallGrub)
	}

	// Relabel SELinux
	err = elemental.ApplySelinuxLabels(r.cfg.Config, r.spec.Partitions)
	if err != nil {
		r.cfg.Logger.Errorf("failed setting SELinux labels: %v", err)
		return elementalError.NewFromError(err, elementalError.SelinuxRelabel)
	}

	err = r.resetChrootHook(cnst.AfterResetChrootHook, cnst.WorkingImgDir)
	if err != nil {
		r.cfg.Logger.Errorf("failed after-reset-chroot hook: %v", err)
		return elementalError.NewFromError(err, elementalError.HookAfterResetChroot)
	}
	err = r.resetHook(cnst.AfterResetHook)
	if err != nil {
		r.cfg.Logger.Errorf("failed after-reset hook: %v", err)
		return elementalError.NewFromError(err, elementalError.HookAfterReset)
	}

	grubVars := r.spec.GetGrubLabels()
	err = r.bootloader.SetPersistentVariables(
		filepath.Join(r.spec.Partitions.EFI.MountPoint, cnst.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		r.cfg.Logger.Error("Error setting GRUB labels: %s", err)
		return elementalError.NewFromError(err, elementalError.SetGrubVariables)
	}

	// Installation rebrand (only grub for now)
	err = r.bootloader.SetDefaultEntry(
		r.spec.Partitions.EFI.MountPoint,
		cnst.WorkingImgDir,
		r.spec.GrubDefEntry,
	)
	if err != nil {
		r.cfg.Logger.Errorf("failed setting defaut GRUB entry: %v", err)
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}
	return nil
}
