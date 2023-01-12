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
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	mountUtils "k8s.io/mount-utils"

	"github.com/rancher/elemental-cli/cmd/config"
	"github.com/rancher/elemental-cli/pkg/action"
	elementalError "github.com/rancher/elemental-cli/pkg/error"
	"github.com/rancher/elemental-cli/pkg/utils"
)

var outputAllowed = []string{"azure", "gce"}

// NewConvertDisk returns a new instance of the convert-disk subcommand and appends it to
// the root command. requireRoot is to initiate it with or without the CheckRoot
// pre-run check. This method is mostly used for testing purposes.
func NewConvertDisk(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "convert-disk RAW_DISK",
		Short: fmt.Sprintf("converts between a raw disk and a cloud operator disk image (%s)", strings.Join(outputAllowed, ",")),
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			mounter := &mountUtils.FakeMounter{}

			cfg, err := config.ReadConfigBuild(viper.GetString("config-dir"), cmd.Flags(), mounter)
			if err != nil {
				return elementalError.NewFromError(err, elementalError.ReadingBuildConfig)
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			imgType, _ := cmd.Flags().GetString("type")
			keepImage, _ := cmd.Flags().GetBool("keep-source")
			rawDisk := args[0]

			if exists, _ := utils.Exists(cfg.Fs, rawDisk); !exists {
				msg := fmt.Sprintf("Raw image %s doesnt exist", rawDisk)
				cfg.Logger.Errorf(msg)
				return elementalError.New(msg, elementalError.StatFile)
			}

			switch imgType {
			case "azure":
				err = action.Raw2Azure(rawDisk, cfg.Fs, cfg.Logger, keepImage)
			case "gce":
				err = action.Raw2Gce(rawDisk, cfg.Fs, cfg.Logger, keepImage)
			}

			return err
		},
	}
	root.AddCommand(c)
	imgType := newEnumFlag(outputAllowed, "azure")
	c.Flags().VarP(imgType, "type", "t", "Type of image to create")
	c.Flags().Bool("keep-source", false, "Keep the source image, otherwise it will delete it once transformed.")
	return c
}

// register the subcommand into rootCmd
var _ = NewConvertDisk(rootCmd, false)
