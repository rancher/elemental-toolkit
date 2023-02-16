/*
Copyright © 2022 - 2023 SUSE LLC

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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-cli/cmd/config"
	"github.com/rancher/elemental-cli/pkg/action"
	elementalError "github.com/rancher/elemental-cli/pkg/error"
)

func InitCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "init",
		Short: "Initialize container image for booting",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), &mount.FakeMounter{})
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			spec, err := config.ReadInitSpec(cfg, cmd.Flags())
			if err != nil {
				cfg.Logger.Errorf("Error reading spec: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			cfg.Logger.Infof("Initializing system...")
			return action.RunInit(cfg, spec)
		},
	}
	root.AddCommand(c)
	c.Flags().Bool("mkinitrd", true, "Run mkinitrd")
	c.Flags().BoolP("force", "f", false, "Force run")
	return c
}

var _ = InitCmd(rootCmd)
