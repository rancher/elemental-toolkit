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
	eleError "github.com/rancher/elemental-cli/pkg/error"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	mountUtils "k8s.io/mount-utils"

	"github.com/rancher/elemental-cli/cmd/config"
	"github.com/rancher/elemental-cli/pkg/action"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
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
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			mounter := &mountUtils.FakeMounter{}

			flags := cmd.Flags()
			cfg, err := config.ReadConfigBuild(viper.GetString("config-dir"), flags, mounter)
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

			spec, err := config.ReadBuildDisk(cfg, flags)
			if err != nil {
				cfg.Logger.Errorf("invalid install command setup %v", err)
				return eleError.NewFromError(err, eleError.ReadingBuildDiskConfig)
			}

			// Get the spec for the arch only
			var specArch *v1.RawDiskArchEntry
			if cfg.Arch == "x86_64" {
				specArch = spec.X86_64
			} else {
				specArch = spec.Arm64
			}

			// TODO map these to buildconfig and rawdisk structs, so they
			// are directly unmarshaled and there is no need handle them here
			imgType, _ := flags.GetString("type")
			output, _ := flags.GetString("output")
			oemLabel, _ := flags.GetString("oem_label")
			recoveryLabel, _ := flags.GetString("recovery_label")

			// Only overwrite repos if some are defined, default repo is alredy there
			if len(cfg.Repos) > 0 {
				cfg.Config.Repos = cfg.Repos
			}

			if exists, _ := utils.Exists(cfg.Fs, output); exists {
				cfg.Logger.Errorf("Output file %s exists, refusing to continue", output)
				return eleError.NewFromError(err, eleError.OutFileExists)
			}

			return action.BuildDiskRun(cfg, specArch, imgType, oemLabel, recoveryLabel, output)
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
