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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rancher-sandbox/elemental/pkg/constants"
	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
)

// Grub is the struct that will allow us to install grub to the target device
type Grub struct {
	config *v1.Config
}

func NewGrub(config *v1.Config) *Grub {
	g := &Grub{
		config: config,
	}

	return g
}

// Install installs grub into the device, copy the config file and add any extra TTY to grub
func (g Grub) Install(target, rootDir, bootDir, grubConf, tty string, efi bool) (err error) { // nolint:gocyclo
	var grubargs []string
	var grubdir, finalContent string

	g.config.Logger.Info("Installing GRUB..")

	if tty == "" {
		// Get current tty and remove /dev/ from its name
		out, err := g.config.Runner.Run("tty")
		tty = strings.TrimPrefix(strings.TrimSpace(string(out)), "/dev/")
		if err != nil {
			g.config.Logger.Warnf("failed to find current tty, leaving it unset")
			tty = ""
		}
	}

	if efi {
		g.config.Logger.Infof("Installing grub efi for arch %s", g.config.Arch)
		grubargs = append(
			grubargs,
			fmt.Sprintf("--target=%s-efi", g.config.Arch),
			fmt.Sprintf("--efi-directory=%s", cnst.EfiDir),
		)
	} else {
		if g.config.Arch == "x86_64" {
			grubargs = append(grubargs, "--target=i386-pc")
		}
	}

	grubargs = append(
		grubargs,
		fmt.Sprintf("--root-directory=%s", rootDir),
		fmt.Sprintf("--boot-directory=%s", bootDir),
		"--removable", target,
	)

	g.config.Logger.Debugf("Running grub with the following args: %s", grubargs)
	out, err := g.config.Runner.Run("grub2-install", grubargs...)
	if err != nil {
		g.config.Logger.Errorf(string(out))
		return err
	}

	grub1dir := filepath.Join(bootDir, "grub")
	grub2dir := filepath.Join(bootDir, "grub2")

	// Select the proper dir for grub
	if ok, _ := IsDir(g.config.Fs, grub1dir); ok {
		grubdir = grub1dir
	}
	if ok, _ := IsDir(g.config.Fs, grub2dir); ok {
		grubdir = grub2dir
	}
	g.config.Logger.Infof("Found grub config dir %s", grubdir)

	grubCfg, err := g.config.Fs.ReadFile(filepath.Join(rootDir, grubConf))
	if err != nil {
		g.config.Logger.Errorf("Failed reading grub config file: %s", filepath.Join(rootDir, grubConf))
		return err
	}

	grubConfTarget, err := g.config.Fs.Create(fmt.Sprintf("%s/grub.cfg", grubdir))
	if err != nil {
		return err
	}

	defer grubConfTarget.Close()

	ttyExists, _ := Exists(g.config.Fs, fmt.Sprintf("/dev/%s", tty))

	if ttyExists && tty != "" && tty != "console" && tty != constants.DefaultTty {
		// We need to add a tty to the grub file
		g.config.Logger.Infof("Adding extra tty (%s) to grub.cfg", tty)
		defConsole := fmt.Sprintf("console=%s", constants.DefaultTty)
		finalContent = strings.Replace(string(grubCfg), defConsole, fmt.Sprintf("%s console=%s", defConsole, tty), -1)
	} else {
		// We don't add anything, just read the file
		finalContent = string(grubCfg)
	}

	g.config.Logger.Infof("Copying grub contents from %s to %s", grubConf, fmt.Sprintf("%s/grub.cfg", grubdir))
	_, err = grubConfTarget.WriteString(finalContent)
	if err != nil {
		return err
	}

	g.config.Logger.Infof("Grub install to device %s complete", target)
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
