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
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

// NewUpgradeCmd returns a new instance of the upgrade subcommand and appends it to
// the root command. requireRoot is to initiate it with or without the CheckRoot
// pre-run check. This method is mostly used for testing purposes.
func NewUpgradeCmd(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the system",
		Args:  cobra.ExactArgs(0),
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},

		RunE: func(cmd *cobra.Command, _ []string) error {
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

			// Adapt 'docker-image' and 'directory'  deprecated flags to 'system' syntax
			adaptDockerImageAndDirectoryFlagsToSystem(cmd.Flags())

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			spec, err := config.ReadUpgradeSpec(cfg, cmd.Flags(), false)
			if err != nil {
				cfg.Logger.Errorf("Invalid upgrade command setup %v", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			cfg.Logger.Infof("Upgrade called")
			upgrade, err := action.NewUpgradeAction(cfg, spec)
			if err != nil {
				cfg.Logger.Errorf("failed to initialize upgrade action: %v", err)
				return err
			}

			err = upgrade.Run()
			if err != nil {
				cfg.Logger.Errorf("upgrade command failed: %v", err)
			}

			return err
		},
	}
	root.AddCommand(c)
	c.Flags().Bool("recovery", false, "Upgrade recovery image too")
	c.Flags().Bool("bootloader", false, "Reinstall bootloader during the upgrade")
	c.Flags().StringSlice("cloud-init-paths", []string{}, "Cloud-init config files to run during upgrade")
	addSharedInstallUpgradeFlags(c)
	addLocalImageFlag(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewUpgradeCmd(rootCmd, true)
