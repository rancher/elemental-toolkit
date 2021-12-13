package config

import (
	"fmt"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
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

func ReadConfigRun(configDir string, logger v1.Logger) (*v1.RunConfig, error) {
	cfg := v1.NewRunConfig(v1.WithLogger(logger))

	cfgExtra := fmt.Sprintf("%s/config.d/", strings.TrimSuffix(configDir, "/"))

	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("config.yaml")
	// If a config file is found, read it in.
	viper.ReadInConfig()

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
	return cfg, nil
}
