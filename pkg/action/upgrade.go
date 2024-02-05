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
	"slices"
	"time"

	"github.com/rancher/elemental-toolkit/pkg/bootloader"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	"github.com/rancher/elemental-toolkit/pkg/snapshotter"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

// UpgradeAction represents the struct that will run the upgrade from start to finish
type UpgradeAction struct {
	cfg         *v1.RunConfig
	spec        *v1.UpgradeSpec
	bootloader  v1.Bootloader
	snapshotter v1.Snapshotter
	snapshot    *v1.Snapshot
}

type UpgradeActionOption func(r *UpgradeAction) error

func WithUpgradeBootloader(bootloader v1.Bootloader) func(u *UpgradeAction) error {
	return func(u *UpgradeAction) error {
		u.bootloader = bootloader
		return nil
	}
}

func NewUpgradeAction(config *v1.RunConfig, spec *v1.UpgradeSpec, opts ...UpgradeActionOption) (*UpgradeAction, error) {
	var err error

	u := &UpgradeAction{cfg: config, spec: spec}

	for _, o := range opts {
		err = o(u)
		if err != nil {
			config.Logger.Errorf("error applying config option: %s", err.Error())
			return nil, err
		}
	}

	if u.bootloader == nil {
		u.bootloader = bootloader.NewGrub(&config.Config, bootloader.WithGrubDisableBootEntry(true))
	}

	if u.snapshotter == nil {
		u.snapshotter, err = snapshotter.NewSnapshotter(config.Config, config.Snapshotter, u.bootloader)
		if err != nil {
			config.Logger.Errorf("error initializing snapshotter of type '%s'", config.Snapshotter.Type)
			return nil, err
		}
	}

	// Check the setup of previous snapshotter and requested one is consistent
	if spec.State != nil && spec.State.Snapshotter.Type != config.Snapshotter.Type {
		config.Logger.Errorf("can't change snaphsotter type on upgrades, not supported. Please review upgrade configuration")
		return nil, fmt.Errorf("failed setting snapshotter for the upgrade, unexpected type '%s'", config.Snapshotter.Type)
	}

	if u.spec.RecoveryUpgrade && elemental.IsRecoveryMode(config.Config) {
		config.Logger.Errorf("Upgrading recovery image from the recovery system itself is not supported")
		return nil, fmt.Errorf("Not supported")
	}

	return u, nil
}

func (u UpgradeAction) Info(s string, args ...interface{}) {
	u.cfg.Logger.Infof(s, args...)
}

func (u UpgradeAction) Debug(s string, args ...interface{}) {
	u.cfg.Logger.Debugf(s, args...)
}

func (u UpgradeAction) Error(s string, args ...interface{}) {
	u.cfg.Logger.Errorf(s, args...)
}

func (u UpgradeAction) upgradeHook(hook string) error {
	u.Info("Applying '%s' hook", hook)
	return Hook(&u.cfg.Config, hook, u.cfg.Strict, u.cfg.CloudInitPaths...)
}

func (u UpgradeAction) upgradeChrootHook(hook string, root string) error {
	u.Info("Applying '%s' hook", hook)
	mountPoints := map[string]string{}

	oemDevice := u.spec.Partitions.OEM
	if oemDevice != nil && oemDevice.MountPoint != "" {
		mountPoints[oemDevice.MountPoint] = constants.OEMPath
	}

	persistentDevice := u.spec.Partitions.Persistent
	if persistentDevice != nil && persistentDevice.MountPoint != "" {
		mountPoints[persistentDevice.MountPoint] = constants.PersistentPath
	}

	return ChrootHook(&u.cfg.Config, hook, u.cfg.Strict, root, mountPoints, u.cfg.CloudInitPaths...)
}

func (u *UpgradeAction) upgradeInstallStateYaml() error {
	var oldActiveID int
	var deletedIDs []int

	if u.spec.Partitions.Recovery == nil || u.spec.Partitions.State == nil {
		return fmt.Errorf("undefined state or recovery partition")
	}

	snapshots, err := u.snapshotter.GetSnapshots()
	if err != nil {
		u.Error("failed getting snapshots list")
		return err
	}

	if u.spec.State == nil {
		u.spec.State = &v1.InstallState{
			Partitions: map[string]*v1.PartitionState{},
		}
	}

	u.spec.State.Snapshotter = u.cfg.Snapshotter
	u.spec.State.Date = time.Now().Format(time.RFC3339)

	statePart := u.spec.State.Partitions[constants.StatePartName]
	if statePart == nil {
		statePart = &v1.PartitionState{
			FSLabel:   u.spec.Partitions.State.FilesystemLabel,
			Snapshots: map[int]*v1.SystemState{},
		}
	}

	if statePart.Snapshots == nil {
		statePart.Snapshots = map[int]*v1.SystemState{}
	}

	for id, state := range statePart.Snapshots {
		if state.Active {
			oldActiveID = id
		}
		if !slices.Contains(snapshots, id) {
			deletedIDs = append(deletedIDs, id)
		}
	}

	statePart.Snapshots[u.snapshot.ID] = &v1.SystemState{
		Source: u.spec.System,
		Digest: u.spec.System.GetDigest(),
		Active: true,
	}

	if statePart.Snapshots[oldActiveID] != nil {
		statePart.Snapshots[oldActiveID].Active = false
	}

	for _, id := range deletedIDs {
		delete(statePart.Snapshots, id)
	}

	u.spec.State.Partitions[constants.StatePartName] = statePart

	if u.spec.RecoveryUpgrade {
		recoveryPart := u.spec.State.Partitions[constants.RecoveryPartName]
		if recoveryPart == nil {
			recoveryPart = &v1.PartitionState{
				FSLabel: u.spec.Partitions.Recovery.FilesystemLabel,
				RecoveryImage: &v1.SystemState{
					FS:     u.spec.RecoverySystem.FS,
					Label:  u.spec.RecoverySystem.Label,
					Source: u.spec.RecoverySystem.Source,
					Digest: u.spec.RecoverySystem.Source.GetDigest(),
				},
			}
			u.spec.State.Partitions[constants.RecoveryPartName] = recoveryPart
		}
	}

	statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
	if u.spec.Partitions.Recovery.MountPoint == constants.RunningStateDir {
		statePath = filepath.Join(u.spec.Partitions.State.MountPoint, constants.InstallStateFile)
	}

	return u.cfg.WriteInstallState(
		u.spec.State, statePath,
		filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.InstallStateFile),
	)
}

func (u *UpgradeAction) mountRWPartitions(cleanup *utils.CleanStack) error {
	umount, err := elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.EFI)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountEFIPartition)
	}
	cleanup.Push(umount)

	if !elemental.IsRecoveryMode(u.cfg.Config) {
		umount, err = elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.Recovery)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.MountRecoveryPartition)
		}
		cleanup.Push(umount)
	} else {
		umount, err = elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.State)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.MountStatePartition)
		}
		cleanup.Push(umount)
	}

	if u.spec.Partitions.Persistent != nil {
		umount, err = elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.Persistent)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.MountPersistentPartition)
		}
		cleanup.Push(umount)
	}

	return nil
}

func (u *UpgradeAction) Run() (err error) {
	cleanup := utils.NewCleanStack()
	defer func() {
		err = cleanup.Cleanup(err)
	}()

	// Mount required partitions as RW
	err = u.mountRWPartitions(cleanup)
	if err != nil {
		return err
	}

	// Init snapshotter
	err = u.snapshotter.InitSnapshotter(u.spec.Partitions.State.MountPoint)
	if err != nil {
		u.cfg.Logger.Errorf("failed initializing snapshotter")
		return elementalError.NewFromError(err, elementalError.SnapshotterInit)
	}

	// Before upgrade hook happens once partitions are RW mounted, just before image OS is deployed
	err = u.upgradeHook(constants.BeforeUpgradeHook)
	if err != nil {
		u.Error("Error while running hook before-upgrade: %s", err)
		return elementalError.NewFromError(err, elementalError.HookBeforeUpgrade)
	}

	// Starting snapshotter transaction
	u.cfg.Logger.Info("Starting snapshotter transaction")
	u.snapshot, err = u.snapshotter.StartTransaction()
	if err != nil {
		u.cfg.Logger.Errorf("failed to start snapshotter transaction")
		return elementalError.NewFromError(err, elementalError.SnapshotterStart)
	}
	cleanup.PushErrorOnly(func() error { return u.snapshotter.CloseTransactionOnError(u.snapshot) })

	// Deploy system image
	err = elemental.DumpSource(u.cfg.Config, u.snapshot.WorkDir, u.spec.System)
	if err != nil {
		u.cfg.Logger.Errorf("failed deploying source: %s", u.spec.System.String())
		return elementalError.NewFromError(err, elementalError.DumpSource)
	}

	// Fine tune the dumped tree
	u.cfg.Logger.Info("Fine tune the dumped root tree")
	err = u.refineDeployment()
	if err != nil {
		u.cfg.Logger.Error("failed refining system root tree")
		return err
	}

	// Manage legacy recovery. This logic should be removed once toolkit v1.1 gets unsupported.
	legacyImg := filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.LegacyImagesPath, constants.RecoveryImgFile)
	if ok, _ := utils.Exists(u.cfg.Fs, legacyImg); ok {
		u.cfg.Logger.Debug("Manage legacy recovery image")
		recoveryImg := filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.RecoveryImgFile)
		err = u.cfg.Fs.Rename(legacyImg, recoveryImg)
		if err != nil {
			u.cfg.Logger.Error("failed renaming recovery image from legacy path")
			return err
		}
	}

	// Closing snapshotter transaction
	u.cfg.Logger.Info("Closing snapshotter transaction")
	err = u.snapshotter.CloseTransaction(u.snapshot)
	if err != nil {
		u.cfg.Logger.Errorf("failed closing snapshot transaction: %v", err)
		return err
	}

	// Upgrade recovery
	if u.spec.RecoveryUpgrade {
		recoverySystem := u.spec.RecoverySystem
		u.cfg.Logger.Info("Deploying recovery system")
		if recoverySystem.Source.String() == u.spec.System.String() {
			// Reuse already deployed root-tree from active snapshot
			recoverySystem.Source, err = u.snapshotter.SnapshotToImageSource(u.snapshot)
			if err != nil {
				return err
			}
			recoverySystem.Source.SetDigest(u.spec.System.GetDigest())
		}
		err = elemental.DeployImage(u.cfg.Config, &recoverySystem)
		if err != nil {
			u.cfg.Logger.Error("failed deploying recovery image")
			return elementalError.NewFromError(err, elementalError.DeployImage)
		}
		recoveryFile := filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.RecoveryImgFile)
		transitionFile := filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.TransitionImgFile)
		if ok, _ := utils.Exists(u.cfg.Fs, recoveryFile); ok {
			err = u.cfg.Fs.Remove(recoveryFile)
			if err != nil {
				u.Error("failed removing old recovery image")
				return err
			}
		}
		err = u.cfg.Fs.Rename(transitionFile, recoveryFile)
		if err != nil {
			u.Error("failed renaming transition recovery image")
			return err
		}
	}

	err = u.upgradeHook(constants.PostUpgradeHook)
	if err != nil {
		u.Error("Error running hook post-upgrade: %s", err)
		return elementalError.NewFromError(err, elementalError.HookPostUpgrade)
	}

	// Update state.yaml file on recovery and state partitions
	err = u.upgradeInstallStateYaml()
	if err != nil {
		u.Error("failed upgrading installation metadata")
		return err
	}

	u.Info("Upgrade completed")

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.Cleanup)
	}

	return PowerAction(u.cfg)
}

func (u *UpgradeAction) refineDeployment() error { //nolint:dupl
	var err error

	// Install grub
	if u.spec.BootloaderUpgrade {
		err = u.bootloader.Install(
			u.snapshot.WorkDir,
			u.spec.Partitions.EFI.MountPoint,
		)
		if err != nil {
			u.cfg.Logger.Errorf("failed installing grub: %v", err)
			return elementalError.NewFromError(err, elementalError.InstallGrub)
		}
	}

	// Relabel SELinux
	err = elemental.ApplySelinuxLabels(u.cfg.Config, u.spec.Partitions)
	if err != nil {
		u.cfg.Logger.Errorf("failed setting SELinux labels: %v", err)
		return elementalError.NewFromError(err, elementalError.SelinuxRelabel)
	}

	err = u.upgradeChrootHook(constants.AfterUpgradeChrootHook, constants.WorkingImgDir)
	if err != nil {
		u.Error("Error running hook after-upgrade-chroot: %s", err)
		return elementalError.NewFromError(err, elementalError.HookAfterUpgradeChroot)
	}
	err = u.upgradeHook(constants.AfterUpgradeHook)
	if err != nil {
		u.Error("Error running hook after-upgrade: %s", err)
		return elementalError.NewFromError(err, elementalError.HookAfterUpgrade)
	}

	grubVars := u.spec.GetGrubLabels()
	err = u.bootloader.SetPersistentVariables(
		filepath.Join(u.spec.Partitions.EFI.MountPoint, constants.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		u.Error("Error setting GRUB labels: %s", err)
		return elementalError.NewFromError(err, elementalError.SetGrubVariables)
	}

	err = u.bootloader.SetDefaultEntry(u.spec.Partitions.EFI.MountPoint, constants.WorkingImgDir, u.spec.GrubDefEntry)
	if err != nil {
		u.Error("failed setting default entry")
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}

	return nil
}
