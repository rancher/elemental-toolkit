package utils

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
	"os"
	"path/filepath"
)

func ReadConfigBuild(path...string)  error {
	var cfg string
	switch len(path) {
	case 1:
		cfg = path[0]
	default:
		cfg = "."
	}

	viper.AddConfigPath(cfg)
	viper.SetConfigType("yaml")
	viper.SetConfigName("manifest.yaml")

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	// Set the prefix for vars so we get only the ones starting with ELEMENTAL
	viper.SetEnvPrefix("ELEMENTAL")

	viper.AutomaticEnv() // read in environment variables that match
	return nil
}

func ReadConfigRun(args...string)  error {
	var cfg, cfgExtra string
	// Kind of need this basically for testing, so we can point at our test dir :/
	switch len(args) {
	case 1:
		cfg = args[0]
		cfgExtra = "/etc/elemental/config.d/"
	case 2:
		cfg = args[0]
		cfgExtra = args[1]
	default:
		// default paths
		cfg = "/etc/elemental/"
		cfgExtra = "/etc/elemental/config.d/"
	}


	viper.AddConfigPath(cfg)
	viper.SetConfigType("yaml")
	viper.SetConfigName("config.yaml")
	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	if _, err = os.Stat(cfgExtra); err == nil {
		viper.AddConfigPath(cfgExtra)
		err = filepath.WalkDir(cfgExtra, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() == false {
				viper.SetConfigName(d.Name())
				cobra.CheckErr(viper.MergeInConfig())
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Set the prefix for vars so we get only the ones starting with ELEMENTAL
	viper.SetEnvPrefix("ELEMENTAL")

	// If we expect to override complex keys in the config, i.e. configs that are nested, we probably need to manually do
	// the env stuff ourselves, as this will only match keys in the config root
	viper.AutomaticEnv() // read in environment variables that match
	return nil
}