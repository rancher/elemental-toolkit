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
	"fmt"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/elemental"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"os"
	"path/filepath"
)

// UpgradeAction represents the struct that will run the upgrade from start to finish
type UpgradeAction struct {
	Config *v1.RunConfig
}

func NewUpgradeAction(config *v1.RunConfig) *UpgradeAction {
	return &UpgradeAction{Config: config}
}

func (u *UpgradeAction) Info(s string, args ...interface{}) {
	u.Config.Logger.Infof(s, args...)
}

func (u *UpgradeAction) Debug(s string, args ...interface{}) {
	u.Config.Logger.Debugf(s, args...)
}

func (u *UpgradeAction) Error(s string, args ...interface{}) {
	u.Config.Logger.Errorf(s, args...)
}

func upgradeHook(config *v1.RunConfig, hook string, chroot bool) error {
	if chroot {
		mountPoints := map[string]string{}

		oemDevice, err := utils.GetFullDeviceByLabel(config.Runner, config.OEMLabel, 2)
		if err == nil && oemDevice.MountPoint != "" {
			mountPoints[oemDevice.MountPoint] = "/oem"
		}

		persistentDevice, err := utils.GetFullDeviceByLabel(config.Runner, config.PersistentLabel, 2)
		if err == nil && persistentDevice.MountPoint != "" {
			mountPoints[persistentDevice.MountPoint] = "/usr/local"
		}

		return ActionChrootHook(config, hook, config.Images[constants.ActiveImgName].MountPoint, mountPoints)
	}
	return ActionHook(config, hook)
}

func (u *UpgradeAction) Run() (err error) {
	var transitionImg string
	var isSquashRecovery bool
	var upgradeStateDir string
	// When booting from recovery the label can be the recovery or the system, depending on the recovery img type (squash/non-squash)
	bootedFromRecovery := utils.BootedFrom(u.Config.Runner, u.Config.RecoveryLabel) || utils.BootedFrom(u.Config.Runner, u.Config.SystemLabel)
	u.Debug("Booted from recovery: %v", bootedFromRecovery)
	// To check if we are on squash recovery we need to check different depending on where we are
	if bootedFromRecovery {
		// Here is simple, we just check if the file is there
		if exists, _ := afero.Exists(u.Config.Fs, filepath.Join(constants.UpgradeRecoveryDir, "cOS", constants.RecoverySquashFile)); exists {
			isSquashRecovery = true
		}
	} else {
		// Booting from active, mount the recovery partition and check for the squashfs file
		recoveryPart, err := utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.RecoveryLabel, 2)
		if err != nil {
			u.Error("Could not find device for %s label: %s", u.Config.RecoveryLabel, err)
			return err
		}
		tmpMountDir, _ := afero.TempDir(u.Config.Fs, "", "elemental")
		err = u.Config.Mounter.Mount(recoveryPart.Path, tmpMountDir, recoveryPart.FS, []string{})
		if err != nil {
			return err
		}
		if exists, _ := afero.Exists(u.Config.Fs, filepath.Join(tmpMountDir, "cOS", constants.RecoverySquashFile)); exists {
			isSquashRecovery = true
		}
		// Cleanup
		err = u.Config.Mounter.Unmount(tmpMountDir)
		if err != nil {
			return err
		}
		err = u.Config.Fs.RemoveAll(tmpMountDir)
		if err != nil {
			return err
		}
	}
	u.Debug("Is squash recovery: %v", isSquashRecovery)

	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	// Work out the paths and current system to mount the upgrade state dir
	// We booted from recovery
	if bootedFromRecovery {
		// We are updating recovery
		if u.Config.RecoveryUpgrade {
			recoveryPart, _ := utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.RecoveryLabel, 2)
			err = u.Config.Mounter.Mount(recoveryPart.Path, recoveryPart.MountPoint, recoveryPart.FS, []string{"remount", "rw"})
			if err != nil {
				u.Error("Error mounting %s: %s", recoveryPart.MountPoint, err)
				return err
			}
			// set upgradeStateDir to recovery mountpoint
			upgradeStateDir = recoveryPart.MountPoint
		} else {
			// We are upgrading the active so we need to mount the state label partition on /run/cos/state
			statePart, _ := utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.StateLabel, 2)
			err = u.Config.Fs.MkdirAll(constants.StateDir, os.ModeDir)
			if err != nil {
				u.Error("Error creating dir %s: %s", constants.StateDir, err)
				return err
			}
			err = u.Config.Mounter.Mount(statePart.Path, constants.StateDir, statePart.FS, []string{})
			if err != nil {
				u.Error("Error mounting %s: %s", constants.StateDir, err)
				return err
			}
			cleanup.Push(func() error {
				return u.Config.Mounter.Unmount(constants.StateDir)
			})
			upgradeStateDir = constants.StateDir
		}
	} else { // We booted from active/passive
		//  We are updating recovery
		if u.Config.RecoveryUpgrade {
			// Try to mount SYSTEM partition, only exists if recovery is squash
			var recoveryPart v1.Partition
			recoveryPart, err = utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.SystemLabel, 2)
			if err != nil {
				// Failure to get the system label, fallback tor recovery label
				recoveryPart, err = utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.RecoveryLabel, 2)
			}
			err = u.Config.Fs.MkdirAll(constants.RecoveryDir, os.ModeDir)
			if err != nil {
				u.Error("Error creating dir %s: %s", constants.RecoveryDir, err)
				return err
			}
			// set upgradeStateDir to /run/cos/recovery
			err = u.Config.Mounter.Mount(recoveryPart.Path, constants.RecoveryDir, recoveryPart.FS, []string{})
			if err != nil {
				u.Error("Error mounting %s: %s", constants.RecoveryDir, err)
				return err
			}
			cleanup.Push(func() error {
				return u.Config.Mounter.Unmount(constants.RecoveryDir)
			})
			upgradeStateDir = constants.RecoveryDir
		} else { // We are updating active
			// Remount state partition RW
			statePart, _ := utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.StateLabel, 2)
			err = u.Config.Mounter.Mount(statePart.Path, statePart.MountPoint, statePart.FS, []string{"remount", "rw"})
			if err != nil {
				u.Error("Error mounting %s: %s", statePart.MountPoint, err)
				return err
			}
			// set upgradeStateDir to /run/initramfs/cos-state
			upgradeStateDir = statePart.MountPoint
		}
	}

	// Some debug info just in case
	u.Debug("Upgrade state dir: %s", upgradeStateDir)

	upgradeTarget, upgradeSource := u.getTargetAndSource()

	u.Config.Logger.Infof("Upgrading %s partition", upgradeTarget)

	// Both Recoveries do not mount persistent, so try to mount it. Ignore errors, as its not mandatory.
	if bootedFromRecovery {
		persistentPart, err := utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.PersistentLabel, 2)
		if err == nil {
			err := u.Config.Fs.MkdirAll(constants.PersistentDir, os.ModeDir)
			if err == nil {
				err = u.Config.Mounter.Mount(persistentPart.Path, constants.PersistentDir, persistentPart.FS, []string{})
				if err != nil {
					u.Config.Logger.Warnf("Could not mount persistent partition: %s", err)
				} else {
					cleanup.Push(func() error { return u.unmount(constants.PersistentDir) })
				}
			}
		}
	}

	// If we are upgrading the recovery and have squash, the transition img naming is different
	if isSquashRecovery && u.Config.RecoveryUpgrade {
		transitionImg = filepath.Join(upgradeStateDir, "cOS", constants.TransitionSquashFile)
	} else {
		transitionImg = filepath.Join(upgradeStateDir, "cOS", constants.TransitionImgFile)
	}

	u.Debug("Using transition img: %s", transitionImg)

	cleanup.Push(func() error { return u.remove(transitionImg) })

	// Get the upgradeTempDir here, so we use the persistent partition if mounted
	upgradeTempDir := utils.GetUpgradeTempDir(u.Config)
	u.Debug("Upgrade temp dir: %s", upgradeTempDir)

	err = u.Config.Fs.MkdirAll(upgradeTempDir, os.ModeDir)
	if err != nil {
		u.Error("Error creating target dir %s: %s", upgradeTempDir, err)
		return err
	}
	cleanup.Push(func() error { return u.remove(upgradeTempDir) })

	// create transition.img
	img := v1.Image{
		File:       transitionImg,
		Size:       u.Config.ImgSize,
		Label:      u.Config.ActiveLabel,
		FS:         constants.LinuxImgFs,
		MountPoint: upgradeTempDir,
		Source:     upgradeSource, // if source is a dir it will copy from here, if it's a docker img it uses Config.DockerImg IN THAT ORDER!
	}

	// If upgrading recovery, set the label to the RecoveryLabel instead
	if u.Config.RecoveryUpgrade {
		img.Label = u.Config.SystemLabel
	}

	ele := elemental.NewElemental(u.Config)

	if u.Config.RecoveryUpgrade && isSquashRecovery {
		u.Debug("Upgrading recovery+squash, not mounting image file")
	} else {
		// Only on recovery+squash we dont use the img file
		err = ele.CreateFileSystemImage(&img)
		if err != nil {
			u.Error("Failed to create %s img: %s", transitionImg, err)
			return err
		}

		// mount the transition img on targetDir, so we can install the upgraded files into targetDir, and they end up on the img
		err = ele.MountImage(&img, "rw")
	}

	for _, d := range []string{"proc", "boot", "dev", "sys", "tmp", "usr/local", "oem"} {
		_ = u.Config.Fs.MkdirAll(filepath.Join(upgradeTempDir, d), os.ModeDir)
	}

	err = upgradeHook(u.Config, "before-upgrade", false)
	if err != nil {
		u.Error("Error while running hook before-upgrade: %s", err)
		return err
	}
	// Setting the activeImg to our img, tricks CopyActive into doing it anyway even if it's a recovery img
	u.Config.Images[constants.ActiveImgName] = &img
	err = ele.CopyImage(&img)
	if err != nil {
		u.Error("Error copying active: %s", err)
		return err
	}
	// Selinux relabel
	// In the original script, any errors are ignored
	_, _ = u.Config.Runner.Run("chmod", "755", upgradeTempDir)
	_ = ele.SelinuxRelabel(upgradeTempDir, false)

	// Only run rebrand on non recovery+squash
	err = upgradeHook(u.Config, "after-upgrade-chroot", true)
	if err != nil {
		u.Error("Error running hook after-upgrade-chroot: %s", err)
		return err
	}

	// for the grub rebrand to work, we need to mount the state partition RW, so it can write into it
	statePartForRecovery, err := utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.StateLabel, 2)
	if err == nil {
		// If its not mounted, mount it so we can rebrand then unmount it
		if statePartForRecovery.MountPoint == "" {
			_ = u.Config.Fs.MkdirAll(constants.StateDir, os.ModeDir)
			if notMounted, _ := u.Config.Mounter.IsLikelyNotMountPoint(constants.StateDir); notMounted {
				err = u.Config.Mounter.Mount(statePartForRecovery.Path, constants.StateDir, statePartForRecovery.FS, []string{"rw"})
				if err != nil {
					u.Error("Could not mount state partition with label %s for rebrand: %s", u.Config.StateLabel, err)
					return err
				}
				cleanup.Push(func() error {
					return u.unmount(constants.StateDir)
				})
			}
		} else {
			// if mounted it may be RO only, so remount it RW?
			err = u.Config.Mounter.Mount(statePartForRecovery.Path, statePartForRecovery.MountPoint, statePartForRecovery.FS, []string{"remount", "rw"})
			if err != nil {
				u.Error("Could not remount state partition with label %s for rebrand: %s", u.Config.StateLabel, err)
				return err
			}
		}
	}
	// Load the os-release file from the new upgraded system
	osRelease, err := utils.LoadEnvFile(u.Config.Fs, filepath.Join(upgradeTempDir, "etc", "os-release"))
	// override grub vars with the new system vars
	u.Config.GrubDefEntry = osRelease["GRUB_ENTRY_NAME"]

	err = ele.Rebrand()

	if err != nil {
		u.Error("Error running rebrand: %s", err)
		return err
	}
	err = upgradeHook(u.Config, "after-upgrade", false)

	if err != nil {
		u.Error("Error running hook after-upgrade: %s", err)
		return err
	}

	if u.Config.RecoveryUpgrade && isSquashRecovery {
		u.Debug("Upgrading recovery+squash, not umounting image file")
	} else {
		// Copy is done, unmount transition.img
		err = ele.UnmountImage(&img)
		if err != nil {
			u.Error("Error unmounting %s: %s", img.MountPoint, err)
			return err
		}
	}

	// If not upgrading recovery, backup active into passive
	if !u.Config.RecoveryUpgrade {
		// backup current active.img to passive.img before overwriting the active.img
		u.Info("Backing up current active image")
		source := filepath.Join(upgradeStateDir, "cOS", constants.ActiveImgFile)
		destination := filepath.Join(upgradeStateDir, "cOS", constants.PassiveImgFile)
		u.Info("Moving %s to %s", source, destination)
		_, err := u.Config.Runner.Run("mv", "-f", source, destination)
		if err != nil {
			u.Error("Failed to move %s to %s: %s", source, destination, err)
			return err
		}
		u.Info("Finished moving %s to %s", source, destination)
		// Label the image to passive!
		out, err := u.Config.Runner.Run("tune2fs", "-L", u.Config.PassiveLabel, destination)
		if err != nil {
			u.Error("Error while labeling the passive image %s: %s", destination, err)
			u.Debug("Error while labeling the passive image %s, command output: %s", out)
			return err
		}
		_, _ = u.Config.Runner.Run("sync")
	}
	// Final step, move the newly updated img/squash into the proper place
	finalDestination := filepath.Join(upgradeStateDir, "cOS", fmt.Sprintf("%s.img", upgradeTarget))

	// if we are upgrading the recovery and its squash recovery, we need to create the squash file
	if isSquashRecovery && u.Config.RecoveryUpgrade {
		finalDestination = filepath.Join(upgradeStateDir, "cOS", constants.RecoverySquashFile)
		options := constants.GetDefaultSquashfsOptions()
		u.Info("Creating %s", constants.RecoverySquashFile)
		err = utils.CreateSquashFS(u.Config.Runner, u.Config.Logger, upgradeTempDir, transitionImg, options)
		if err != nil {
			return err
		}
	}

	u.Info("Moving %s to %s", transitionImg, finalDestination)
	_, err = u.Config.Runner.Run("mv", "-f", transitionImg, finalDestination)
	if err != nil {
		u.Error("Failed to move %s to %s: %s", transitionImg, finalDestination, err)
		return err
	}
	u.Info("Finished moving %s to %s", transitionImg, finalDestination)

	_, _ = u.Config.Runner.Run("sync")

	u.Info("Upgrade completed")

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return err
	}
	if u.Config.Reboot {
		u.Info("Rebooting in 5 seconds")
		return utils.Reboot(u.Config.Runner, 5)
	} else if u.Config.PowerOff {
		u.Info("Shutting down in 5 seconds")
		return utils.Shutdown(u.Config.Runner, 5)
	}
	return err
}

// unmount attempts to unmount the given path. Does nothing if not mounted
func (u *UpgradeAction) unmount(path string) error {
	if notMounted, _ := u.Config.Mounter.IsLikelyNotMountPoint(path); !notMounted {
		u.Debug("[Cleanup] Unmounting %s", path)
		return u.Config.Mounter.Unmount(path)
	}
	return nil
}

// remove attempts to remove the given path. Does nothing if it doesn't exist
func (u *UpgradeAction) remove(path string) error {
	if exists, _ := afero.Exists(u.Config.Fs, path); exists {
		u.Debug("[Cleanup] Removing %s", path)
		return u.Config.Fs.RemoveAll(path)
	}
	return nil
}

// remount attemps to remount the given mountpoint with the provided options. Does nothing if not mounted
func (u *UpgradeAction) remount(m mount.MountPoint, opts ...string) error {
	if notMounted, _ := u.Config.Mounter.IsLikelyNotMountPoint(m.Path); !notMounted {
		u.Debug("[Cleanup] Remount %s", m.Path)
		return u.Config.Mounter.Mount(m.Device, m.Path, m.Type, append([]string{"remount"}, opts...))
	}
	return nil
}

// getTargetAndSource finds our the target and source for the upgrade
func (u *UpgradeAction) getTargetAndSource() (string, v1.ImageSource) {
	upgradeSource := v1.NewChannelSrc(constants.UpgradeSource)
	upgradeTarget := constants.UpgradeActive

	if u.Config.RecoveryUpgrade {
		u.Debug("Upgrading recovery")
		upgradeTarget = constants.UpgradeRecovery
	}
	// if channel_upgrades==true then it picks the default image from /etc/cos-upgrade-image
	// this means, it gets the UPGRADE_IMAGE(default system/cos)/RECOVERY_IMAGE from the luet repo configured on the system
	if u.Config.ChannelUpgrades {
		u.Debug("Source is channel-upgrades")
		if u.Config.RecoveryUpgrade {
			if u.Config.RecoveryImage == "" {
				if u.Config.UpgradeImage != "" {
					upgradeSource = v1.NewChannelSrc(u.Config.UpgradeImage)
				}
			} else {
				upgradeSource = v1.NewChannelSrc(u.Config.RecoveryImage)
			}
		} else {
			if u.Config.UpgradeImage != "" { // I don't think it's possible to have an empty UpgradeImage....
				// Only override the source if we have a valid UpgradeImage, otherwise use the default
				upgradeSource = v1.NewChannelSrc(u.Config.UpgradeImage) // Loaded from /etc/cos-upgrade-image
			}
		}
	} else {
		// if channel_upgrades==false then
		// if docker-image -> upgrade from image directly, ignores release_channel and pulls the given image directly
		if u.Config.DockerImg != "" {
			u.Debug("Source is docker image: %s", u.Config.DockerImg)
			upgradeSource = v1.NewDockerSrc(u.Config.DockerImg)
		}
		// if directory -> upgrade from dir directly, ignores release_channel and uses the given directory
		if u.Config.DirectoryUpgrade != "" {
			u.Debug("Source is directory: %s", u.Config.DirectoryUpgrade)
			upgradeSource = v1.NewDirSrc(u.Config.DirectoryUpgrade)
		}
	}
	u.Debug("Upgrade target: %s Upgrade source: %s", upgradeTarget, upgradeSource.Value())
	return upgradeTarget, upgradeSource
}
