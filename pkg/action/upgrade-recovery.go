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

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var ErrUpgradeRecoveryFromRecovery = errors.New("Can not upgrade recovery from recovery partition")

// UpgradeRecoveryAction represents the struct that will run the recovery upgrade from start to finish
type UpgradeRecoveryAction struct {
	cfg                *v1.RunConfig
	spec               *v1.UpgradeSpec
	updateInstallState bool
}

type UpgradeRecoveryActionOption func(r *UpgradeRecoveryAction) error

func WithUpdateInstallState(updateInstallState bool) func(u *UpgradeRecoveryAction) error {
	return func(u *UpgradeRecoveryAction) error {
		u.updateInstallState = updateInstallState
		return nil
	}
}

func NewUpgradeRecoveryAction(config *v1.RunConfig, spec *v1.UpgradeSpec, opts ...UpgradeRecoveryActionOption) (*UpgradeRecoveryAction, error) {
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
			return nil, fmt.Errorf("Could not load current install state")
		}
	}

	return u, nil
}

func (u UpgradeRecoveryAction) Info(s string, args ...interface{}) {
	u.cfg.Logger.Infof(s, args...)
}

func (u UpgradeRecoveryAction) Debug(s string, args ...interface{}) {
	u.cfg.Logger.Debugf(s, args...)
}

func (u UpgradeRecoveryAction) Error(s string, args ...interface{}) {
	u.cfg.Logger.Errorf(s, args...)
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

	// Upgrade recovery
	err = elemental.DeployImage(u.cfg.Config, &u.spec.RecoverySystem)
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

	// Update state.yaml file on recovery and state partitions
	if u.updateInstallState {
		err = u.upgradeInstallStateYaml()
		if err != nil {
			u.Error("failed upgrading installation metadata")
			return err
		}
	}

	u.Info("Recovery upgrade completed")

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.Cleanup)
	}

	return nil
}
