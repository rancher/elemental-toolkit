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

package utils

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/spf13/afero"
)

// Grub is the struct that will allow us to install grub to the target device
type Grub struct {
	config *v1.RunConfig
}

func NewGrub(config *v1.RunConfig) *Grub {
	g := &Grub{
		config: config,
	}

	return g
}

// Install installs grub into the device, copy the config file and add any extra TTY to grub
func (g Grub) Install() (err error) { // nolint:gocyclo
	var grubargs []string
	var arch, grubdir, tty, finalContent string

	switch runtime.GOARCH {
	case "arm64":
		arch = "arm64"
	default:
		arch = "x86_64"
	}
	g.config.Logger.Info("Installing GRUB..")

	if g.config.Tty == "" {
		// Get current tty and remove /dev/ from its name
		out, err := g.config.Runner.Run("tty")
		tty = strings.TrimPrefix(strings.TrimSpace(string(out)), "/dev/")
		if err != nil {
			return err
		}
	} else {
		tty = g.config.Tty
	}

	efiExists, _ := afero.Exists(g.config.Fs, cnst.EfiDevice)

	if g.config.ForceEfi || efiExists {
		g.config.Logger.Infof("Installing grub efi for arch %s", arch)
		grubargs = append(
			grubargs,
			fmt.Sprintf("--target=%s-efi", arch),
			fmt.Sprintf("--efi-directory=%s", cnst.EfiDir),
		)
	}

	statePart := g.config.Partitions.GetByName(cnst.StatePartName)
	activeImg := g.config.Images.GetActive()
	if statePart == nil || activeImg == nil {
		g.config.Logger.Errorf("State partition and/or Active image configuration is missing")
		return errors.New("Failed setting grub arguments")
	}

	grubargs = append(
		grubargs,
		fmt.Sprintf("--root-directory=%s", activeImg.MountPoint),
		fmt.Sprintf("--boot-directory=%s", statePart.MountPoint),
		"--removable", g.config.Target,
	)

	g.config.Logger.Debugf("Running grub with the following args: %s", grubargs)
	out, err := g.config.Runner.Run("grub2-install", grubargs...)
	if err != nil {
		g.config.Logger.Errorf(string(out))
		return err
	}

	grub1dir := filepath.Join(statePart.MountPoint, "grub")
	grub2dir := filepath.Join(statePart.MountPoint, "grub2")

	// Select the proper dir for grub
	if ok, _ := afero.IsDir(g.config.Fs, grub1dir); ok {
		grubdir = grub1dir
	}
	if ok, _ := afero.IsDir(g.config.Fs, grub2dir); ok {
		grubdir = grub2dir
	}
	g.config.Logger.Infof("Found grub config dir %s", grubdir)

	grubConf, err := afero.ReadFile(g.config.Fs, filepath.Join(activeImg.MountPoint, g.config.GrubConf))
	if err != nil {
		g.config.Logger.Errorf("Failed reading grub config file: %s", filepath.Join(activeImg.MountPoint, g.config.GrubConf))
		return err
	}

	grubConfTarget, err := g.config.Fs.Create(fmt.Sprintf("%s/grub.cfg", grubdir))
	if err != nil {
		return err
	}
	defer func(grubConfTarget afero.File) {
		_ = grubConfTarget.Close()
	}(grubConfTarget)

	ttyExists, _ := afero.Exists(g.config.Fs, fmt.Sprintf("/dev/%s", tty))

	if ttyExists && tty != "" && tty != "console" && tty != "tty1" {
		// We need to add a tty to the grub file
		g.config.Logger.Infof("Adding extra tty (%s) to grub.cfg", tty)
		finalContent = strings.Replace(string(grubConf), "console=tty1", fmt.Sprintf("console=tty1 console=%s", tty), -1)
	} else {
		// We don't add anything, just read the file
		finalContent = string(grubConf)
	}

	g.config.Logger.Infof("Copying grub contents from %s to %s", g.config.GrubConf, fmt.Sprintf("%s/grub.cfg", grubdir))
	_, err = grubConfTarget.WriteString(finalContent)
	if err != nil {
		return err
	}

	g.config.Logger.Infof("Grub install to device %s complete", g.config.Target)
	return nil
}

// Sets the given key value pairs into as grub variables into the given file
func (g Grub) SetPersistentVariables(grubEnvFile string, vars map[string]string) error {
	for key, value := range vars {
		g.config.Logger.Debugf("Running grub2-editenv with params: %s set %s=%s", grubEnvFile, key, value)
		out, err := g.config.Runner.Run("grub2-editenv", grubEnvFile, "set", fmt.Sprintf("%s=%s", key, value))
		if err != nil {
			g.config.Logger.Errorf(fmt.Sprintf("Failed setting grub variables: %s", out))
			return err
		}
	}
	return nil
}
