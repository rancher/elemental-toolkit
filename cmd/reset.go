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

func NewResetCmd(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "reset",
		Short: "Reset OS",
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
				return elementalError.NewFromError(err, elementalError.ReadingInstallUpgradeFlags)
			}

			// Adapt 'docker-image' and 'directory'  deprecated flags to 'system' syntax
			adaptDockerImageAndDirectoryFlagsToSystem(cmd.Flags())

			cmd.SilenceUsage = true
			spec, err := config.ReadResetSpec(cfg, cmd.Flags())
			if err != nil {
				cfg.Logger.Errorf("invalid reset command setup %v", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			cfg.Logger.Infof("Reset called")
			reset, err := action.NewResetAction(cfg, spec)
			if err != nil {
				cfg.Logger.Errorf("failed to initialize reset action: %v", err)
				return err
			}

			err = reset.Run()
			if err != nil {
				cfg.Logger.Errorf("reset command failed: %v", err)
			}

			return err
		},
	}
	root.AddCommand(c)
	c.Flags().StringSliceP("cloud-init", "c", []string{}, "Cloud-init config files")
	c.Flags().BoolP("reset-persistent", "", false, "Clear persistent partitions")
	c.Flags().BoolP("reset-oem", "", false, "Clear OEM partitions")
	c.Flags().Bool("disable-boot-entry", false, "Dont create an EFI entry for the system install.")
	c.Flags().StringSlice("cloud-init-paths", []string{}, "Cloud-init config files to run during reset")
	addResetFlags(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewResetCmd(rootCmd, true)
