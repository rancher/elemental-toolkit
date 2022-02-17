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
	"github.com/rancher-sandbox/elemental/pkg/config"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/fs"
	"io/ioutil"
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

func ReadConfigRun(configDir string, mounter mount.Interface) (*v1.RunConfig, error) {
	cfg := config.NewRunConfig(
		v1.WithLogger(v1.NewLogger()),
		v1.WithMounter(mounter),
	)

	// Set debug level
	if viper.GetBool("debug") {
		cfg.Logger.SetLevel(v1.DebugLevel())
	}

	// Set formatter so both file and stdout format are equal
	cfg.Logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:      true,
		DisableColors:    false,
		DisableTimestamp: false,
		FullTimestamp:    true,
	})

	// Logfile
	logfile := viper.GetString("logfile")
	if logfile != "" {
		o, err := cfg.Fs.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fs.ModePerm)

		if err != nil {
			cfg.Logger.Errorf("Could not open %s for logging to file: %s", logfile, err.Error())
		}

		if viper.GetBool("quiet") { // if quiet is set, only set the log to the file
			cfg.Logger.SetOutput(o)
		} else { // else set it to both stdout and the file
			mw := io.MultiWriter(os.Stdout, o)
			cfg.Logger.SetOutput(mw)
		}
	} else { // no logfile
		if viper.GetBool("quiet") { // quiet is enabled so discard all logging
			cfg.Logger.SetOutput(ioutil.Discard)
		} else { // default to stdout
			cfg.Logger.SetOutput(os.Stdout)
		}
	}

	cfgDefault := []string{"/etc/os-release", "/etc/cos/config", "/etc/cos-upgrade-image"}

	for _, c := range cfgDefault {
		if _, err := os.Stat(c); err == nil {
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

	// Manually bind public key env variable as it uses a different name in config files or flags.
	viper.BindEnv("CosingPubKey", "COSIGN_PUBLIC_KEY_LOCATION")

	viper.AutomaticEnv() // read in environment variables that match

	// unmarshal all the vars into the config object
	viper.Unmarshal(cfg)

	return cfg, nil
}
