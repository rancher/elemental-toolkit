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
	"time"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

// UpgradeAction represents the struct that will run the upgrade from start to finish
type UpgradeAction struct {
	config *v1.RunConfig
	spec   *v1.UpgradeSpec
}

func NewUpgradeAction(config *v1.RunConfig, spec *v1.UpgradeSpec) *UpgradeAction {
	return &UpgradeAction{config: config, spec: spec}
}

func (u UpgradeAction) Info(s string, args ...interface{}) {
	u.config.Logger.Infof(s, args...)
}

func (u UpgradeAction) Debug(s string, args ...interface{}) {
	u.config.Logger.Debugf(s, args...)
}

func (u UpgradeAction) Error(s string, args ...interface{}) {
	u.config.Logger.Errorf(s, args...)
}

func (u UpgradeAction) upgradeHook(hook string) error {
	u.Info("Applying '%s' hook", hook)
	return Hook(&u.config.Config, hook, u.config.Strict, u.config.CloudInitPaths...)
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
		mountPoints[persistentDevice.MountPoint] = constants.UsrLocalPath
	}

	return ChrootHook(&u.config.Config, hook, u.config.Strict, root, mountPoints, u.config.CloudInitPaths...)
}

func (u *UpgradeAction) upgradeInstallStateYaml(meta interface{}, img v1.Image) error {
	if u.spec.Partitions.Recovery == nil || u.spec.Partitions.State == nil {
		return fmt.Errorf("undefined state or recovery partition")
	}

	if u.spec.State == nil {
		u.spec.State = &v1.InstallState{
			Partitions: map[string]*v1.PartitionState{},
		}
	}

	u.spec.State.Date = time.Now().Format(time.RFC3339)
	imgState := &v1.ImageState{
		Source:         img.Source,
		SourceMetadata: meta,
		Label:          img.Label,
		FS:             img.FS,
	}
	if u.spec.RecoveryUpgrade {
		recoveryPart := u.spec.State.Partitions[constants.RecoveryPartName]
		if recoveryPart == nil {
			recoveryPart = &v1.PartitionState{
				Images:  map[string]*v1.ImageState{},
				FSLabel: u.spec.Partitions.Recovery.FilesystemLabel,
			}
			u.spec.State.Partitions[constants.RecoveryPartName] = recoveryPart
		}
		recoveryPart.Images[constants.RecoveryImgName] = imgState
	} else {
		statePart := u.spec.State.Partitions[constants.StatePartName]
		if statePart == nil {
			statePart = &v1.PartitionState{
				Images:  map[string]*v1.ImageState{},
				FSLabel: u.spec.Partitions.State.FilesystemLabel,
			}
			u.spec.State.Partitions[constants.StatePartName] = statePart
		}
		if statePart.Images[constants.PassiveImgName] == nil {
			statePart.Images[constants.PassiveImgName] = &v1.ImageState{
				Label: u.spec.Passive.Label,
			}
		}
		if statePart.Images[constants.ActiveImgName] != nil {
			// Do not copy the label from the old active image
			statePart.Images[constants.PassiveImgName].Source = statePart.Images[constants.ActiveImgName].Source
			statePart.Images[constants.PassiveImgName].SourceMetadata = statePart.Images[constants.ActiveImgName].SourceMetadata
			statePart.Images[constants.PassiveImgName].FS = statePart.Images[constants.ActiveImgName].FS
		}
		statePart.Images[constants.ActiveImgName] = imgState
	}

	return u.config.WriteInstallState(
		u.spec.State,
		filepath.Join(u.spec.Partitions.State.MountPoint, constants.InstallStateFile),
		filepath.Join(u.spec.Partitions.Recovery.MountPoint, constants.InstallStateFile),
	)
}

func (u *UpgradeAction) Run() (err error) {
	var upgradeImg v1.Image
	var finalImageFile string

	cleanup := utils.NewCleanStack()
	defer func() {
		err = cleanup.Cleanup(err)
	}()

	e := elemental.NewElemental(&u.config.Config)

	if u.spec.RecoveryUpgrade {
		upgradeImg = u.spec.Recovery
		finalImageFile = filepath.Join(u.spec.Partitions.Recovery.MountPoint, "cOS", constants.RecoveryImgFile)
	} else {
		upgradeImg = u.spec.Active
		finalImageFile = filepath.Join(u.spec.Partitions.State.MountPoint, "cOS", constants.ActiveImgFile)
	}

	umount, err := e.MountRWPartition(u.spec.Partitions.State)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountStatePartition)
	}
	cleanup.Push(umount)
	umount, err = e.MountRWPartition(u.spec.Partitions.Recovery)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountRecoveryPartition)
	}
	cleanup.Push(umount)

	// Cleanup transition image file before leaving
	cleanup.Push(func() error { return u.remove(upgradeImg.File) })

	// Recovery does not mount persistent, so try to mount it. Ignore errors, as it's not mandatory.
	persistentPart := u.spec.Partitions.Persistent
	if persistentPart != nil {
		// Create the dir otherwise the check for mounted dir fails
		_ = utils.MkdirAll(u.config.Fs, persistentPart.MountPoint, constants.DirPerm)
		if mnt, err := utils.IsMounted(&u.config.Config, persistentPart); !mnt && err == nil {
			u.Debug("mounting persistent partition")
			umount, err = e.MountRWPartition(persistentPart)
			if err != nil {
				u.config.Logger.Warn("could not mount persistent partition: %s", err.Error())
			} else {
				cleanup.Push(umount)
			}
		}
	}

	// before upgrade hook happens once partitions are RW mounted, just before image OS is deployed
	err = u.upgradeHook(constants.BeforeUpgradeHook)
	if err != nil {
		u.Error("Error while running hook before-upgrade: %s", err)
		return elementalError.NewFromError(err, elementalError.HookBeforeUpgrade)
	}

	u.Info("deploying image %s to %s", upgradeImg.Source.Value(), upgradeImg.File)
	// Deploy active image
	upgradeMeta, treeCleaner, err := e.DeployImgTree(&upgradeImg, constants.WorkingImgDir)
	if err != nil {
		u.Error("Failed deploying image to file '%s': %s", upgradeImg.File, err)
		return elementalError.NewFromError(err, elementalError.DeployImgTree)
	}
	cleanup.Push(func() error { return treeCleaner() })

	// Selinux relabel
	// Doesn't make sense to relabel a readonly filesystem
	if upgradeImg.FS != constants.SquashFs {
		// Relabel SELinux
		// TODO probably relabelling persistent volumes should be an opt in feature, it could
		// have undesired effects in case of failures
		binds := map[string]string{}
		if mnt, _ := utils.IsMounted(&u.config.Config, u.spec.Partitions.Persistent); mnt {
			binds[u.spec.Partitions.Persistent.MountPoint] = constants.UsrLocalPath
		}
		if mnt, _ := utils.IsMounted(&u.config.Config, u.spec.Partitions.OEM); mnt {
			binds[u.spec.Partitions.OEM.MountPoint] = constants.OEMPath
		}
		err = utils.ChrootedCallback(
			&u.config.Config, constants.WorkingImgDir, binds,
			func() error { return e.SelinuxRelabel("/", true) },
		)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.SelinuxRelabel)
		}
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
	err = utils.NewGrub(&u.config.Config).SetPersistentVariables(
		filepath.Join(u.spec.Partitions.State.MountPoint, constants.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		u.Error("Error setting GRUB labels: %s", err)
		return elementalError.NewFromError(err, elementalError.SetGrubVariables)
	}

	// Only apply rebrand stage for system upgrades
	if !u.spec.RecoveryUpgrade {
		u.Info("rebranding")

		err = e.SetDefaultGrubEntry(u.spec.Partitions.State.MountPoint, constants.WorkingImgDir, u.spec.GrubDefEntry)
		if err != nil {
			u.Error("failed setting default entry")
			return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
		}
	}

	err = e.CreateImgFromTree(constants.WorkingImgDir, &upgradeImg, false, treeCleaner)
	if err != nil {
		u.Error("failed creating transition image")
		return elementalError.NewFromError(err, elementalError.CreateImgFromTree)
	}

	// If not upgrading recovery, backup active into passive
	if !u.spec.RecoveryUpgrade {
		//TODO this step could be part of elemental package
		// backup current active.img to passive.img before overwriting the active.img
		u.Info("Backing up current active image")
		source := filepath.Join(u.spec.Partitions.State.MountPoint, "cOS", constants.ActiveImgFile)
		u.Info("Moving %s to %s", source, u.spec.Passive.File)
		_, err := u.config.Runner.Run("mv", "-f", source, u.spec.Passive.File)
		if err != nil {
			u.Error("Failed to move %s to %s: %s", source, u.spec.Passive.File, err)
			return elementalError.NewFromError(err, elementalError.MoveFile)
		}
		u.Info("Finished moving %s to %s", source, u.spec.Passive.File)
		// Label the image to passive!
		out, err := u.config.Runner.Run("tune2fs", "-L", u.spec.Passive.Label, u.spec.Passive.File)
		if err != nil {
			u.Error("Error while labeling the passive image %s: %s", u.spec.Passive.File, err)
			u.Debug("Error while labeling the passive image %s, command output: %s", out)
			return elementalError.NewFromError(err, elementalError.LabelImage)
		}
		_, _ = u.config.Runner.Run("sync")
	}

	u.Info("Moving %s to %s", upgradeImg.File, finalImageFile)
	_, err = u.config.Runner.Run("mv", "-f", upgradeImg.File, finalImageFile)
	if err != nil {
		u.Error("Failed to move %s to %s: %s", upgradeImg.File, finalImageFile, err)
		return elementalError.NewFromError(err, elementalError.MoveFile)
	}
	u.Info("Finished moving %s to %s", upgradeImg.File, finalImageFile)

	_, _ = u.config.Runner.Run("sync")

	err = u.upgradeHook(constants.PostUpgradeHook)
	if err != nil {
		u.Error("Error running hook post-upgrade: %s", err)
		return elementalError.NewFromError(err, elementalError.HookPostUpgrade)
	}

	// Update state.yaml file on recovery and state partitions
	err = u.upgradeInstallStateYaml(upgradeMeta, upgradeImg)
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

	return PowerAction(u.config)
}

// remove attempts to remove the given path. Does nothing if it doesn't exist
func (u *UpgradeAction) remove(path string) error {
	if exists, _ := utils.Exists(u.config.Fs, path); exists {
		u.Debug("[Cleanup] Removing %s", path)
		return u.config.Fs.RemoveAll(path)
	}
	return nil
}
