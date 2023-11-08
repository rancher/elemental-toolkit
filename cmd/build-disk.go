/*
Copyright © 2022 - 2023 SUSE LLC

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
	"os/exec"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	eleError "github.com/rancher/elemental-toolkit/pkg/error"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rancher/elemental-toolkit/cmd/config"
	"github.com/rancher/elemental-toolkit/pkg/action"
)

// NewBuildDisk returns a new instance of the build-disk subcommand and appends it to
// the root command. requireRoot is to initiate it with or without the CheckRoot
// pre-run check. This method is mostly used for testing purposes.
func NewBuildDisk(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "build-disk image",
		Short: "Build a disk image using the given image (experimental and subject to change)",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var cfg *v1.BuildConfig
			var spec *v1.DiskSpec
			var imgSource *v1.ImageSource

			defer func() {
				if cfg != nil && err != nil {
					cfg.Logger.Errorf("Woophs, something went terribly wrong: %s", err)
				}
			}()

			path, err := exec.LookPath("mount")
			if err != nil {
				return err
			}
			mounter := v1.NewMounter(path)

			flags := cmd.Flags()
			cfg, err = config.ReadConfigBuild(viper.GetString("config-dir"), flags, mounter)
			if err != nil {
				return eleError.NewFromError(err, eleError.ReadingBuildConfig)
			}

			err = validateCosignFlags(cfg.Logger, flags)
			if err != nil {
				return eleError.NewFromError(err, eleError.CosignWrongFlags)
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			spec, err = config.ReadBuildDisk(cfg, flags)
			if err != nil {
				cfg.Logger.Errorf("invalid install command setup %v", err)
				return eleError.NewFromError(err, eleError.ReadingBuildDiskConfig)
			}

			if len(args) > 0 {
				imgSource, err = v1.NewSrcFromURI(args[0])
				if err != nil {
					cfg.Logger.Errorf("not a valid image argument: %s", args[0])
					return elementalError.NewFromError(err, elementalError.IdentifySource)
				}
				spec.Recovery.Source = imgSource
			}

			builder := action.NewBuildDiskAction(cfg, spec)
			return builder.BuildDiskRun()
		},
	}
	root.AddCommand(c)
	imgType := newEnumFlag([]string{constants.RawType, constants.AzureType, constants.GCEType}, constants.RawType)
	c.Flags().StringP("name", "n", "", "Basename of the generated disk file")
	c.Flags().StringP("output", "o", "", "Output directory (defaults to current directory)")
	c.Flags().Bool("date", false, "Adds a date suffix into the generated disk file")
	c.Flags().Bool("expandable", false, "Creates an expandable image including only the recovery image")
	c.Flags().Bool("unprivileged", false, "Makes a build runnable within a non-privileged container, avoids mounting filesystems (experimental)")
	c.Flags().VarP(imgType, "type", "t", "Type of image to create")
	c.Flags().StringSliceP("cloud-init", "c", []string{}, "Cloud-init config files to include in disk")
	c.Flags().StringSlice("cloud-init-paths", []string{}, "Cloud-init config files to run during build")
	c.Flags().StringSlice("deploy-command", []string{"elemental", "--debug", "reset", "--reboot"}, "Deployment command for expandable images")
	addPlatformFlags(c)
	addLocalImageFlag(c)
	addSquashFsCompressionFlags(c)
	addCosignFlags(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewBuildDisk(rootCmd, true)
