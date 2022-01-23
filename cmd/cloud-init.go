/*
Copyright © 2021 SUSE LLC

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
	"io/ioutil"
	"os"

	"github.com/mudler/yip/pkg/schema"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// cloudInit represents the cloud-init command
var cloudInit = &cobra.Command{
	Use:   "cloud-init",
	Short: "elemental cloud-init",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := logrus.New()
		logger.SetOutput(os.Stdout)

		stage, _ := cmd.Flags().GetString("stage")
		dot, _ := cmd.Flags().GetBool("dotnotation")

		runner := v1.NewYipCloudInitRunner(logger)
		fromStdin := len(args) == 1 && args[0] == "-"

		if dot {
			runner.SetModifier(schema.DotNotationModifier)
		}

		if fromStdin {
			std, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			args = []string{string(std)}
		}

		return runner.Run(stage, args...)
	},
}

func init() {
	rootCmd.AddCommand(cloudInit)
	cloudInit.PersistentFlags().StringP("stage", "s", "default", "Stage to apply")
	cloudInit.PersistentFlags().BoolP("dotnotation", "d", false, "Parse input in dotnotation ( e.g. `stages.foo.name=..` ) ")
	viper.BindPFlags(cloudInit.Flags())
}
