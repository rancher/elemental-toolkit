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

package config

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/sanity-io/litter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/rancher/elemental-toolkit/v2/internal/version"
	"github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var decodeHook = viper.DecodeHook(
	mapstructure.ComposeDecodeHookFunc(
		UnmarshalerHook(),
		KeyValuePairHook(),
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	),
)

type Unmarshaler interface {
	CustomUnmarshal(interface{}) (bool, error)
}

func KeyValuePairHook() mapstructure.DecodeHookFuncType {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() != reflect.String {
			return data, nil
		}

		if to != reflect.TypeOf(types.KeyValuePair{}) {
			return data, nil
		}

		return types.KeyValuePairFromData(data)
	}
}

func UnmarshalerHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Value, to reflect.Value) (interface{}, error) {
		// get the destination object address if it is not passed by reference
		if to.CanAddr() {
			to = to.Addr()
		}
		// If the destination implements the unmarshaling interface
		u, ok := to.Interface().(Unmarshaler)
		if !ok {
			return from.Interface(), nil
		}
		// If it is nil and a pointer, create and assign the target value first
		if to.IsNil() && to.Type().Kind() == reflect.Ptr {
			to.Set(reflect.New(to.Type().Elem()))
			u = to.Interface().(Unmarshaler)
		}
		// Call the custom unmarshaling method
		cont, err := u.CustomUnmarshal(from.Interface())
		if cont {
			// Continue with the decoding stack
			return from.Interface(), err
		}
		// Decoding finalized
		return to.Interface(), err
	}
}

// setDecoder sets ZeroFields mastructure attribute to true
func setDecoder(config *mapstructure.DecoderConfig) {
	// Make sure we zero fields before applying them, this is relevant for slices
	// so we do not merge with any already present value and directly apply whatever
	// we got form configs.
	config.ZeroFields = true
}

// BindGivenFlags binds to viper only passed flags, ignoring any non provided flag
func bindGivenFlags(vp *viper.Viper, flagSet *pflag.FlagSet) {
	if flagSet != nil {
		flagSet.VisitAll(func(f *pflag.Flag) {
			if f.Changed {
				_ = vp.BindPFlag(f.Name, f)
			}
		})
	}
}

func ReadConfigBuild(configDir string, flags *pflag.FlagSet, mounter types.Mounter) (*types.BuildConfig, error) {
	logger := types.NewLogger()

	cfg := config.NewBuildConfig(
		config.WithLogger(logger),
		config.WithMounter(mounter),
		config.WithOCIImageExtractor(),
	)

	configLogger(cfg.Logger, cfg.Fs)
	if configDir == "" {
		configDir = "."
		cfg.Logger.Info("Reading configuration from current directory")
	} else {
		cfg.Logger.Infof("Reading configuration from '%s'", configDir)
	}

	// merge yaml config files on top of default runconfig
	if exists, _ := utils.Exists(cfg.Fs, filepath.Join(configDir, "manifest.yaml")); exists {
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("manifest")
		// If a config file is found, read it in.
		err := viper.MergeInConfig()
		if err != nil {
			cfg.Logger.Error("error merging config files: %s", err)
			return cfg, err
		}
	}

	// Bind buildconfig flags
	bindGivenFlags(viper.GetViper(), flags)
	// merge environment variables on top for rootCmd
	viperReadEnv(viper.GetViper(), "BUILD", constants.GetBuildKeyEnvMap())

	// unmarshal all the vars into the config object
	err := viper.Unmarshal(cfg, setDecoder, decodeHook)
	if err != nil {
		cfg.Logger.Warnf("error unmarshalling config: %s", err)
	}

	err = cfg.Sanitize()
	cfg.Logger.Debugf("Full config loaded: %s", litter.Sdump(cfg))
	return cfg, err
}

func ReadConfigRun(configDir string, flags *pflag.FlagSet, mounter types.Mounter) (*types.RunConfig, error) {
	cfg := config.NewRunConfig(
		config.WithLogger(types.NewLogger()),
		config.WithMounter(mounter),
		config.WithOCIImageExtractor(),
	)
	configLogger(cfg.Logger, cfg.Fs)
	if configDir == "" {
		configDir = constants.ConfigDir
	}
	cfg.Logger.Infof("Reading configuration from '%s'", configDir)

	const cfgDefault = "/etc/os-release"
	if exists, _ := utils.Exists(cfg.Fs, cfgDefault); exists {
		viper.SetConfigFile(cfgDefault)
		viper.SetConfigType("env")

		err := viper.MergeInConfig()
		if err != nil {
			cfg.Logger.Errorf("error merging os-release file: %s", err)
			return cfg, err
		}
	}

	// merge yaml config files on top of default runconfig
	if exists, _ := utils.Exists(cfg.Fs, filepath.Join(configDir, "config.yaml")); exists {
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
		// If a config file is found, read it in.
		err := viper.MergeInConfig()
		if err != nil {
			cfg.Logger.Errorf("error merging config files: %s", err)
			return cfg, err
		}
	}

	// Load extra config files on configdir/config.d/ so we can override config values
	cfgExtra := filepath.Join(configDir, "config.d")
	if exists, _ := utils.Exists(cfg.Fs, cfgExtra); exists {
		viper.AddConfigPath(cfgExtra)
		err := filepath.WalkDir(cfgExtra, func(_ string, d fs.DirEntry, _ error) error {
			if !d.IsDir() && filepath.Ext(d.Name()) == ".yaml" {
				viper.SetConfigType("yaml")
				viper.SetConfigName(strings.TrimSuffix(d.Name(), ".yaml"))
				return viper.MergeInConfig()
			}
			return nil
		})

		if err != nil {
			cfg.Logger.Errorf("error merging extra config files: %s", err)
			return cfg, err
		}
	}

	// Bind runconfig flags
	bindGivenFlags(viper.GetViper(), flags)
	// merge environment variables on top for rootCmd
	viperReadEnv(viper.GetViper(), "", constants.GetRunKeyEnvMap())

	// unmarshal all the vars into the RunConfig object
	err := viper.Unmarshal(cfg, setDecoder, decodeHook)
	if err != nil {
		cfg.Logger.Warnf("error unmarshalling RunConfig: %s", err)
	}

	err = cfg.Sanitize()
	cfg.Logger.Debugf("Full config loaded: %s", litter.Sdump(cfg))
	return cfg, err
}

func ReadInstallSpec(r *types.RunConfig, flags *pflag.FlagSet) (*types.InstallSpec, error) {
	install := config.NewInstallSpec(r.Config)
	vp := viper.Sub("install")
	if vp == nil {
		vp = viper.New()
	}
	// Bind install cmd flags
	bindGivenFlags(vp, flags)
	// Bind install env vars
	viperReadEnv(vp, "INSTALL", constants.GetInstallKeyEnvMap())

	err := vp.Unmarshal(install, setDecoder, decodeHook)
	if err != nil {
		r.Logger.Warnf("error unmarshalling InstallSpec: %s", err)
	}
	err = install.Sanitize()
	r.Logger.Debugf("Loaded install spec: %s", litter.Sdump(install))
	return install, err
}

func ReadInitSpec(r *types.RunConfig, flags *pflag.FlagSet) (*types.InitSpec, error) {
	init := config.NewInitSpec()
	vp := viper.Sub("init")
	if vp == nil {
		vp = viper.New()
	}
	// Bind init cmd flags
	bindGivenFlags(vp, flags)
	// Bind init env vars
	viperReadEnv(vp, "INIT", constants.GetInitKeyEnvMap())

	err := vp.Unmarshal(init, setDecoder, decodeHook)
	if err != nil {
		r.Logger.Warnf("error unmarshalling InitSpec: %s", err)
	}
	return init, err
}

func ReadMountSpec(r *types.RunConfig, flags *pflag.FlagSet) (*types.MountSpec, error) {
	mount := config.NewMountSpec(r.Config)

	vp := viper.Sub("mount")
	if vp == nil {
		vp = viper.New()
	}
	// Bind mount cmd flags
	bindGivenFlags(vp, flags)
	// Bind mount env vars
	viperReadEnv(vp, "MOUNT", constants.GetMountKeyEnvMap())

	err := vp.Unmarshal(mount, setDecoder, decodeHook)
	if err != nil {
		r.Logger.Warnf("error unmarshalling MountSpec: %s", err)
		return mount, err
	}

	if err := applyMountEnvVars(r, mount); err != nil {
		r.Logger.Errorf("Error reading mount env-vars: %s", err.Error())
		return mount, err
	}

	if err := applyKernelCmdline(r, mount); err != nil {
		r.Logger.Errorf("Error reading kernel cmdline: %s", err.Error())
		return mount, err
	}

	err = mount.Sanitize()
	r.Logger.Debugf("Loaded mount spec: %s", litter.Sdump(mount))
	return mount, err
}

func applyKernelCmdline(r *types.RunConfig, mount *types.MountSpec) error {
	cmdline, err := r.Config.Fs.ReadFile("/proc/cmdline")
	if err != nil {
		r.Logger.Errorf("Error reading /proc/cmdline: %s", err.Error())
		return err
	}

	for _, cmd := range strings.Fields(string(cmdline)) {
		split := strings.SplitN(cmd, "=", 2)

		val := ""
		if len(split) == 2 {
			val = split[1]
		}

		switch split[0] {
		case "elemental.mode":
			switch {
			case strings.Contains(val, constants.ActiveImgName):
				mount.Mode = constants.ActiveImgName
			case strings.Contains(val, constants.PassiveImgName):
				mount.Mode = constants.PassiveImgName
			case strings.Contains(val, constants.RecoveryImgName):
				mount.Mode = constants.RecoveryImgName
			default:
				r.Logger.Errorf("Error parsing cmdline %s", cmd)
				return fmt.Errorf("Unknown image path: %s", val)
			}
		case "elemental.disable":
			mount.Disable = true
		case "elemental.overlay":
			err := applyMountOverlay(mount, val)
			if err != nil {
				return err
			}
		case "elemental.oemlabel":
			oemdev := fmt.Sprintf("LABEL=%s", val)
			var mnt *types.VolumeMount
			for _, mnt = range mount.Volumes {
				if mnt.Mountpoint == constants.OEMPath {
					mnt.Device = oemdev
					break
				}
			}
			if mnt == nil {
				mount.Volumes = append(mount.Volumes, &types.VolumeMount{
					Mountpoint: constants.OEMPath,
					Device:     oemdev,
					Options:    []string{"rw", "defaults"},
				})
			}
		}
	}

	return nil
}

func applyMountEnvVars(r *types.RunConfig, mount *types.MountSpec) error {
	r.Logger.Debugf("Applying mount env-vars")

	overlay := os.Getenv("OVERLAY")
	if overlay != "" {
		r.Logger.Debugf("Setting ephemeral settings based on OVERLAY")

		err := applyMountOverlay(mount, overlay)
		if err != nil {
			return err
		}
	}

	rwPaths := os.Getenv("RW_PATHS")
	if rwPaths != "" {
		r.Logger.Debugf("Setting ephemeral paths based on RW_PATHS")
		mount.Ephemeral.Paths = strings.Split(rwPaths, " ")
	}

	persistentPaths := os.Getenv("PERSISTENT_STATE_PATHS")
	if persistentPaths != "" {
		r.Logger.Debugf("Setting persistent paths based on PERSISTENT_STATE_PATHS")
		mount.Persistent.Paths = strings.Split(persistentPaths, " ")
	}

	persistentBind := os.Getenv("PERSISTENT_STATE_BIND")
	if persistentBind != "" {
		r.Logger.Debugf("Setting persistent bind based on PERSISTENT_STATE_BIND")
		mount.Persistent.Mode = constants.BindMode
	}

	return nil
}

func applyMountOverlay(mount *types.MountSpec, overlay string) error {
	split := strings.Split(overlay, ":")

	if len(split) == 2 && split[0] == constants.Tmpfs {
		mount.Ephemeral.Device = ""
		mount.Ephemeral.Type = split[0]
		mount.Ephemeral.Size = split[1]
		return nil
	}

	mount.Ephemeral.Type = constants.Block
	mount.Ephemeral.Size = ""

	blockSplit := strings.Split(overlay, "=")
	if len(blockSplit) != 2 {
		return fmt.Errorf("Unknown block overlay '%s'", overlay)
	}

	switch blockSplit[0] {
	case "LABEL":
		mount.Ephemeral.Device = fmt.Sprintf("/dev/disk/by-label/%s", blockSplit[1])
	case "UUID":
		mount.Ephemeral.Device = fmt.Sprintf("/dev/disk/by-uuid/%s", blockSplit[1])
	default:
		return fmt.Errorf("Unknown block overlay '%s'", overlay)
	}

	return nil
}

func ReadResetSpec(r *types.RunConfig, flags *pflag.FlagSet) (*types.ResetSpec, error) {
	reset, err := config.NewResetSpec(r.Config)
	if err != nil {
		return nil, fmt.Errorf("failed initializing reset spec: %v", err)
	}
	vp := viper.Sub("reset")
	if vp == nil {
		vp = viper.New()
	}
	// Bind reset cmd flags
	bindGivenFlags(vp, flags)
	// Bind reset env vars
	viperReadEnv(vp, "RESET", constants.GetResetKeyEnvMap())

	err = vp.Unmarshal(reset, setDecoder, decodeHook)
	if err != nil {
		r.Logger.Warnf("error unmarshalling ResetSpec: %s", err)
	}
	err = reset.Sanitize()
	r.Logger.Debugf("Loaded reset spec: %s", litter.Sdump(reset))
	return reset, err
}

func ReadUpgradeSpec(r *types.RunConfig, flags *pflag.FlagSet, recoveryOnly bool) (*types.UpgradeSpec, error) {
	upgrade, err := config.NewUpgradeSpec(r.Config)
	if err != nil {
		return nil, fmt.Errorf("failed initializing upgrade spec: %v", err)
	}
	vp := viper.Sub("upgrade")
	if vp == nil {
		vp = viper.New()
	}
	// Bind upgrade cmd flags
	bindGivenFlags(vp, flags)
	// Bind upgrade env vars
	viperReadEnv(vp, "UPGRADE", constants.GetUpgradeKeyEnvMap())

	err = vp.Unmarshal(upgrade, setDecoder, decodeHook)
	if err != nil {
		r.Logger.Warnf("error unmarshalling UpgradeSpec: %s", err)
	}

	if recoveryOnly {
		err = upgrade.SanitizeForRecoveryOnly()
	} else {
		err = upgrade.Sanitize()
	}
	r.Logger.Debugf("Loaded upgrade UpgradeSpec: %s", litter.Sdump(upgrade))
	return upgrade, err
}

func ReadBuildISO(b *types.BuildConfig, flags *pflag.FlagSet) (*types.LiveISO, error) {
	iso := config.NewISO()
	vp := viper.Sub("iso")
	if vp == nil {
		vp = viper.New()
	}
	// Bind build-iso cmd flags
	bindGivenFlags(vp, flags)
	// Bind build-iso env vars
	viperReadEnv(vp, "ISO", constants.GetISOKeyEnvMap())

	err := vp.Unmarshal(iso, setDecoder, decodeHook)
	if err != nil {
		b.Logger.Warnf("error unmarshalling LiveISO: %s", err)
	}
	err = iso.Sanitize()
	b.Logger.Debugf("Loaded LiveISO: %s", litter.Sdump(iso))
	return iso, err
}

func ReadBuildDisk(b *types.BuildConfig, flags *pflag.FlagSet) (*types.DiskSpec, error) {
	disk := config.NewDisk(b)
	vp := viper.Sub("disk")
	if vp == nil {
		vp = viper.New()
	}
	// Bind build-disk cmd flags
	bindGivenFlags(vp, flags)
	// Bind build-disk env vars
	viperReadEnv(vp, "DISK", constants.GetDiskKeyEnvMap())

	err := vp.Unmarshal(disk, setDecoder, decodeHook)
	if err != nil {
		b.Logger.Warnf("error unmarshalling Disk: %s", err)
	}
	err = disk.Sanitize()
	b.Logger.Debugf("Loaded Disk: %s", litter.Sdump(disk))
	return disk, err
}

func configLogger(log types.Logger, vfs types.FS) {
	// Set debug level
	if viper.GetBool("debug") {
		log.SetLevel(types.DebugLevel())
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
			log.SetOutput(io.Discard)
		} else { // default to stdout
			log.SetOutput(os.Stdout)
		}
	}

	v := version.Get()
	if log.GetLevel() == logrus.DebugLevel {
		log.Debugf("Starting elemental version %s on commit %s", v.Version, v.GitCommit)
	} else {
		log.Infof("Starting elemental version %s", v.Version)
	}
}

func viperReadEnv(vp *viper.Viper, prefix string, keyMap map[string]string) {
	// If we expect to override complex keys in the config, i.e. configs
	// that are nested, we probably need to manually do the env stuff
	// ourselves, as this will only match keys in the config root
	replacer := strings.NewReplacer("-", "_")
	vp.SetEnvKeyReplacer(replacer)

	if prefix == "" {
		prefix = "ELEMENTAL"
	} else {
		prefix = fmt.Sprintf("ELEMENTAL_%s", prefix)
	}

	// Manually bind keys to env variable if custom names are needed.
	for k, v := range keyMap {
		_ = vp.BindEnv(k, fmt.Sprintf("%s_%s", prefix, v))
	}
}
