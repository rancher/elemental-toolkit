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
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-cli/cmd/config"
	elementalError "github.com/rancher/elemental-cli/pkg/error"
)

func NewDerivativeCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:           "new FLAVOR",
		Short:         "Create skeleton Dockerfile for a derivative",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true, // Do not show usage on error
		SilenceErrors: true, // Do not propagate errors down the line, we control them
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), &mount.FakeMounter{})
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			flavor := strings.ToLower(args[0])
			if flavor != "opensuse" && flavor != "ubuntu" && flavor != "fedora" {
				cfg.Logger.Errorf("Unsupported flavor")
				return elementalError.New("unsupported flavor", elementalError.UnsupportedFlavor)
			}

			client := &getter.Client{
				Ctx:  context.Background(),
				Dst:  fmt.Sprintf("derivatives/%s", flavor),
				Dir:  true,
				Src:  "github.com/rancher/elemental-toolkit/examples/standard",
				Mode: getter.ClientModeDir,
				Detectors: []getter.Detector{
					&getter.GitHubDetector{},
				},
			}

			cfg.Logger.Infof("Downloading template...")
			err = client.Get()
			if err != nil {
				cfg.Logger.Errorf("Unable to create derivative")
				return elementalError.NewFromError(err, elementalError.DownloadFile)
			}

			cfg.Logger.Infof("New derivative created successfully")
			return nil
		},
	}
	root.AddCommand(c)
	c.Flags().String("arch", "", "X86_64 or aarch64 architectures")
	return c
}

// register the subcommand into rootCmd
var _ = NewDerivativeCmd(rootCmd)
