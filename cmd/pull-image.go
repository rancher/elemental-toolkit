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
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-cli/cmd/config"
	"github.com/rancher/elemental-cli/pkg/luet"
)

func NewPullImageCmd(root *cobra.Command, addCheckRoot bool) *cobra.Command {
	c := &cobra.Command{
		Use:   "pull-image IMAGE DESTINATION",
		Short: "Pull remote image to local file",
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if addCheckRoot {
				return CheckRoot()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), &mount.FakeMounter{})

			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
			}

			image := args[0]
			destination, err := filepath.Abs(args[1])
			if err != nil {
				cfg.Logger.Errorf("Invalid path %s", destination)
				return err
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			verify, _ := cmd.Flags().GetBool("verify")
			user, _ := cmd.Flags().GetString("auth-username")
			local, _ := cmd.Flags().GetBool("local")
			pass, _ := cmd.Flags().GetString("auth-password")
			authType, _ := cmd.Flags().GetString("auth-type")
			server, _ := cmd.Flags().GetString("auth-server-address")
			identity, _ := cmd.Flags().GetString("auth-identity-token")
			registryToken, _ := cmd.Flags().GetString("auth-registry-token")
			plugins, _ := cmd.Flags().GetStringArray("plugin")

			auth := &types.AuthConfig{
				Username:      user,
				Password:      pass,
				ServerAddress: server,
				Auth:          authType,
				IdentityToken: identity,
				RegistryToken: registryToken,
			}

			l := luet.NewLuet(luet.WithLogger(cfg.Logger), luet.WithAuth(auth), luet.WithPlugins(plugins...))
			l.VerifyImageUnpack = verify
			_, err = l.Unpack(destination, image, local)

			if err != nil {
				cfg.Logger.Error(err.Error())
				return err
			}

			return nil
		},
	}
	root.AddCommand(c)
	c.Flags().String("auth-username", "", "Username to authenticate to registry/notary")
	c.Flags().String("auth-password", "", "Password to authenticate to registry")
	c.Flags().String("auth-type", "", "Auth type")
	c.Flags().String("auth-server-address", "", "Authentication server address")
	c.Flags().String("auth-identity-token", "", "Authentication identity token")
	c.Flags().String("auth-registry-token", "", "Authentication registry token")
	c.Flags().Bool("verify", false, "Verify signed images to notary before to pull")
	c.Flags().StringArray("plugin", []string{}, "A list of runtime plugins to load. Can be repeated to add more than one plugin")
	addLocalImageFlag(c)
	return c
}

// register the subcommand into rootCmd
var _ = NewPullImageCmd(rootCmd, true)
