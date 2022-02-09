/*
Copyright © 2022 SUSE LLC

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
	"errors"
	"github.com/rancher-sandbox/elemental/cmd/config"
	"github.com/rancher-sandbox/elemental/pkg/action"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
	"os/exec"
)

// upgradeCmd represents the upgrade command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "upgrade the system",
	Args:  cobra.ExactArgs(0),
	PreRun: func(cmd *cobra.Command, args []string) {
		// We bind the --directory into the DirectoryUpgrade value directly to have a more explicit var in the config
		viper.BindPFlag("DirectoryUpgrade", cmd.Flags().Lookup("directory"))
		// We bind the --recovery flag into RecoveryUpgrade value to have a more explicit var in the config
		viper.BindPFlag("RecoveryUpgrade", cmd.Flags().Lookup("recovery"))
		// bind the rest of the flags into their direct values as they are mapped 1to1
		viper.BindPFlags(cmd.Flags())
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := exec.LookPath("mount")
		if err != nil {
			return err
		}
		mounter := mount.New(path)

		cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), mounter)

		if err != nil {
			cfg.Logger.Errorf("Error reading config: %s\n", err)
		}

		// Init luet
		action.SetupLuet(cfg)

		// docker-image and directory are mutually exclusive. Can't have your cake and eat it too.
		if viper.GetString("docker-image") != "" && viper.GetString("directory") != "" {
			msg := "flags docker-image and directory are mutually exclusive, please only set one of them"
			return errors.New(msg)
		}

		if cfg.DockerImg != "" || cfg.DirectoryUpgrade != "" {
			// Force channel upgrades to be false, because as its loaded from the config files,
			// it will probably always be set to true due to it being the default value
			cfg.ChannelUpgrades = false
		}
		// Set this after parsing of the flags, so it fails on parsing and prints usage properly
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

		upgrade := action.NewUpgradeAction(cfg)
		err = upgrade.Run()
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().String("docker-image", "", "Install a specified container image")
	upgradeCmd.Flags().String("directory", "", "Use directory as source to install from")
	upgradeCmd.Flags().Bool("recovery", false, "Upgrade the recovery")
	upgradeCmd.Flags().Bool("no-verify", false, "Disable mtree verification")
	upgradeCmd.Flags().Bool("cosign", false, "Disable cosign verification")
	upgradeCmd.Flags().Bool("strict", false, "Fail on any errors")
	upgradeCmd.Flags().BoolP("reboot", "", false, "Reboot the system after install")
	upgradeCmd.Flags().BoolP("poweroff", "", false, "Shutdown the system after install")
}
