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

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

// UpgradeRecoveryAction represents the struct that will run the upgrade from start to finish
type UpgradeRecoveryAction struct {
	cfg  *v1.RunConfig
	spec *v1.UpgradeSpec
}

func NewUpgradeRecoveryAction(config *v1.RunConfig, spec *v1.UpgradeSpec) *UpgradeRecoveryAction {
	return &UpgradeRecoveryAction{cfg: config, spec: spec}
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
		return fmt.Errorf("Can not upgrade recovery from recovery partition")
	}

	umount, err := elemental.MountRWPartition(u.cfg.Config, u.spec.Partitions.Recovery)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountRecoveryPartition)
	}
	cleanup.Push(umount)

	return nil
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
	if u.spec.RecoveryUpgrade {
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
	}

	u.Info("Recovery upgrade completed")

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.Cleanup)
	}

	return nil
}
