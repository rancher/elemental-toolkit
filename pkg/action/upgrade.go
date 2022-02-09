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
	"github.com/hashicorp/go-multierror"
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

// Cleanup is the struct that will be passed to the cleanup function at the end of the upgrade or at any error before returning
type Cleanup struct {
	Unmount []string
	Remove  []string
	Remount []mount.MountPoint
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
		return ActionChrootHook(
			config, hook, config.ActiveImage.MountPoint,
			map[string]string{
				"/usr/local": "/usr/local",
				"/oem":       "/oem",
			},
		)
	}
	return ActionHook(config, hook)
}

func rebrandChroot(config *v1.RunConfig) error {
	grub := utils.NewGrub(config)
	chroot := utils.NewChroot(config.ActiveImage.MountPoint, config)
	stateDevice, err := utils.GetFullDeviceByLabel(config.Runner, config.StateLabel, 5)
	if err != nil {
		config.Logger.Errorf("Could not get state partition")
		return err
	}

	chroot.SetExtraMounts(map[string]string{
		"/usr/local":           "/usr/local",
		"/oem":                 "/oem",
		stateDevice.MountPoint: "/run/boot",
	})

	err = chroot.Prepare()
	if err != nil {
		config.Logger.Errorf("Failed to setup chroot: %s", err)
		err := chroot.Close()
		if err != nil {
			config.Logger.Errorf("Also failed to close chroot: %s", err)
			return err
		}
		return err
	}
	defer chroot.Close()

	callback := func() error {
		// Reload the data from the chroot /etc/os-release as it's different from the running system
		chrootOsRelease, err := utils.LoadOsRelease(config.Fs)
		if err != nil {
			config.Logger.Errorf("Could not load /etc/os-release values of the chroot system: %s", err)
			return err
		}
		grubEnvFile := filepath.Join("/", "run", "boot", constants.GrubOEMEnv)
		return grub.SetPersistentVariables(grubEnvFile, map[string]string{"default_menu_entry": chrootOsRelease["GRUB_ENTRY_NAME"]})
	}
	return chroot.RunCallback(callback)
}

func (u *UpgradeAction) Run() error {
	var statePart v1.Partition
	var err error
	var transitionImg string
	cleanup := Cleanup{Remove: []string{constants.UpgradeTempDir}}
	upgradeStateDir := constants.UpgradeStateDir

	// if upgrading the recovery we mount the state in a different place as its already mounted RO, we need to remount it
	if u.Config.RecoveryUpgrade {
		upgradeStateDir = constants.UpgradeRecoveryDir
	}

	upgradeTarget, upgradeSource := u.getTargetAndSource()

	u.Config.Logger.Infof("Upgrading %s partition", upgradeTarget)

	err = u.Config.Fs.MkdirAll(constants.UpgradeTempDir, os.ModeDir)
	if err != nil {
		u.Error("Error creating target dir %s: %s", constants.UpgradeTempDir, err)
		return u.cleanup(cleanup, err)
	}

	cleanup.Remove = append(cleanup.Remove, constants.UpgradeTempDir)

	if u.Config.RecoveryUpgrade {
		statePart, err = utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.RecoveryLabel, 5)
		if err != nil {
			u.Error("Could not find state partition to mount with label %s", u.Config.RecoveryLabel)
			return u.cleanup(cleanup, err)
		}
	} else {
		statePart, err = utils.GetFullDeviceByLabel(u.Config.Runner, u.Config.StateLabel, 5)
		if err != nil {
			u.Error("Could not find state partition to mount with label %s", u.Config.StateLabel)
			return u.cleanup(cleanup, err)
		}
	}

	u.Info("Mounting state partition %s in %s", statePart.Path, upgradeStateDir)
	if exists, _ := afero.Exists(u.Config.Fs, upgradeStateDir); !exists {
		err = u.Config.Fs.MkdirAll(upgradeStateDir, os.ModeDir)

		if err != nil {
			u.Error("Error creating statedir %s: %s", upgradeStateDir, err)
			return u.cleanup(cleanup, err)
		}
	}

	statePartMountOptions := []string{"remount", "rw"}

	// If we want to upgrade the active but are booting from recovery, the statedir is not mounted, so dont remount
	if !u.Config.RecoveryUpgrade && utils.BootedFrom(u.Config.Runner, u.Config.RecoveryLabel) {
		statePartMountOptions = []string{"rw"}
		cleanup.Unmount = append(cleanup.Unmount, upgradeStateDir)
	}

	// If we want to upgrade the recovery but are not booting from recovery, the stateDir is not mounted, so dont try to remount
	if u.Config.RecoveryUpgrade && !utils.BootedFrom(u.Config.Runner, u.Config.RecoveryLabel) {
		statePartMountOptions = []string{"rw"}
		cleanup.Unmount = append(cleanup.Unmount, upgradeStateDir)
	}

	err = u.Config.Mounter.Mount(statePart.Path, upgradeStateDir, statePart.FS, statePartMountOptions)
	if err != nil {
		u.Error("Error mounting %s: %s", upgradeStateDir, err)
		return u.cleanup(cleanup, err)
	}

	if !utils.BootedFrom(u.Config.Runner, u.Config.RecoveryLabel) {
		cleanup.Remount = append(cleanup.Remount, mount.MountPoint{Device: statePart.Path, Path: upgradeStateDir, Type: statePart.FS})
	}

	// Track if recovery.squash file exists which indeicates that the recovery is squash
	isSquashRecovery, _ := afero.Exists(u.Config.Fs, filepath.Join(upgradeStateDir, "cOS", constants.RecoverySquashFile))

	if isSquashRecovery {
		u.Debug("Recovery is squash")
		transitionImg = filepath.Join(upgradeStateDir, "cOS", constants.TransitionSquashFile)
	} else {
		transitionImg = filepath.Join(upgradeStateDir, "cOS", constants.TransitionImgFile)
	}

	u.Debug("Using transition img: %s", transitionImg)

	cleanup.Remove = append(cleanup.Remove, transitionImg)

	// create transition.img
	img := v1.Image{
		File:       transitionImg,
		Size:       u.Config.ImgSize,
		Label:      u.Config.ActiveLabel,
		FS:         constants.LinuxImgFs,
		MountPoint: constants.UpgradeTempDir,
		RootTree:   upgradeSource.Source, // if source is a dir it will copy from here, if it's a docker img it uses Config.DockerImg IN THAT ORDER!
	}

	// If on recovery, set the label to the RecoveryLabel instead
	if utils.BootedFrom(u.Config.Runner, u.Config.RecoveryLabel) {
		img.Label = u.Config.SystemLabel
	}

	ele := elemental.NewElemental(u.Config)

	if !isSquashRecovery {
		// Only on recovery+squash we dont use the img file
		err = ele.CreateFileSystemImage(img)
		if err != nil {
			u.Error("Failed to create %s img: %s", transitionImg, err)
			return u.cleanup(cleanup, err)
		}

		// mount the transition img on targetDir, so we can install the upgraded files into targetDir, and they end up on the img
		err = ele.MountImage(&img, "rw")
	}

	for _, d := range []string{"proc", "boot", "dev", "sys", "tmp", "usr/local", "oem"} {
		_ = u.Config.Fs.MkdirAll(filepath.Join(constants.UpgradeTempDir, d), os.ModeDir)
	}

	err = upgradeHook(u.Config, "before-upgrade", false)
	if err != nil {
		u.Error("Error while running hook before-upgrade: %s", err)
		return u.cleanup(cleanup, err)
	}
	// Setting the activeImg to our img, tricks CopyActive into doing it anyway even if it's a recovery img
	u.Config.ActiveImage = img
	err = ele.CopyActive(upgradeSource)
	if err != nil {
		u.Error("Error copying active: %s", err)
		return u.cleanup(cleanup, err)
	}
	// Selinux relabel
	// In the original script, any errors are ignored
	_, _ = u.Config.Runner.Run("chmod", "755", constants.UpgradeTempDir)
	_ = ele.SelinuxRelabel(constants.UpgradeTempDir, false)

	// Only run rebrand on non recovery+squash
	err = upgradeHook(u.Config, "after-upgrade-chroot", true)
	if err != nil {
		u.Error("Error running hook after-upgrade-chroot: %s", err)
		return u.cleanup(cleanup, err)
	}

	err = rebrandChroot(u.Config)
	if err != nil {
		u.Error("Error running rebrand: %s", err)
		return u.cleanup(cleanup, err)
	}
	err = upgradeHook(u.Config, "after-upgrade", false)

	if err != nil {
		u.Error("Error running hook after-upgrade: %s", err)
		return u.cleanup(cleanup, err)
	}

	if !isSquashRecovery {
		// Copy is done, unmount transition.img
		err = ele.UnmountImage(&img)
		if err != nil {
			u.Error("Error unmounting %s: %s", img.MountPoint, err)
			return err
		}
	}

	// If booted from active and not updating recovery, backup active into passive
	if utils.BootedFrom(u.Config.Runner, u.Config.ActiveLabel) && !u.Config.RecoveryUpgrade {
		// backup current active.img to passive.img before overwriting the active.img
		u.Info("Backing up current active image")
		source := filepath.Join(upgradeStateDir, "cOS", constants.ActiveImgFile)
		destination := filepath.Join(upgradeStateDir, "cOS", constants.PassiveImgFile)
		u.Info("Moving %s to %s", source, destination)
		_, err := u.Config.Runner.Run("mv", "-f", source, destination)
		if err != nil {
			u.Error("Failed to move %s to %s: %s", source, destination, err)
			return u.cleanup(cleanup, err)
		}
		u.Info("Finished moving %s to %s", source, destination)
		// Label the image to passive!
		out, err := u.Config.Runner.Run("tune2fs", "-L", u.Config.PassiveLabel, destination)
		if err != nil {
			u.Error("Error while labeling the passive image %s: %s", destination, err)
			u.Debug("Error while labeling the passive image %s, command output: %s", out)
			return u.cleanup(cleanup, err)
		}
		_, _ = u.Config.Runner.Run("sync")
	}
	// Final step, move the newly updated img/squash into the proper place
	finalDestination := filepath.Join(upgradeStateDir, "cOS", fmt.Sprintf("%s.img", upgradeTarget))

	if isSquashRecovery {
		finalDestination = filepath.Join(upgradeStateDir, "cOS", constants.RecoverySquashFile)
		options := constants.GetDefaultSquashfsOptions()
		u.Info("Creating %s", constants.RecoverySquashFile)
		err = utils.CreateSquashFS(u.Config.Runner, u.Config.Logger, constants.UpgradeTempDir, transitionImg, options)
		if err != nil {
			return u.cleanup(cleanup, err)
		}
	}

	u.Info("Moving %s to %s", transitionImg, finalDestination)
	_, err = u.Config.Runner.Run("mv", "-f", transitionImg, finalDestination)
	if err != nil {
		u.Error("Failed to move %s to %s: %s", transitionImg, finalDestination, err)
		return u.cleanup(cleanup, err)
	}
	u.Info("Finished moving %s to %s", transitionImg, finalDestination)

	_, _ = u.Config.Runner.Run("sync")

	u.Info("Upgrade completed")

	if u.Config.Reboot {
		err = u.cleanup(cleanup, err)
		if err != nil {
			// If cleanup fails, do not reboot
			return err
		} else {
			u.Info("Rebooting in 5 seconds")
			return utils.Reboot(u.Config.Runner, 5)
		}
	} else if u.Config.PowerOff {
		err = u.cleanup(cleanup, err)
		if err != nil {
			// If cleanup fails, do not shut down
			return err
		} else {
			u.Info("Shutting down in 5 seconds")
			return utils.Shutdown(u.Config.Runner, 5)
		}
	}
	return u.cleanup(cleanup, err)
}

func (u *UpgradeAction) cleanup(cleanup Cleanup, originalError error) error {
	// first try to unmount
	var errs error

	for _, m := range cleanup.Unmount {
		if notMounted, _ := u.Config.Mounter.IsLikelyNotMountPoint(m); !notMounted {
			u.Debug("[Cleanup] Unmounting %s", m)
			err := u.Config.Mounter.Unmount(m)
			if err != nil {
				// Save errors and continue
				errs = multierror.Append(errs, err)
			}
		}
	}

	// Then cleanup dirs/files
	for _, f := range cleanup.Remove {
		if exists, _ := afero.Exists(u.Config.Fs, f); exists {
			u.Debug("[Cleanup] Removing %s", f)
			err := u.Config.Fs.RemoveAll(f)
			if err != nil {
				// Save errors and continue
				errs = multierror.Append(errs, err)
			}
		}
	}

	// Then remount as RO
	for _, r := range cleanup.Remount {
		if notMounted, _ := u.Config.Mounter.IsLikelyNotMountPoint(r.Path); !notMounted {
			u.Debug("[Cleanup] Remount %s", r.Path)
			err := u.Config.Mounter.Mount(r.Device, r.Path, r.Type, []string{"remount", "ro"})
			if err != nil {
				// Save errors and continue
				errs = multierror.Append(errs, err)
			}
		}
	}

	if errs != nil {
		if originalError != nil {
			// Log errors but return the original error
			u.Error("Found errors while cleaning up: %s", errs)
			return originalError
		}
		return errs
	} else {
		if originalError != nil {
			return originalError
		}
		return nil
	}
}

// getTargetAndSource finds our the target and source for the upgrade
func (u *UpgradeAction) getTargetAndSource() (string, v1.InstallUpgradeSource) {
	upgradeSource := v1.InstallUpgradeSource{Source: constants.UpgradeSource, IsChannel: true}
	upgradeTarget := constants.UpgradeActive

	// if upgrade_recovery==true then it upgrades only the recovery
	// if upgrade_recovery==false then it upgrades only the active
	// default is active
	if u.Config.RecoveryUpgrade {
		u.Debug("Upgrading recovery")
		upgradeTarget = constants.UpgradeRecovery
	}

	// if channel_upgrades==true then it picks the default image from /etc/cos-upgrade-image
	// this means, it gets the UPGRADE_IMAGE(default system/cos) from the luet repo configured on the system
	if u.Config.ChannelUpgrades {
		u.Debug("Source is channel-upgrades")
		upgradeSource.Source = u.Config.UpgradeImage // Loaded from /etc/cos-upgrade-image
	} else {
		// if channel_upgrades==false then
		// if docker-image -> upgrade from image directly, ignores release_channel and pulls the given image directly
		if u.Config.DockerImg != "" {
			u.Debug("Source is docker image: %s", u.Config.DockerImg)
			upgradeSource = v1.InstallUpgradeSource{Source: u.Config.DockerImg, IsDocker: true}
		}
		// if directory -> upgrade from dir directly, ignores release_channel and uses the given directory
		if u.Config.DirectoryUpgrade != "" {
			u.Debug("Source is directory: %s", u.Config.DirectoryUpgrade)
			upgradeSource = v1.InstallUpgradeSource{Source: u.Config.DirectoryUpgrade, IsDir: true}
		}
	}
	return upgradeTarget, upgradeSource
}
