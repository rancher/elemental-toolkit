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

package cmd

import (
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rancher/elemental-toolkit/v2/cmd/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

// NewInstallCmd returns a new instance of the install subcommand and appends it to
// the root command. requireRoot is to initiate it with or without the CheckRoot
// pre-run check. This method is mostly used for testing purposes.
func NewInstallCmd(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "install DEVICE",
		Short: "Elemental installer",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := exec.LookPath("mount")
			if err != nil {
				return err
			}
			mounter := types.NewMounter(path)

			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), mounter)
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			if err := validateInstallUpgradeFlags(cfg.Logger, cmd.Flags()); err != nil {
				cfg.Logger.Errorf("Error reading install/upgrade flags: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingInstallUpgradeFlags)
			}

			// Manage deprecated flags
			// Adapt 'docker-image' and 'directory'  deprecated flags to 'system' syntax
			adaptDockerImageAndDirectoryFlagsToSystem(cmd.Flags())

			cmd.SilenceUsage = true
			spec, err := config.ReadInstallSpec(cfg, cmd.Flags())
			if err != nil {
				cfg.Logger.Errorf("invalid install command setup %v", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			if len(args) == 1 {
				spec.Target = args[0]
			}

			if spec.Target == "" {
				return elementalError.New("at least a target device must be supplied", elementalError.InvalidTarget)
			}

			// If the device path is a symlink, resolve it (e.g., /dev/disk/by-path/platform-fe340000.mmc-part1)
			spec.Target, _ = utils.ResolveLink(cfg.Fs, spec.Target, "/", constants.MaxLinkDepth)

			cfg.Logger.Infof("Install called")
			install, err := action.NewInstallAction(cfg, spec)
			if err != nil {
				cfg.Logger.Errorf("failed to initialize install action: %v", err)
				return err
			}
			err = install.Run()
			if err != nil {
				cfg.Logger.Errorf("install command failed: %v", err)
			}
			return err
		},
	}
	firmType := newEnumFlag([]string{types.EFI}, types.EFI)
	pTableType := newEnumFlag([]string{types.GPT}, types.GPT)
	snapshotterType := newEnumFlag(
		[]string{constants.LoopDeviceSnapshotterType, constants.BtrfsSnapshotterType},
		constants.LoopDeviceSnapshotterType,
	)

	root.AddCommand(c)
	c.Flags().StringSliceP("cloud-init", "c", []string{}, "Cloud-init config files")
	c.Flags().StringP("iso", "i", "", "Performs an installation from the ISO url")
	c.Flags().Bool("no-format", false, "Don’t format disks. It is implied that COS_STATE, COS_RECOVERY, COS_PERSISTENT, COS_OEM are already existing")

	c.Flags().Var(firmType, "firmware", "Firmware to install, only 'efi' is currently supported")
	_ = c.Flags().MarkDeprecated("firmware", "'firmware' is deprecated. only efi firmware is supported.")

	c.Flags().Var(pTableType, "part-table", "Partition table type to use, only GPT type is currently supported")
	_ = c.Flags().MarkDeprecated("part-table", "'part-table' is deprecated. only GPT type is supported.")

	c.Flags().Bool("force", false, "Force install")
	c.Flags().Bool("eject-cd", false, "Try to eject the cd on reboot, only valid if booting from iso")
	c.Flags().Bool("disable-boot-entry", false, "Dont create an EFI entry for the system install.")
	c.Flags().Var(snapshotterType, "snapshotter.type", "Sets the snapshotter type to install")
	c.Flags().StringSlice("cloud-init-paths", []string{}, "Cloud-init config files to run during install")
	addSharedInstallUpgradeFlags(c)
	addLocalImageFlag(c)
	addPlatformFlags(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewInstallCmd(rootCmd, true)
