/*
Copyright © 2022 - 2025 SUSE LLC

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

	"github.com/rancher/elemental-toolkit/v2/pkg/bootloader"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/snapshotter"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

// resetHook runs the given hook without chroot. Moreover if the hook is 'after-reset'
// it appends defined cloud init paths rooted to the deployed root. This way any
// 'after-reset' hook provided by the deployed system image is also taken into account.
func (r *ResetAction) resetHook(hook string) error {
	cIPaths := r.cfg.CloudInitPaths
	if hook == constants.AfterResetHook {
		cIPaths = append(cIPaths, utils.PreAppendRoot(constants.WorkingImgDir, r.cfg.CloudInitPaths...)...)
	}
	return Hook(&r.cfg.Config, hook, r.cfg.Strict, cIPaths...)
}

func (r *ResetAction) resetChrootHook(hook string, root string) error {
	extraMounts := map[string]string{}
	persistent := r.spec.Partitions.Persistent
	if persistent != nil && persistent.MountPoint != "" {
		extraMounts[persistent.MountPoint] = constants.PersistentPath
	}
	oem := r.spec.Partitions.OEM
	if oem != nil && oem.MountPoint != "" {
		extraMounts[oem.MountPoint] = constants.OEMPath
	}
	efi := r.spec.Partitions.Boot
	if efi != nil && efi.MountPoint != "" {
		extraMounts[efi.MountPoint] = constants.BootDir
	}
	return ChrootHook(&r.cfg.Config, hook, r.cfg.Strict, root, extraMounts, r.cfg.CloudInitPaths...)
}

type ResetActionOption func(r *ResetAction) error

func WithResetBootloader(bootloader types.Bootloader) func(r *ResetAction) error {
	return func(i *ResetAction) error {
		i.bootloader = bootloader
		return nil
	}
}

type ResetAction struct {
	cfg         *types.RunConfig
	spec        *types.ResetSpec
	bootloader  types.Bootloader
	snapshotter types.Snapshotter
	snapshot    *types.Snapshot
}

func NewResetAction(cfg *types.RunConfig, spec *types.ResetSpec, opts ...ResetActionOption) (*ResetAction, error) {
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
			&cfg.Config,
			bootloader.WithGrubDisableBootEntry(r.spec.DisableBootEntry),
			bootloader.WithGrubAutoDisableBootEntry(),
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

	date := time.Now().Format(time.RFC3339)

	installState := &types.InstallState{
		Date:        date,
		Snapshotter: r.cfg.Snapshotter,
		Partitions: map[string]*types.PartitionState{
			constants.StatePartName: {
				FSLabel: r.spec.Partitions.State.FilesystemLabel,
				Snapshots: map[int]*types.SystemState{
					r.snapshot.ID: {
						Source:     src,
						Digest:     src.GetDigest(),
						Active:     true,
						Labels:     r.spec.SnapshotLabels,
						Date:       date,
						FromAction: constants.ActionReset,
					},
				},
			},
		},
	}
	if r.spec.Partitions.OEM != nil {
		installState.Partitions[constants.OEMPartName] = &types.PartitionState{
			FSLabel: r.spec.Partitions.OEM.FilesystemLabel,
		}
	}
	if r.spec.Partitions.Persistent != nil {
		installState.Partitions[constants.PersistentPartName] = &types.PartitionState{
			FSLabel: r.spec.Partitions.Persistent.FilesystemLabel,
		}
	}
	if r.spec.State != nil && r.spec.State.Partitions != nil {
		installState.Partitions[constants.RecoveryPartName] = r.spec.State.Partitions[constants.RecoveryPartName]
	}

	umount, err := elemental.MountRWPartition(r.cfg.Config, r.spec.Partitions.Recovery)
	if err != nil {
		return err
	}
	cleanup.Push(umount)

	return r.cfg.WriteInstallState(
		installState,
		filepath.Join(r.spec.Partitions.State.MountPoint, constants.InstallStateFile),
		filepath.Join(r.spec.Partitions.Recovery.MountPoint, constants.InstallStateFile),
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
	err = r.snapshotter.InitSnapshotter(r.spec.Partitions.State, r.spec.Partitions.Boot.MountPoint)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SnapshotterInit)
	}

	// Before reset hook happens once partitions are aready and before deploying the OS image
	err = r.resetHook(constants.BeforeResetHook)
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
	err = elemental.MirrorRoot(r.cfg.Config, r.snapshot.WorkDir, r.spec.System)
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

	err = r.resetHook(constants.PostResetHook)
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
		r.spec.Partitions.Boot.MountPoint,
	)
	if err != nil {
		r.cfg.Logger.Errorf("failed installing grub: %v", err)
		return elementalError.NewFromError(err, elementalError.InstallGrub)
	}

	err = r.resetChrootHook(constants.AfterResetChrootHook, constants.WorkingImgDir)
	if err != nil {
		r.cfg.Logger.Errorf("failed after-reset-chroot hook: %v", err)
		return elementalError.NewFromError(err, elementalError.HookAfterResetChroot)
	}
	err = r.resetHook(constants.AfterResetHook)
	if err != nil {
		r.cfg.Logger.Errorf("failed after-reset hook: %v", err)
		return elementalError.NewFromError(err, elementalError.HookAfterReset)
	}

	grubVars := r.spec.GetGrubLabels()
	err = r.bootloader.SetPersistentVariables(
		filepath.Join(r.spec.Partitions.Boot.MountPoint, constants.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		r.cfg.Logger.Error("Error setting GRUB labels: %s", err)
		return elementalError.NewFromError(err, elementalError.SetGrubVariables)
	}

	// Installation rebrand (only grub for now)
	err = r.bootloader.SetDefaultEntry(
		r.spec.Partitions.Boot.MountPoint,
		constants.WorkingImgDir,
		r.spec.GrubDefEntry,
	)
	if err != nil {
		r.cfg.Logger.Errorf("failed setting defaut GRUB entry: %v", err)
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}
	return nil
}
