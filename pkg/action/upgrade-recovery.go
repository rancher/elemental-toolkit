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
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	cnst "github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var ErrUpgradeRecoveryFromRecovery = errors.New("can not upgrade recovery from recovery partition")

// UpgradeRecoveryAction represents the struct that will run the recovery upgrade from start to finish
type UpgradeRecoveryAction struct {
	cfg                *types.RunConfig
	spec               *types.UpgradeSpec
	updateInstallState bool
}

type UpgradeRecoveryActionOption func(r *UpgradeRecoveryAction) error

func WithUpdateInstallState(updateInstallState bool) func(u *UpgradeRecoveryAction) error {
	return func(u *UpgradeRecoveryAction) error {
		u.updateInstallState = updateInstallState
		return nil
	}
}

func NewUpgradeRecoveryAction(config *types.RunConfig, spec *types.UpgradeSpec, opts ...UpgradeRecoveryActionOption) (*UpgradeRecoveryAction, error) {
	var err error

	u := &UpgradeRecoveryAction{cfg: config, spec: spec}

	for _, o := range opts {
		err = o(u)
		if err != nil {
			config.Logger.Errorf("error applying config option: %s", err.Error())
			return nil, err
		}
	}

	if elemental.IsRecoveryMode(config.Config) {
		config.Logger.Errorf("Upgrading recovery image from the recovery system itself is not supported")
		return nil, ErrUpgradeRecoveryFromRecovery
	}

	if u.spec.Partitions.Recovery == nil {
		return nil, fmt.Errorf("undefined recovery partition")
	}

	if u.updateInstallState {
		if u.spec.Partitions.State == nil {
			return nil, fmt.Errorf("undefined state partition")
		}
		// A nil State should never be the case.
		// However if it happens we need to abort, we we can't recreate
		// a correct install state when upgrading recovery only.
		if u.spec.State == nil {
			return nil, fmt.Errorf("could not load current install state")
		}
	}

	return u, nil
}

func (u UpgradeRecoveryAction) Infof(s string, args ...interface{}) {
	u.cfg.Logger.Infof(s, args...)
}

func (u UpgradeRecoveryAction) Debugf(s string, args ...interface{}) {
	u.cfg.Logger.Debugf(s, args...)
}

func (u UpgradeRecoveryAction) Errorf(s string, args ...interface{}) {
	u.cfg.Logger.Errorf(s, args...)
}

func (u UpgradeRecoveryAction) Warnf(s string, args ...interface{}) {
	u.cfg.Logger.Warnf(s, args...)
}

func (u *UpgradeRecoveryAction) mountRWPartitions(cleanup *utils.CleanStack) error {
	umount, err := elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.Recovery)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountRecoveryPartition)
	}
	cleanup.Push(umount)

	return nil
}

func (u *UpgradeRecoveryAction) upgradeInstallStateYaml() error {
	u.spec.State.Date = time.Now().Format(time.RFC3339)

	recoveryPart := u.spec.State.Partitions[constants.RecoveryPartName]
	if recoveryPart == nil {
		recoveryPart = &types.PartitionState{
			FSLabel: u.spec.Partitions.Recovery.FilesystemLabel,
			RecoveryImage: &types.SystemState{
				FS:         u.spec.RecoverySystem.FS,
				Label:      u.spec.RecoverySystem.Label,
				Source:     u.spec.RecoverySystem.Source,
				Digest:     u.spec.RecoverySystem.Source.GetDigest(),
				Labels:     u.spec.SnapshotLabels,
				Date:       u.spec.State.Date,
				FromAction: cnst.ActionUpgradeRecovery,
			},
		}
		u.spec.State.Partitions[constants.RecoveryPartName] = recoveryPart
	} else if recoveryPart.RecoveryImage != nil {
		recoveryPart.RecoveryImage.Date = u.spec.State.Date
		recoveryPart.RecoveryImage.Labels = u.spec.SnapshotLabels
		recoveryPart.RecoveryImage.FromAction = cnst.ActionUpgradeRecovery
	}

	// State partition is mounted in three different locations.
	// We can expect the state partition to be always RW mounted, since we are running from an active system, not recovery.
	// The problem is at partitions and mountpoint detection, it only returns a single mountpoint, not  all available ones.
	// Hardcoding constants.RunningStateDir should (currently) always work.
	statePath := filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
	return u.cfg.WriteInstallState(
		u.spec.State, statePath,
		filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.InstallStateFile),
	)
}

func (u *UpgradeRecoveryAction) Run() (err error) {
	cleanup := utils.NewCleanStack()
	defer func() {
		err = cleanup.Cleanup(err)
	}()

	// Mount required partitions as RW
	err = u.mountRWPartitions(cleanup)
	if err != nil {
		return err
	}

	// Remove any traces of previously errored upgrades
	transitionDir := filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.BootTransitionPath)
	u.Debugf("removing any orphaned recovery system %s", transitionDir)
	err = utils.RemoveAll(u.cfg.Fs, transitionDir)
	if err != nil {
		u.Errorf("failed removing orphaned recovery image: %s", err.Error())
		return err
	}

	// Deploy recovery system to transition dir
	err = elemental.DeployRecoverySystem(u.cfg.Config, &u.spec.RecoverySystem)
	if err != nil {
		u.cfg.Logger.Errorf("failed deploying recovery image: %s", err.Error())
		return elementalError.NewFromError(err, elementalError.DeployImage)
	}

	// Switch places on /boot and transition-dir
	bootDir := filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.BootPath)
	oldBootDir := filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.OldBootPath)

	// If a previous upgrade failed, remove old boot-dir
	err = utils.RemoveAll(u.cfg.Fs, oldBootDir)
	if err != nil {
		u.Errorf("failed removing orphaned recovery image: %s", err.Error())
		return err
	}

	// Rename current boot-dir in case we need to use it again
	if ok, _ := utils.Exists(u.cfg.Fs, bootDir); ok {
		err = u.cfg.Fs.Rename(bootDir, oldBootDir)
		if err != nil {
			u.Errorf("failed removing old recovery image: %s", err.Error())
			return err
		}
	}

	// Move new boot-dir to /boot
	err = u.cfg.Fs.Rename(transitionDir, bootDir)
	if err != nil {
		u.cfg.Logger.Errorf("failed renaming transition recovery image: %s", err.Error())

		// Try to salvage old recovery system
		if ok, _ := utils.Exists(u.cfg.Fs, oldBootDir); ok {
			err = u.cfg.Fs.Rename(oldBootDir, bootDir)
			if err != nil {
				u.cfg.Logger.Errorf("failed salvaging old recovery system: %s", err.Error())
			}
		}

		return err
	}

	// Remove old boot-dir when new recovery system is in place
	err = utils.RemoveAll(u.cfg.Fs, oldBootDir)
	if err != nil {
		u.Warnf("failed removing old recovery image: %s", err.Error())
	}

	// Update state.yaml file on recovery and state partitions
	if u.updateInstallState {
		err = u.upgradeInstallStateYaml()
		if err != nil {
			u.Errorf("failed upgrading installation metadata: %s", err.Error())
			return err
		}
	}

	u.Infof("Recovery upgrade completed")

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		u.Errorf("failed cleanup: %s", err.Error())
		return elementalError.NewFromError(err, elementalError.Cleanup)
	}

	return PowerAction(u.cfg)
}
