/*
Copyright © 2022 - 2025 SUSE LLC

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
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/cmd/config"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
)

func NewPullImageCmd(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "pull-image IMAGE DESTINATION",
		Short: "Pull remote image to local file",
		Args:  cobra.ExactArgs(2),
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), &mount.FakeMounter{})
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			image := args[0]
			destination, err := filepath.Abs(args[1])
			if err != nil {
				cfg.Logger.Errorf("Invalid path %s", destination)
				return elementalError.NewFromError(err, elementalError.StatFile)
			}

			local, err := cmd.Flags().GetBool("local")
			if err != nil {
				cfg.Logger.Errorf("Invalid local-flag %s", err.Error())
				return elementalError.NewFromError(err, elementalError.ReadingBuildConfig)
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			cfg.Logger.Infof("Pulling image %s platform %s", image, cfg.Platform.String())

			e := v1.OCIImageExtractor{}
			if err = e.ExtractImage(image, destination, cfg.Platform.String(), local); err != nil {
				cfg.Logger.Error(err.Error())
				return elementalError.NewFromError(err, elementalError.UnpackImage)
			}

			return nil
		},
	}
	root.AddCommand(c)
	addPlatformFlags(c)
	addLocalImageFlag(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewPullImageCmd(rootCmd, true)
