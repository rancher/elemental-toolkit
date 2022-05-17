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
	"os/exec"

	"github.com/rancher-sandbox/elemental/cmd/config"
	"github.com/rancher-sandbox/elemental/pkg/action"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
)

// NewBuildISO returns a new instance of the buid-iso subcommand and appends it to
// the root command. requireRoot is to initiate it with or without the CheckRoot
// pre-run check. This method is mostly used for testing purposes.
func NewBuildISO(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "build-iso IMAGE",
		Short: "builds bootable installation media ISOs",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			bindSquashFsCompressionFlags(cmd)
			_ = viper.BindPFlags(cmd.Flags())
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := exec.LookPath("mount")
			if err != nil {
				return err
			}
			mounter := mount.New(path)

			cfg, err := config.ReadConfigBuild(viper.GetString("config-dir"), mounter)
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
			}

			if len(args) >= 1 {
				cfg.ISO.RootFS = []string{args[0]}
			}

			err = validateCosignFlags(cfg.Logger)
			if err != nil {
				return err
			}

			if len(cfg.ISO.RootFS) == 0 {
				return fmt.Errorf("no rootfs image source provided")
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			oRootfs, _ := cmd.Flags().GetString("overlay-rootfs")
			oUEFI, _ := cmd.Flags().GetString("overlay-uefi")
			oISO, _ := cmd.Flags().GetString("overlay-iso")
			repoURIs, _ := cmd.Flags().GetStringArray("repo")
			label, _ := cmd.Flags().GetString("label")

			if label != "" {
				cfg.ISO.Label = label
			}

			if oRootfs != "" {
				if ok, err := utils.Exists(cfg.Fs, oRootfs); ok {
					cfg.ISO.RootFS = append(cfg.ISO.RootFS, oRootfs)
				} else {
					cfg.Logger.Errorf("Invalid value for overlay-rootfs")
					return fmt.Errorf("Invalid path '%s': %v", oRootfs, err)
				}
			}
			if oUEFI != "" {
				if ok, err := utils.Exists(cfg.Fs, oUEFI); ok {
					cfg.ISO.UEFI = append(cfg.ISO.UEFI, oUEFI)
				} else {
					cfg.Logger.Errorf("Invalid value for overlay-uefi")
					return fmt.Errorf("Invalid path '%s': %v", oUEFI, err)
				}
			}
			if oISO != "" {
				if ok, err := utils.Exists(cfg.Fs, oISO); ok {
					cfg.ISO.Image = append(cfg.ISO.Image, oISO)
				} else {
					cfg.Logger.Errorf("Invalid value for overlay-iso")
					return fmt.Errorf("Invalid path '%s': %v", oISO, err)
				}
			}

			if len(cfg.Repos) == 0 {
				repo := constants.LuetDefaultRepoURI
				if cfg.Arch != "x86_64" {
					repo = fmt.Sprintf("%s-%s", constants.LuetDefaultRepoURI, cfg.Arch)
				}
				cfg.Repos = []v1.Repository{{
					Name:     "cos",
					Type:     "docker",
					URI:      repo,
					Priority: constants.LuetDefaultRepoPrio,
				}}
			}

			for _, u := range repoURIs {
				cfg.Repos = append(cfg.Repos, v1.Repository{URI: u, Priority: constants.LuetRepoMaxPrio})
			}

			err = action.BuildISORun(cfg)
			if err != nil {
				return err
			}

			return nil
		},
	}
	root.AddCommand(c)
	c.Flags().StringP("name", "n", "", "Basename of the generated ISO file")
	c.Flags().StringP("output", "o", "", "Output directory (defaults to current directory)")
	c.Flags().Bool("date", false, "Adds a date suffix into the generated ISO file")
	c.Flags().String("overlay-rootfs", "", "Path of the overlayed rootfs data")
	c.Flags().String("overlay-uefi", "", "Path of the overlayed uefi data")
	c.Flags().String("overlay-iso", "", "Path of the overlayed iso data")
	c.Flags().String("label", "", "Label of the ISO volume")
	c.Flags().StringArray("repo", []string{}, "A repository URI for luet. Can be repeated to add more than one source.")
	addArchFlags(c)
	addCosignFlags(c)
	addSquashFsCompressionFlags(c)
	addLocalImageFlag(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewBuildISO(rootCmd, true)
