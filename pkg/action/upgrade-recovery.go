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
		return nil, fmt.Errorf("Not supported")
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
	if elemental.IsRecoveryMode(u.cfg.Config) {
		return ErrUpgradeRecoveryFromRecovery
	}

	if u.updateInstallState {
		umount, err := elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.State)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.MountStatePartition)
		}
		cleanup.Push(umount)
	}

	umount, err := elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.Recovery)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountRecoveryPartition)
	}
	cleanup.Push(umount)

	return nil
}

func (u *UpgradeRecoveryAction) upgradeInstallStateYaml() error {
	if u.spec.Partitions.Recovery == nil || u.spec.Partitions.State == nil {
		return fmt.Errorf("undefined state or recovery partition")
	}

	// A nil State should never be the case.
	// However if it happens we need to abort, we we can't recreate
	// a correct install state when upgrading recovery only.
	if u.spec.State == nil {
		return fmt.Errorf("Could not load current install state")
	}

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

	// Hack to ensure we are not using / or /.snapshots mountpoints. Btrfs based deployments
	// mount state partition into multiple locations
	statePath := filepath.Join(u.spec.Partitions.State.MountPoint, constants.InstallStateFile)
	if u.spec.Partitions.State.MountPoint == "/" || u.spec.Partitions.State.MountPoint == "/.snapshots" {
		statePath = filepath.Join(constants.RunningStateDir, constants.InstallStateFile)
	}

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
