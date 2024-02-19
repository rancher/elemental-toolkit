/*
Copyright © 2022 - 2024 SUSE LLC

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

package bootloader

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"

	efilib "github.com/canonical/go-efilib"

	eleefi "github.com/rancher/elemental-toolkit/v2/pkg/efi"
)

const (
	grubCfgFile = "grub.cfg"
)

var (
	defaultGrubPrefixes = []string{constants.FallbackEFIPath, constants.EntryEFIPath}
)

func getGModulePatterns(module string) []string {
	var patterns []string
	for _, pattern := range constants.GetDefaultGrubModulesPatterns() {
		patterns = append(patterns, filepath.Join(pattern, module))
	}
	return patterns
}

type Grub struct {
	logger   v2.Logger
	fs       v2.FS
	runner   v2.Runner
	platform *v2.Platform

	shimImg    string
	grubEfiImg string
	mokMngr    string

	grubPrefixes       []string
	configFile         string
	elementalCfg       string
	legacyElementalCfg string
	disableBootEntry   bool
	clearBootEntry     bool
	secureBoot         bool
}

var _ v2.Bootloader = (*Grub)(nil)

type GrubOptions func(g *Grub) error

func NewGrub(cfg *v2.Config, opts ...GrubOptions) *Grub {
	secureBoot := true
	if cfg.Platform.Arch == constants.ArchRiscV64 {
		// There is no secure boot for riscv64 for the time being (Dec 2023)
		secureBoot = false
	}
	g := &Grub{
		fs:                 cfg.Fs,
		logger:             cfg.Logger,
		runner:             cfg.Runner,
		platform:           cfg.Platform,
		configFile:         grubCfgFile,
		grubPrefixes:       defaultGrubPrefixes,
		elementalCfg:       filepath.Join(constants.GrubCfgPath, constants.GrubCfg),
		legacyElementalCfg: filepath.Join(constants.LegacyGrubCfgPath, constants.GrubCfg),
		clearBootEntry:     true,
		secureBoot:         secureBoot,
	}

	for _, o := range opts {
		err := o(g)
		if err != nil {
			g.logger.Errorf("error applying config option: %s", err.Error())
			return nil
		}
	}

	return g
}

func WithSecureBoot(secureboot bool) func(g *Grub) error {
	return func(g *Grub) error {
		g.secureBoot = secureboot
		return nil
	}
}

func WithGrubPrefixes(prefixes ...string) func(g *Grub) error {
	return func(g *Grub) error {
		g.grubPrefixes = prefixes
		return nil
	}
}

func WithGrubDisableBootEntry(disableBootEntry bool) func(g *Grub) error {
	return func(g *Grub) error {
		g.disableBootEntry = disableBootEntry
		return nil
	}
}

func WithGrubClearBootEntry(clearBootEntry bool) func(g *Grub) error {
	return func(g *Grub) error {
		g.clearBootEntry = clearBootEntry
		return nil
	}
}

func (g *Grub) findEFIImages(rootDir string) error {
	var err error

	if g.secureBoot && g.shimImg == "" {
		g.shimImg, err = utils.FindFile(g.fs, rootDir, constants.GetShimFilePatterns()...)
		if err != nil {
			g.logger.Errorf("failed to find shim image")
			return err
		}
	}

	if g.grubEfiImg == "" {
		g.grubEfiImg, err = utils.FindFile(g.fs, rootDir, constants.GetGrubEFIFilePatterns()...)
		if err != nil {
			g.logger.Errorf("failed to find grub image")
			return err
		}
	}

	if g.secureBoot && g.mokMngr == "" {
		g.mokMngr, err = utils.FindFile(g.fs, rootDir, constants.GetMokMngrFilePatterns()...)
		if err != nil {
			g.logger.Errorf("failed to find mok manager")
			return err
		}
	}

	return nil
}

func (g *Grub) findModules(rootDir string, modules ...string) ([]string, error) {
	fModules := []string{}

	for _, module := range modules {
		foundModule, err := utils.FindFile(g.fs, rootDir, getGModulePatterns(module)...)
		if err != nil {
			return []string{}, err
		}
		fModules = append(fModules, foundModule)
	}
	return fModules, nil
}

func (g *Grub) installModules(rootDir, bootDir string, modules ...string) error {
	modules, err := g.findModules(rootDir, modules...)
	if err != nil {
		return err
	}
	for _, grubPrefix := range g.grubPrefixes {
		for _, module := range modules {
			fileWriteName := filepath.Join(bootDir, grubPrefix, fmt.Sprintf("%s-efi", g.platform.Arch), filepath.Base(module))
			g.logger.Debugf("Copying %s to %s", module, fileWriteName)
			err = utils.MkdirAll(g.fs, filepath.Dir(fileWriteName), constants.DirPerm)
			if err != nil {
				return fmt.Errorf("error creating destination folder: %v", err)
			}
			err = utils.CopyFile(g.fs, module, fileWriteName)
			if err != nil {
				return fmt.Errorf("error copying %s to %s: %s", module, fileWriteName, err.Error())
			}
		}
	}
	return nil
}

func (g *Grub) InstallEFI(rootDir, efiDir string) error {
	err := g.installModules(rootDir, efiDir, constants.GetDefaultGrubModules()...)
	if err != nil {
		return err
	}

	for _, prefix := range g.grubPrefixes {
		err = g.InstallEFIBinaries(rootDir, efiDir, prefix)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Grub) InstallEFIBinaries(rootDir, efiDir, prefix string) error {
	err := g.findEFIImages(rootDir)
	if err != nil {
		return err
	}

	installPath := filepath.Join(efiDir, prefix)
	err = utils.MkdirAll(g.fs, installPath, constants.DirPerm)
	if err != nil {
		g.logger.Errorf("Error creating dirs: %s", err)
		return err
	}

	shimImg := filepath.Join(installPath, filepath.Base(g.shimImg))
	grubEfi := filepath.Join(installPath, filepath.Base(g.grubEfiImg))

	var bootImg string
	if prefix == constants.FallbackEFIPath {
		switch g.platform.Arch {
		case constants.ArchAmd64, constants.Archx86:
			bootImg = filepath.Join(installPath, constants.EfiImgX86)
		case constants.ArchArm64:
			bootImg = filepath.Join(installPath, constants.EfiImgArm64)
		case constants.ArchRiscV64:
			bootImg = filepath.Join(installPath, constants.EfiImgRiscv64)
		default:
			err = fmt.Errorf("Not supported architecture: %v", g.platform.Arch)
		}
		if err != nil {
			return err
		}
		if g.secureBoot {
			shimImg = bootImg
		} else {
			grubEfi = bootImg
		}
	}

	if g.secureBoot {
		g.logger.Debugf("Copying %s to %s", g.mokMngr, installPath)
		err = utils.CopyFile(g.fs, g.mokMngr, installPath)
		if err != nil {
			return fmt.Errorf("failed copying %s to %s: %s", g.mokMngr, installPath, err.Error())
		}

		g.logger.Debugf("Copying %s to %s", g.shimImg, shimImg)
		err = utils.CopyFile(g.fs, g.shimImg, shimImg)
		if err != nil {
			return fmt.Errorf("failed copying %s to %s: %s", g.shimImg, shimImg, err.Error())
		}
	}

	g.logger.Debugf("Copying %s to %s", g.grubEfiImg, grubEfi)
	err = utils.CopyFile(g.fs, g.grubEfiImg, grubEfi)
	if err != nil {
		return fmt.Errorf("failed copying %s to %s: %s", g.grubEfiImg, installPath, err.Error())
	}

	return nil
}

// DoEFIEntries creates clears any previous entry if requested and creates a new one with the given shim name.
func (g *Grub) DoEFIEntries(shimName, efiDir string) error {
	efivars := eleefi.RealEFIVariables{}
	if g.clearBootEntry {
		err := g.clearEntry()
		if err != nil {
			return err
		}
	}
	return g.CreateEntry(shimName, filepath.Join(efiDir, constants.EntryEFIPath), efivars)
}

// clearEntry will go over the BootXXXX efi vars and remove any that matches our name
// Used in install as we re-create the partitions, so the UUID of those partitions is no longer valid for the old entry
// And we don't want to leave a broken entry around
func (g *Grub) clearEntry() error {
	variables, _ := efilib.ListVariables()
	for _, v := range variables {
		if regexp.MustCompile(`Boot[0-9a-fA-F]{4}`).MatchString(v.Name) {
			variable, _, _ := efilib.ReadVariable(v.Name, v.GUID)
			option, err := efilib.ReadLoadOption(bytes.NewReader(variable))
			if err != nil {
				continue
			}
			// TODO: Find a way to identify the old VS new partition UUID and compare them before removing?
			if option.Description == constants.BootEntryName {
				g.logger.Debugf("Entry for %s already exists, removing it: %s", constants.BootEntryName, option.String())
				_, attrs, err := efilib.ReadVariable(v.Name, v.GUID)
				if err != nil {
					g.logger.Errorf("failed to remove efi entry %s: %s", v.Name, err.Error())
					return err
				}
				err = efilib.WriteVariable(v.Name, v.GUID, attrs, nil)
				if err != nil {
					g.logger.Errorf("failed to remove efi entry %s: %s", v.Name, err.Error())
					return err
				}
			}
		}
	}
	return nil
}

// createBootEntry will create an entry in the efi vars for our shim and set it to boot first in the bootorder
func (g *Grub) CreateEntry(shimName string, relativeTo string, efiVariables eleefi.Variables) error {
	g.logger.Debugf("Creating boot entry for elemental pointing to shim %s/%s", constants.EntryEFIPath, shimName)
	bm, err := eleefi.NewBootManagerForVariables(efiVariables)
	if err != nil {
		return err
	}

	// HINT: FindOrCreate does not find older entries if the partition UUID has changed, i.e. on a reinstall.
	bootEntryNumber, err := bm.FindOrCreateEntry(eleefi.BootEntry{
		Filename:    shimName,
		Label:       constants.BootEntryName,
		Description: constants.BootEntryName,
	}, relativeTo)
	if err != nil {
		g.logger.Errorf("error creating boot entry: %s", err.Error())
		return err
	}
	// Commit the new boot order by prepending our entry to the current boot order
	err = bm.PrependAndSetBootOrder([]int{bootEntryNumber})
	if err != nil {
		g.logger.Errorf("error setting boot order: %s", err.Error())
		return err
	}
	g.logger.Infof("Entry created for %s in the EFI boot manager", constants.BootEntryName)
	return nil
}

// Sets the given key value pairs into as grub variables into the given file
func (g *Grub) SetPersistentVariables(grubEnvFile string, vars map[string]string) error {
	cmd := "grub2-editenv"
	if !g.runner.CommandExists(cmd) {
		cmd = "grub-editenv"
	}

	for key, value := range vars {
		g.logger.Debugf("Running %s with params: %s set %s=%s", cmd, grubEnvFile, key, value)
		out, err := g.runner.Run(cmd, grubEnvFile, "set", fmt.Sprintf("%s=%s", key, value))
		if err != nil {
			g.logger.Errorf(fmt.Sprintf("Failed setting grub variables: %s", out))
			return err
		}
	}
	return nil
}

// SetDefaultEntry Sets the default_meny_entry value in RunConfig.GrubOEMEnv file at in
// State partition mountpoint. If there is not a custom value in the os-release file, we do nothing
// As the grub config already has a sane default
func (g *Grub) SetDefaultEntry(partMountPoint, imgMountPoint, defaultEntry string) error {
	var configEntry string
	osRelease, err := utils.LoadEnvFile(g.fs, filepath.Join(imgMountPoint, "etc", "os-release"))
	g.logger.Debugf("Looking for GRUB_ENTRY_NAME name in %s", filepath.Join(imgMountPoint, "etc", "os-release"))
	if err != nil {
		g.logger.Warnf("Could not load os-release file: %v", err)
	} else {
		configEntry = osRelease["GRUB_ENTRY_NAME"]
		// If its not empty override the defaultEntry and set the one set on the os-release file
		if configEntry != "" {
			defaultEntry = configEntry
		}
	}

	if defaultEntry == "" {
		g.logger.Warn("No default entry name for grub, not setting a name")
		return nil
	}

	g.logger.Infof("Setting default grub entry to %s", defaultEntry)
	return g.SetPersistentVariables(
		filepath.Join(partMountPoint, constants.GrubOEMEnv),
		map[string]string{"default_menu_entry": defaultEntry},
	)
}

// Install installs grub into the device, copy the config file and add any extra TTY to grub
func (g *Grub) Install(rootDir, bootDir string) (err error) {
	err = g.InstallEFI(rootDir, bootDir)
	if err != nil {
		return err
	}

	if !g.disableBootEntry {
		image := g.grubEfiImg
		if g.secureBoot {
			image = g.shimImg
		}
		err = g.DoEFIEntries(filepath.Base(image), constants.EfiDir)
		if err != nil {
			return err
		}
	}

	return g.InstallConfig(rootDir, bootDir)
}

// InstallConfig installs grub configuraton files to the expected location.
// rootDir is the root of the OS image, bootDir is the folder grub read the
// configuration from, usually EFI partition mountpoint
func (g Grub) InstallConfig(rootDir, bootDir string) error {
	for _, path := range g.grubPrefixes {
		grubFile := filepath.Join(rootDir, g.elementalCfg)
		if exists, _ := utils.Exists(g.fs, grubFile); !exists {
			grubFile = filepath.Join(rootDir, g.legacyElementalCfg)
			g.logger.Warnf("Grub config not found, using legacy config: %s", grubFile)
		}

		dstGrubFile := filepath.Join(bootDir, path, g.configFile)

		g.logger.Infof("Using grub config file %s", grubFile)

		// Create Needed dir under state partition to store the grub.cfg and any needed modules
		err := utils.MkdirAll(g.fs, filepath.Join(bootDir, path), constants.DirPerm)
		if err != nil {
			return fmt.Errorf("error creating grub dir: %s", err)
		}

		g.logger.Infof("Copying grub config file from %s to %s", grubFile, dstGrubFile)
		err = utils.CopyFile(g.fs, grubFile, dstGrubFile)
		if err != nil {
			g.logger.Errorf("Failed copying grub config file: %s", err)
			return err
		}
	}

	return nil
}
