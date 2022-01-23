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

package config

import (
	"fmt"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
	"k8s.io/mount-utils"
	"os"
	"path/filepath"
	"strings"
)

func ReadConfigBuild(configDir string) (*v1.BuildConfig, error) {
	cfg := &v1.BuildConfig{}
	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("manifest.yaml")

	// If a config file is found, read it in.
	viper.ReadInConfig()

	// Set the prefix for vars so we get only the ones starting with ELEMENTAL
	viper.SetEnvPrefix("ELEMENTAL")

	viper.AutomaticEnv() // read in environment variables that match
	// unmarshal all the vars into the config object
	viper.Unmarshal(cfg)
	return cfg, nil
}

func ReadConfigRun(configDir string, logger v1.Logger, mounter mount.Interface) (*v1.RunConfig, error) {
	cfg := v1.NewRunConfig(
		v1.WithLogger(logger),
		v1.WithMounter(mounter),
	)

	cfgDefault := []string{"/etc/os-release", "/etc/cos/config"}

	for _, c := range cfgDefault {
		if _, err := os.Stat(c); err == nil {
			cfg.Logger.Debug("Loading config file: %s", c)
			viper.SetConfigFile(c)
			viper.SetConfigType("env")
			cobra.CheckErr(viper.MergeInConfig())
		}
	}

	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("config.yaml")
	// If a config file is found, read it in.
	viper.MergeInConfig()

	// Load extra config files on configdir/config.d/ so we can override config values
	cfgExtra := fmt.Sprintf("%s/config.d/", strings.TrimSuffix(configDir, "/"))
	if _, err := os.Stat(cfgExtra); err == nil {
		viper.AddConfigPath(cfgExtra)
		err = filepath.WalkDir(cfgExtra, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() == false {
				viper.SetConfigName(d.Name())
				cobra.CheckErr(viper.MergeInConfig())
			}
			return nil
		})
	}

	// Set the prefix for vars so we get only the ones starting with ELEMENTAL
	viper.SetEnvPrefix("ELEMENTAL")

	// If we expect to override complex keys in the config, i.e. configs that are nested, we probably need to manually do
	// the env stuff ourselves, as this will only match keys in the config root
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv() // read in environment variables that match

	// unmarshal all the vars into the config object
	viper.Unmarshal(cfg)

	// Set debug level
	if cfg.Debug {
		cfg.Logger.SetLevel(v1.DebugLevel())
	}
	return cfg, nil
}
