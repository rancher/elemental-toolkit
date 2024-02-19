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
	"io"
	"os"

	"github.com/rancher/elemental-toolkit/v2/cmd/config"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"

	"github.com/rancher/yip/pkg/schema"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCloudInitCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "cloud-init",
		Short: "Run cloud-init",
		Args:  cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, _ []string) {
			_ = viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), v2.NewDummyMounter())
			if err != nil {
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			stage, _ := cmd.Flags().GetString("stage")
			dot, _ := cmd.Flags().GetBool("dotnotation")

			fromStdin := len(args) == 1 && args[0] == "-"

			if dot {
				cfg.CloudInitRunner.SetModifier(schema.DotNotationModifier)
			}

			if fromStdin {
				std, err := io.ReadAll(os.Stdin)
				if err != nil {
					return elementalError.NewFromError(err, elementalError.ReadFile)
				}

				args = []string{string(std)}
			}

			err = cfg.CloudInitRunner.Run(stage, args...)
			return elementalError.NewFromError(err, elementalError.CloudInitRunStage)
		},
	}
	root.AddCommand(c)
	c.PersistentFlags().StringP("stage", "s", "default", "Stage to apply")
	c.PersistentFlags().BoolP("dotnotation", "d", false, "Parse input in dotnotation ( e.g. `stages.foo.name=..` ) ")
	return c
}

// register the subcommand into rootCmd
var _ = NewCloudInitCmd(rootCmd)
