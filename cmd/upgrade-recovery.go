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
func NewUpgradeRecoveryCmd(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "upgrade-recovery",
		Short: "Upgrade the Recovery system",
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

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			spec, err := config.ReadUpgradeSpec(cfg, cmd.Flags(), true)
			if err != nil {
				cfg.Logger.Errorf("Invalid upgrade-recovery command setup %v", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			cfg.Logger.Infof("Upgrade Recovery called")
			upgrade, err := action.NewUpgradeRecoveryAction(cfg, spec, action.WithUpdateInstallState(true))
			if err != nil {
				cfg.Logger.Errorf("failed to initialize upgrade-recovery action: %v", err)
				return err
			}

			err = upgrade.Run()
			if err != nil {
				cfg.Logger.Errorf("upgrade-recovery command failed: %v", err)
			}

			return err
		},
	}
	root.AddCommand(c)
	addSnapshotLabelsFlag(c)
	addRecoverySystemFlag(c)
	addPowerFlags(c)
	addSquashFsCompressionFlags(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewUpgradeRecoveryCmd(rootCmd, true)
