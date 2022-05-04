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

package config

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher-sandbox/elemental/internal/version"
	"github.com/rancher-sandbox/elemental/pkg/config"
	"github.com/rancher-sandbox/elemental/pkg/luet"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
)

func ReadConfigBuild(configDir string, mounter mount.Interface) (*v1.BuildConfig, error) {
	logger := v1.NewLogger()
	arch := viper.GetString("arch")
	if arch == "" {
		arch = "x86_64"
	}

	cfg := config.NewBuildConfig(
		config.WithLogger(logger),
		config.WithMounter(mounter),
		config.WithLuet(luet.NewLuet(luet.WithLogger(logger))),
		config.WithArch(arch),
	)

	configLogger(cfg.Logger, cfg.Fs)

	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("manifest.yaml")
	// If a config file is found, read it in.
	_ = viper.MergeInConfig()
	viperReadEnv()

	// unmarshal all the vars into the config object
	err := viper.Unmarshal(cfg, func(config *mapstructure.DecoderConfig) {
		// Make sure we zero fields before applying them, this is relevant for slices
		// so we do not merge with any already present value and directly apply whatever
		// we got form configs.
		config.ZeroFields = true
	})
	if err != nil {
		cfg.Logger.Warnf("error unmarshalling config: %s", err)
	}

	cfg.Logger.Debugf("Full config loaded: %+v", cfg)

	return cfg, nil
}

func ReadConfigRun(configDir string, mounter mount.Interface) (*v1.RunConfig, error) {
	cfg := config.NewRunConfig(
		config.WithLogger(v1.NewLogger()),
		config.WithMounter(mounter),
	)

	configLogger(cfg.Logger, cfg.Fs)

	cfgDefault := []string{"/etc/os-release", "/etc/cos/config", "/etc/cos-upgrade-image"}

	for _, c := range cfgDefault {
		if _, err := os.Stat(c); err == nil {
			viper.SetConfigFile(c)
			viper.SetConfigType("env")
			cobra.CheckErr(viper.MergeInConfig())
		}
	}

	if exists, _ := utils.Exists(cfg.Fs, configDir); exists {
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
		// If a config file is found, read it in.
		err := viper.MergeInConfig()
		if err != nil {
			cfg.Logger.Warnf("error merging config files: %s", err)
		}
	}

	// Load extra config files on configdir/config.d/ so we can override config values
	cfgExtra := fmt.Sprintf("%s/config.d/", strings.TrimSuffix(configDir, "/"))
	if _, err := os.Stat(cfgExtra); err == nil {
		viper.AddConfigPath(cfgExtra)
		_ = filepath.WalkDir(cfgExtra, func(path string, d fs.DirEntry, err error) error {
			if !d.IsDir() {
				viper.SetConfigName(d.Name())
				cobra.CheckErr(viper.MergeInConfig())
			}
			return nil
		})
	}

	viperReadEnv()

	// unmarshal all the vars into the config object
	err := viper.Unmarshal(cfg)
	if err != nil {
		cfg.Logger.Warnf("error unmarshalling config: %s", err)
	}

	cfg.Logger.Debugf("Full config loaded: %+v", cfg)

	return cfg, nil
}

func configLogger(log v1.Logger, vfs v1.FS) {
	// Set debug level
	if viper.GetBool("debug") {
		log.SetLevel(v1.DebugLevel())
	}

	// Set formatter so both file and stdout format are equal
	log.SetFormatter(&logrus.TextFormatter{
		ForceColors:      true,
		DisableColors:    false,
		DisableTimestamp: false,
		FullTimestamp:    true,
	})

	// Logfile
	logfile := viper.GetString("logfile")
	if logfile != "" {
		o, err := vfs.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fs.ModePerm)

		if err != nil {
			log.Errorf("Could not open %s for logging to file: %s", logfile, err.Error())
		}

		if viper.GetBool("quiet") { // if quiet is set, only set the log to the file
			log.SetOutput(o)
		} else { // else set it to both stdout and the file
			mw := io.MultiWriter(os.Stdout, o)
			log.SetOutput(mw)
		}
	} else { // no logfile
		if viper.GetBool("quiet") { // quiet is enabled so discard all logging
			log.SetOutput(ioutil.Discard)
		} else { // default to stdout
			log.SetOutput(os.Stdout)
		}
	}

	v := version.Get()
	log.Infof("Starting elemental version %s", v.Version)
}

func viperReadEnv() {
	// Set the prefix for vars so we get only the ones starting with ELEMENTAL
	viper.SetEnvPrefix("ELEMENTAL")

	// If we expect to override complex keys in the config, i.e. configs that are nested, we probably need to manually do
	// the env stuff ourselves, as this will only match keys in the config root
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)

	// Manually bind public key env variable as it uses a different name in config files or flags.
	_ = viper.BindEnv("CosingPubKey", "COSIGN_PUBLIC_KEY_LOCATION")

	viper.AutomaticEnv() // read in environment variables that match
}
