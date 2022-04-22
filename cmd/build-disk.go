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
	"github.com/rancher-sandbox/elemental/cmd/config"
	"github.com/rancher-sandbox/elemental/pkg/action"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	mountUtils "k8s.io/mount-utils"
)

// NewBuildDisk returns a new instance of the build-disk subcommand and appends it to
// the root command. requireRoot is to initiate it with or without the CheckRoot
// pre-run check. This method is mostly used for testing purposes.
func NewBuildDisk(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "build-disk",
		Short: "elemental build-disk",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			_ = viper.BindPFlags(cmd.Flags())
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			mounter := &mountUtils.FakeMounter{}

			// If configDir is empty try to get the manifest from current dir
			configDir := viper.GetString("config-dir")
			if configDir == "" {
				configDir = "."
			}

			cfg, err := config.ReadConfigBuild(configDir, mounter, true)
			if err != nil {
				return err
			}

			err = validateCosignFlags(cfg.Logger)
			if err != nil {
				return err
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them
			imgType, _ := cmd.Flags().GetString("type")
			archType, _ := cmd.Flags().GetString("arch")
			output, _ := cmd.Flags().GetString("output")
			oemLabel, _ := cmd.Flags().GetString("oem_label")
			recoveryLabel, _ := cmd.Flags().GetString("recovery_label")

			// Set the repo depending on the arch we are building for
			var repos []v1.Repository
			for _, u := range cfg.RawDisk[archType].Repositories {
				repos = append(repos, v1.Repository{URI: u.URI})
			}
			cfg.Config.Repos = repos

			err = action.BuildDiskRun(cfg, imgType, archType, oemLabel, recoveryLabel, output)
			if err != nil {
				return err
			}

			return nil
		},
	}
	root.AddCommand(c)
	imgType := newEnumFlag([]string{"raw"}, "raw")
	archType := newEnumFlag([]string{"x86_64", "aarch64", "odroid_c2"}, "x86_64")
	c.Flags().VarP(imgType, "type", "t", "Type of image to create")
	c.Flags().VarP(archType, "arch", "a", "Arch to build the image for")
	c.Flags().StringP("output", "o", "disk.raw", "Arch to build the image for")
	c.Flags().String("oem_label", "COS_OEM", "Oem partition label")
	c.Flags().String("recovery_label", "COS_RECOVERY", "Recovery partition label")
	addCosignFlags(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewBuildDisk(rootCmd, true)
