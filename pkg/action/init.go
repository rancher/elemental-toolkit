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

package action

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/rancher/elemental-cli/pkg/constants"
	elementalError "github.com/rancher/elemental-cli/pkg/error"
	"github.com/rancher/elemental-cli/pkg/systemd"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

//go:embed init-files
var files embed.FS

const (
	embeddedRoot = "init-files"
)

func RunInit(cfg *v1.RunConfig, spec *v1.InitSpec) error {
	if exists, _ := utils.Exists(cfg.Fs, "/.dockerenv"); !exists && !spec.Force {
		return elementalError.New("running outside of container, pass --force to run anyway", elementalError.StatFile)
	}

	entries, err := files.ReadDir(embeddedRoot)
	if err != nil {
		cfg.Config.Logger.Infof("Error reading embedded files: %s", err.Error())
		return err
	}

	units := []*systemd.Unit{}
	dracutConf := []fs.DirEntry{}

	cfg.Config.Logger.Infof("Reading %v files.", len(entries))

	for _, entry := range entries {
		if IsSystemdUnit(entry) {
			cfg.Config.Logger.Debugf("Loading systemd unit %v", entry.Name())

			content, err := files.ReadFile(filepath.Join(embeddedRoot, entry.Name()))
			if err != nil {
				cfg.Config.Logger.Errorf("Error reading unit '%s': %v", entry.Name(), err.Error())
				return err
			}

			units = append(units, systemd.NewUnit(entry.Name(), content))
		} else if IsDracutConfig(entry) {
			dracutConf = append(dracutConf, entry)
		}
	}

	if err := utils.MkdirAll(cfg.Config.Fs, "/usr/lib/systemd/system", constants.DirPerm); err != nil {
		cfg.Config.Logger.Errorf("Failed to create systemd dir: %v", err.Error)
		return err
	}

	cfg.Config.Logger.Infof("Installing %v systemd units.", len(units))

	for _, unit := range units {
		cfg.Config.Logger.Debugf("Installing unit '%s'", unit.Name)

		if err = systemd.Install(cfg.Config.Fs, unit); err != nil {
			cfg.Config.Logger.Errorf("Error installing unit '%s': %v", unit.Name, err.Error())
			return err
		}

		if err = systemd.Enable(cfg.Config.Runner, unit); err != nil {
			cfg.Config.Logger.Errorf("Error enabling unit '%s': %v", unit.Name, err.Error())
			return err
		}
	}

	if err := utils.MkdirAll(cfg.Config.Fs, "/etc/dracut.conf.d", constants.DirPerm); err != nil {
		cfg.Config.Logger.Errorf("Failed to create dracut conf dir: %v", err.Error)
		return err
	}

	cfg.Config.Logger.Infof("Installing %v dracut config files.", len(dracutConf))

	for _, conf := range dracutConf {
		if err := ExtractFile(files, filepath.Join(embeddedRoot, conf.Name()), cfg.Config.Fs, filepath.Join("/etc/dracut.conf.d", conf.Name())); err != nil {
			cfg.Config.Logger.Infof("Failed to copy dracut config %s: %s", conf.Name(), err.Error())
			return err
		}
	}

	if err := utils.MkdirAll(cfg.Config.Fs, "/etc/cos", constants.DirPerm); err != nil {
		cfg.Config.Logger.Errorf("Failed to create cos conf dir: %v", err.Error)
		return err
	}

	cfg.Config.Logger.Infof("Installing GRUB2 config files.")

	if err := ExtractFile(files, filepath.Join(embeddedRoot, "grub.cfg"), cfg.Config.Fs, "/etc/cos/grub.cfg"); err != nil {
		cfg.Config.Logger.Infof("Failed to copy grub.cfg: %s", err.Error())
		return err
	}

	if err := ExtractFile(files, filepath.Join(embeddedRoot, "bootargs.cfg"), cfg.Config.Fs, "/etc/cos/bootargs.cfg"); err != nil {
		cfg.Config.Logger.Infof("Failed to copy bootargs.cfg: %s", err.Error())
		return err
	}

	grub := utils.NewGrub(&cfg.Config)
	firstboot := map[string]string{"next_entry": "recovery"}
	if err := grub.SetPersistentVariables("/etc/cos/grubenv_firstboot", firstboot); err != nil {
		cfg.Config.Logger.Infof("Failed to set GRUB nextboot: %s", err.Error())
		return err
	}

	cfg.Config.Logger.Infof("Make initrd.")
	_, err = cfg.Runner.Run("mkinitrd")
	return err
}

func ExtractFile(srcFs embed.FS, srcPath string, dstFs v1.FS, dstPath string) error {
	content, err := srcFs.ReadFile(srcPath)
	if err != nil {
		return err
	}

	return dstFs.WriteFile(dstPath, content, 0644)
}

func IsSystemdUnit(entry fs.DirEntry) bool {
	return !entry.IsDir() && strings.HasSuffix(entry.Name(), ".service") || strings.HasSuffix(entry.Name(), ".timer")
}

func IsDracutConfig(entry fs.DirEntry) bool {
	return !entry.IsDir() && strings.HasSuffix(entry.Name(), ".conf")
}
