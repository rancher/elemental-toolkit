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
	"errors"
	"github.com/rancher-sandbox/elemental/cmd/config"
	"github.com/rancher-sandbox/elemental/pkg/action"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
	"os/exec"
)

// resetCmd represents the install command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "elemental reset OS",
	Args:  cobra.ExactArgs(0),
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

		err = errors.New("Invalid options")
		if viper.GetBool("reboot") && viper.GetBool("poweroff") {
			cfg.Logger.Errorf("'reboot' and 'poweroff' are mutually exclusive options")
			return err
		}

		if viper.GetString("cosign-key") != "" && !viper.GetBool("cosign") {
			cfg.Logger.Errorf("'cosign-key' requires 'cosing' option to be enabled")
			return err
		}

		if viper.GetBool("cosign") && viper.GetString("cosign-key") == "" {
			cfg.Logger.Warnf("No 'cosign-key' option set, keyless cosign verification is experimental")
		}

		cmd.SilenceUsage = true
		err = action.ResetSetup(cfg)
		if err != nil {
			return err
		}

		cfg.Logger.Infof("Reset called")

		return action.ResetRun(cfg)
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().StringP("docker-image", "", "", "Reset using a specified container image")
	resetCmd.Flags().StringP("directory", "d", "", "Reset from a local root tree")
	resetCmd.Flags().BoolP("no-verify", "", false, "Disable mtree checksum verification (requires images manifests generated with mtree separately)")
	resetCmd.Flags().BoolP("cosign", "", false, "Enable cosign verification (requires images with signatures)")
	resetCmd.Flags().StringP("cosign-key", "", "", "Sets the URL of the public key to be used by cosign validation")
	resetCmd.Flags().BoolP("strict", "", false, "Enable strict check of hooks (They need to exit with 0)")
	resetCmd.Flags().BoolP("tty", "", false, "Add named tty to grub")
	resetCmd.Flags().BoolP("reset-persistent", "", false, "Clear persistent partitions")
	resetCmd.Flags().BoolP("reboot", "", false, "Reboot the system after install")
	resetCmd.Flags().BoolP("poweroff", "", false, "Shutdown the system after install")
}
