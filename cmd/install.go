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
	"fmt"
	"github.com/rancher-sandbox/elemental-cli/pkg/action"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install DEVICE",
	Short: "elemental installer",
	Args: cobra.ExactArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		err := utils.ReadConfigRun(viper.GetString("config_dir"))
		if err != nil {
			fmt.Printf("Error reading config: %s\n", err)
		}

		// Should probably load whatever env vars we want to overload here and merge them into the viper configs
		// Note that vars with ELEMENTAL in front and that match entries in teh config (only one level deep) are overwritten automatically
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("VALUE: %s", viper.GetString("config_dir"))
		fmt.Println("Install called")
		install := action.NewInstallAction(args[0])
		err := install.Run()
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringP("config_dir", "c", "/etc/elemental/", "dir where the config config reside")
	viper.BindPFlags(installCmd.Flags())

}
