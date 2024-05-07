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

package cmd

import (
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/v2/cmd/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
)

func NewRelabelCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "relabel",
		Short: "Set correct SELinux file labels during boot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Some extended attributes are lost on copy-up bsc#1210690.
			// When/if this bug is solved, we could try only writing the
			// relabel file to /run/systemd/extra-relabel.d in the mount
			// command and remove this command+feature entirely.

			mounter := mount.New(constants.MountBinary)

			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), mounter)
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			err = godotenv.Load(constants.MountLayoutPath)
			if err != nil {
				cfg.Logger.Debugf("Error loading %s: %s", constants.MountLayoutPath, err.Error())
			}

			cmd.SilenceUsage = true
			spec, err := config.ReadMountSpec(cfg, cmd.Flags())
			if err != nil {
				cfg.Logger.Errorf("Error reading spec: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			if spec.Disable {
				cfg.Logger.Info("Mounting disabled, exiting")
				return nil
			}

			cfg.Logger.Info("Relabeling system...")
			return action.SelinuxRelabel(cfg, spec)
		},
	}
	root.AddCommand(c)
	return c
}

var _ = NewRelabelCmd(rootCmd)
