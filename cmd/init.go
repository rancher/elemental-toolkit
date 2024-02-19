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
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rancher/elemental-toolkit/v2/cmd/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/features"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
)

func InitCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "init FEATURES",
		Short: "Initialize container image for booting",
		Long: "Init a container image with elemental configuration\n\n" +
			"FEATURES - should be provided as a comma-separated list of features to install.\n" +
			"    Available features: " + strings.Join(features.All, ",") + "\n" +
			"    Defaults to all",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), v2.NewDummyMounter())
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			cmd.SilenceUsage = true
			spec, err := config.ReadInitSpec(cfg, cmd.Flags())
			if err != nil {
				cfg.Logger.Errorf("Error reading spec: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			if len(args) == 0 || args[0] == "all" {
				spec.Features = features.All
			} else {
				spec.Features = strings.Split(args[0], ",")
			}

			cfg.Logger.Infof("Initializing system...")
			return action.RunInit(cfg, spec)
		},
	}
	root.AddCommand(c)
	c.Flags().Bool("mkinitrd", true, "Run dracut to generate initramdisk")
	c.Flags().BoolP("force", "f", false, "Force run")
	return c
}

var _ = InitCmd(rootCmd)
