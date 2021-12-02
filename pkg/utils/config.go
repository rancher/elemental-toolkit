package utils

import (
	"fmt"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ReadConfigBuild(configDir string)  (*config.BuildConfig, error) {
	cfg := &config.BuildConfig{}
	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("manifest.yaml")

	// If a config file is found, read it in.
	_ = viper.ReadInConfig()

	// Set the prefix for vars so we get only the ones starting with ELEMENTAL
	viper.SetEnvPrefix("ELEMENTAL")

	viper.AutomaticEnv() // read in environment variables that match
	// unmarshal all the vars into the config object
	_ = viper.Unmarshal(cfg)
	return cfg, nil
}

func ReadConfigRun(configDir string)  (*config.RunConfig, error) {
	cfg := &config.RunConfig{}

	cfgExtra := fmt.Sprintf("%s/config.d/", strings.TrimSuffix(configDir, "/"))

	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("config.yaml")
	// If a config file is found, read it in.
	_ = viper.ReadInConfig()

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
	_ = viper.Unmarshal(cfg)
	return cfg, nil
}