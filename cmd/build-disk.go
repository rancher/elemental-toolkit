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
	"fmt"

	"github.com/rancher-sandbox/elemental/cmd/config"
	"github.com/rancher-sandbox/elemental/pkg/action"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
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
		Short: "Build a raw recovery image",
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

			cfg, err := config.ReadConfigBuild(configDir, mounter)
			if err != nil {
				return err
			}

			err = validateCosignFlags(cfg.Logger, cmd.Flags())
			if err != nil {
				return err
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them
			imgType, _ := cmd.Flags().GetString("type")
			output, _ := cmd.Flags().GetString("output")
			oemLabel, _ := cmd.Flags().GetString("oem_label")
			recoveryLabel, _ := cmd.Flags().GetString("recovery_label")

			entry := cfg.RawDisk[cfg.Arch]
			if entry == nil {
				// We didnt load anything from the config file, create empty map
				cfg.RawDisk = map[string]*v1.RawDiskArchEntry{
					cfg.Arch: {Repositories: nil, Packages: nil},
				}
			}

			// Set the repo depending on the arch we are building for
			var repos []v1.Repository
			for _, u := range cfg.RawDisk[cfg.Arch].Repositories {
				repos = append(repos, v1.Repository{URI: u.URI, Priority: constants.LuetDefaultRepoPrio})
			}
			cfg.Config.Repos = repos

			if exists, _ := utils.Exists(cfg.Fs, output); exists {
				cfg.Logger.Errorf("Output file %s exists, refusing to continue", output)
				return fmt.Errorf("output file %s exists, refusing to continue", output)
			}

			// Set defaults if they are empty
			if len(cfg.Config.Repos) == 0 {
				repo := constants.LuetDefaultRepoURI
				if cfg.Arch != "x86_64" {
					repo = fmt.Sprintf("%s-%s", repo, cfg.Arch)
				}
				cfg.Logger.Infof("Repositories are empty, setting default value: %s", repo)
				cfg.Config.Repos = append(cfg.Config.Repos, v1.Repository{URI: repo, Priority: constants.LuetDefaultRepoPrio})

				cfg.RawDisk[cfg.Arch].Repositories = cfg.Config.Repos
			}

			// Set defaults packages if empty
			if len(cfg.RawDisk[cfg.Arch].Packages) == 0 {
				defaultPackages := constants.GetBuildDiskDefaultPackages()
				var packages []v1.RawDiskPackage
				for pkg, target := range defaultPackages {
					packages = append(packages, v1.RawDiskPackage{Name: pkg, Target: target})
				}
				cfg.Logger.Infof("Packages are empty, setting default values: %+v", packages)
				cfg.RawDisk[cfg.Arch].Packages = packages
			}

			err = action.BuildDiskRun(cfg, imgType, oemLabel, recoveryLabel, output)
			if err != nil {
				return err
			}

			return nil
		},
	}
	root.AddCommand(c)
	imgType := newEnumFlag([]string{"raw", "azure", "gce"}, "raw")
	c.Flags().VarP(imgType, "type", "t", "Type of image to create")
	c.Flags().StringP("output", "o", "disk.raw", "Output file (Extension auto changes based of the image type)")
	c.Flags().String("oem_label", "COS_OEM", "Oem partition label")
	c.Flags().String("recovery_label", "COS_RECOVERY", "Recovery partition label")
	addArchFlags(c)
	addCosignFlags(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewBuildDisk(rootCmd, true)
