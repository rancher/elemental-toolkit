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

	"github.com/rancher/elemental-toolkit/v2/cmd/config"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func NewStateCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "state",
		Args:  cobra.ExactArgs(0),
		Short: "Shows the install state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			viper.SetDefault("quiet", true) // Prevents any other writes to stdout
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

			// Unmarshal and remashal install state for sanity check.
			state, err := cfg.LoadInstallState()
			if err != nil {
				cfg.Logger.Errorf("Error reading installation state: %s\n", err)
				return elementalError.NewFromError(err, elementalError.DisplayingInstallationState)
			}
			stateBytes, err := yaml.Marshal(state)
			if err != nil {
				cfg.Logger.Errorf("Error marshalling installation state: %s\n", err)
				return elementalError.NewFromError(err, elementalError.DisplayingInstallationState)
			}

			if _, err := cmd.OutOrStdout().Write(stateBytes); err != nil {
				cfg.Logger.Errorf("Error writing installation state on stdout: %s\n", err)
				return elementalError.NewFromError(err, elementalError.DisplayingInstallationState)
			}
			return nil
		},
	}
	root.AddCommand(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewStateCmd(rootCmd)
