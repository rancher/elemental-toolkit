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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/cmd/config"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

func NewRunStage(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "run-stage STAGE",
		Short: "Run stage from cloud-init",
		Args:  cobra.MinimumNArgs(1),
		PreRun: func(_ *cobra.Command, _ []string) {

		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), &mount.FakeMounter{})
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			err = utils.RunStage(&cfg.Config, args[0], cfg.Strict, cfg.CloudInitPaths...)
			return elementalError.NewFromError(err, elementalError.CloudInitRunStage)
		},
	}
	root.AddCommand(c)
	c.Flags().Bool("strict", false, "Set strict checking for errors, i.e. fail if errors were found")
	return c
}

// register the subcommand into rootCmd
var _ = NewRunStage(rootCmd)
