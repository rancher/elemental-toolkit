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
	"github.com/rancher-sandbox/elemental/cmd/config"
	"github.com/rancher-sandbox/elemental/pkg/action"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
	"os/exec"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install DEVICE",
	Short: "elemental installer",
	Args:  cobra.ExactArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
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

		if err := validateInstallFlags(cfg.Logger); err != nil {
			return err
		}

		// Should probably load whatever env vars we want to overload here and merge them into the viper configs
		// Note that vars with ELEMENTAL in front and that match entries in the config (only one level deep) are overwritten automatically
		cfg.Target = args[0]

		err = action.InstallSetup(cfg)
		if err != nil {
			return err
		}
		cmd.SilenceUsage = true

		cfg.Logger.Infof("Install called")

		err = action.InstallRun(cfg)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringP("cloud-init", "c", "", "Cloud-init config file")
	installCmd.Flags().StringP("iso", "i", "", "Performs an installation from the ISO url")
	installCmd.Flags().StringP("partition-layout", "p", "", "Partitioning layout file")
	installCmd.Flags().BoolP("no-format", "", false, "Don’t format disks. It is implied that COS_STATE, COS_RECOVERY, COS_PERSISTENT, COS_OEM are already existing")
	installCmd.Flags().BoolP("force-efi", "", false, "Forces an EFI installation")
	installCmd.Flags().BoolP("force-gpt", "", false, "Forces a GPT partition table")
	installCmd.Flags().BoolP("tty", "", false, "Add named tty to grub")
	installCmd.Flags().BoolP("force", "", false, "Force install")
	addSharedInstallUpgradeFlags(installCmd)
	addCosignFlags(installCmd)
	addPowerFlags(installCmd)
}
