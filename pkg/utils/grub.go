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

package utils

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	efilib "github.com/canonical/go-efilib"
	"github.com/rancher/elemental-cli/pkg/constants"
	cnst "github.com/rancher/elemental-cli/pkg/constants"
	eleefi "github.com/rancher/elemental-cli/pkg/efi"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
)

const bootEntryName = "elemental-shim"

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
func (g Grub) Install(target, rootDir, bootDir, grubConf, tty string, efi bool, stateLabel string, disableBootEntry bool, clearBootEntries bool) (err error) { // nolint:gocyclo
	var grubargs []string
	var grubdir, finalContent string
	// only install grub on non-efi systems
	if !efi {
		g.config.Logger.Info("Installing GRUB..")

		grubargs = append(
			grubargs,
			fmt.Sprintf("--root-directory=%s", rootDir),
			fmt.Sprintf("--boot-directory=%s", bootDir),
			"--target=i386-pc",
			target,
		)
		g.config.Logger.Debugf("Running grub with the following args: %s", grubargs)
		out, err := g.config.Runner.Run("grub2-install", grubargs...)
		if err != nil {
			g.config.Logger.Errorf(string(out))
			return err
		}
		g.config.Logger.Infof("Grub install to device %s complete", target)
	}

	// At this point the active mountpoint has all the data from the installation source, so we should be able to use
	// the grub.cfg bundled in there
	grubdir = filepath.Join(rootDir, grubConf)
	g.config.Logger.Infof("Using grub config dir %s", grubdir)

	grubCfg, err := g.config.Fs.ReadFile(grubdir)
	if err != nil {
		g.config.Logger.Errorf("Failed reading grub config file: %s", filepath.Join(rootDir, grubConf))
		return err
	}

	// Create Needed dir under state partition to store the grub.cfg and any needed modules
	err = MkdirAll(g.config.Fs, filepath.Join(bootDir, fmt.Sprintf("grub2/%s-efi", g.config.Arch)), cnst.DirPerm)
	if err != nil {
		return fmt.Errorf("error creating grub dir: %s", err)
	}

	grubConfTarget, err := g.config.Fs.Create(filepath.Join(bootDir, "grub2/grub.cfg"))
	if err != nil {
		return err
	}

	defer grubConfTarget.Close()

	if tty == "" {
		// Get current tty and remove /dev/ from its name
		out, err := g.config.Runner.Run("tty")
		tty = strings.TrimPrefix(strings.TrimSpace(string(out)), "/dev/")
		if err != nil {
			g.config.Logger.Warnf("failed to find current tty, leaving it unset")
			tty = ""
		}
	}

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

	g.config.Logger.Infof("Copying grub contents from %s to %s", grubdir, filepath.Join(bootDir, "grub2/grub.cfg"))
	_, err = grubConfTarget.WriteString(finalContent)
	if err != nil {
		return err
	}

	if efi {
		// Copy required extra modules to boot dir under the state partition
		// otherwise if we insmod it will fail to find them
		// We no longer call grub-install here so the modules are not setup automatically in the state partition
		// as they were before. We now use the bundled grub.efi provided by the shim package
		g.config.Logger.Infof("Generating grub files for efi on %s", target)
		var foundModules bool
		var foundEfi bool
		for _, m := range []string{"loopback.mod", "squash4.mod", "xzio.mod"} {
			err = WalkDirFs(g.config.Fs, rootDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.Name() == m && strings.Contains(path, g.config.Arch) {
					fileWriteName := filepath.Join(bootDir, fmt.Sprintf("grub2/%s-efi/%s", g.config.Arch, m))
					g.config.Logger.Debugf("Copying %s to %s", path, fileWriteName)
					fileContent, err := g.config.Fs.ReadFile(path)
					if err != nil {
						return fmt.Errorf("error reading %s: %s", path, err)
					}
					err = g.config.Fs.WriteFile(fileWriteName, fileContent, cnst.FilePerm)
					if err != nil {
						return fmt.Errorf("error writing %s: %s", fileWriteName, err)
					}
					foundModules = true
					return nil
				}
				return err
			})
			if !foundModules {
				return fmt.Errorf("did not find grub modules under %s", rootDir)
			}
		}

		err = MkdirAll(g.config.Fs, filepath.Join(cnst.EfiDir, "EFI/boot/"), cnst.DirPerm)
		if err != nil {
			g.config.Logger.Errorf("Error creating dirs: %s", err)
			return err
		}
		err = MkdirAll(g.config.Fs, filepath.Join(cnst.EfiDir, "EFI/elemental/"), cnst.DirPerm)
		if err != nil {
			g.config.Logger.Errorf("Error creating dirs: %s", err)
			return err
		}

		// Copy needed files for efi boot
		system, err := IdentifySourceSystem(g.config.Fs, rootDir)
		if err != nil {
			return err
		}
		g.config.Logger.Infof("Identified source system as %s", system)

		var shimFiles []string
		var shimName string

		switch system {
		case cnst.Fedora:
			switch g.config.Arch {
			case cnst.ArchArm64:
				shimFiles = []string{"shimaa64.efi", "mmaa64.efi", "grubx64.efi"}
				shimName = "shimaa64.efi"
			default:
				shimFiles = []string{"shimx64.efi", "mmx64.efi", "grubx64.efi"}
				shimName = "shimx64.efi"
			}
		case cnst.Ubuntu:
			switch g.config.Arch {
			case cnst.ArchArm64:
				shimFiles = []string{"shimaa64.efi.signed", "mmaa64.efi", "grubx64.efi.signed"}
				shimName = "shimaa64.efi.signed"
			default:
				shimFiles = []string{"shimx64.efi.signed", "mmx64.efi", "grubx64.efi.signed"}
				shimName = "shimx64.efi.signed"
			}
		case cnst.Suse:
			shimFiles = []string{"shim.efi", "MokManager.efi", "grub.efi"}
			shimName = "shim.efi"
		}

		for _, f := range shimFiles {
			_ = WalkDirFs(g.config.Fs, rootDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.Name() == f {
					fileContent, err := g.config.Fs.ReadFile(path)
					if err != nil {
						return fmt.Errorf("error reading %s: %s", path, err)
					}

					// Copy to fallback dir
					fileWriteName := filepath.Join(cnst.EfiDir, fmt.Sprintf("EFI/boot/%s", f))
					g.config.Logger.Debugf("Copying %s to %s", path, fileWriteName)
					err = g.config.Fs.WriteFile(fileWriteName, fileContent, cnst.FilePerm)
					if err != nil {
						return fmt.Errorf("error writing %s: %s", fileWriteName, err)
					}
					// Copy to proper dir
					fileWriteName = filepath.Join(cnst.EfiDir, fmt.Sprintf("EFI/elemental/%s", f))
					g.config.Logger.Debugf("Copying %s to %s", path, fileWriteName)
					err = g.config.Fs.WriteFile(fileWriteName, fileContent, cnst.FilePerm)
					if err != nil {
						return fmt.Errorf("error writing %s: %s", fileWriteName, err)
					}

					foundEfi = true
					return nil
				}
				return err
			})
			if !foundEfi {
				return fmt.Errorf("did not find efi artifacts under %s", rootDir)
			}
		}

		// Rename the shimName to the fallback name so the system boots from fallback. This means that we do not create
		// any bootloader entries, so our recent installation has the lower priority if something else is on the bootloader
		writeShim := "bootx64.efi"

		if g.config.Arch == cnst.ArchArm64 {
			writeShim = "bootaa64.efi"
		}

		readShim, err := g.config.Fs.ReadFile(filepath.Join(cnst.EfiDir, "EFI/boot/", shimName))
		if err != nil {
			return fmt.Errorf("could not read shim file %s at dir %s", shimName, cnst.EfiDir)
		}

		err = g.config.Fs.WriteFile(filepath.Join(cnst.EfiDir, "EFI/boot/", writeShim), readShim, cnst.FilePerm)
		if err != nil {
			return fmt.Errorf("could nto write shim file %s at dir %s", shimName, cnst.EfiDir)
		}

		// Add grub.cfg in EFI that chainloads the grub.cfg in recovery
		// Notice that we set the config to /grub2/grub.cfg which means the above we need to copy the file from
		// the installation source into that dir
		grubCfgContent := []byte(fmt.Sprintf("search --no-floppy --label --set=root %s\nset prefix=($root)/grub2\nconfigfile ($root)/grub2/grub.cfg", stateLabel))
		// Fallback
		err = g.config.Fs.WriteFile(filepath.Join(cnst.EfiDir, "EFI/boot/grub.cfg"), grubCfgContent, cnst.FilePerm)
		if err != nil {
			return fmt.Errorf("error writing %s: %s", filepath.Join(cnst.EfiDir, "EFI/boot/grub.cfg"), err)
		}
		// Proper efi dir
		err = g.config.Fs.WriteFile(filepath.Join(cnst.EfiDir, "EFI/elemental/grub.cfg"), grubCfgContent, cnst.FilePerm)
		if err != nil {
			return fmt.Errorf("error writing %s: %s", filepath.Join(cnst.EfiDir, "EFI/boot/grub.cfg"), err)
		}

		if !disableBootEntry {
			efivars := eleefi.RealEFIVariables{}
			if clearBootEntries {
				err = g.ClearBootEntry()
				if err != nil {
					return err
				}
			}
			err = g.CreateBootEntry(shimName, filepath.Join(cnst.EfiDir, "/EFI/elemental/"), efivars)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ClearBootEntry will go over the BootXXXX efi vars and remove any that matches our name
// Used in install as we re-create the partitions, so the UUID of those partitions is no longer valid for the old entry
// And we don't want to leave a broken entry around
func (g Grub) ClearBootEntry() error {
	variables, _ := efilib.ListVariables()
	for _, v := range variables {
		if regexp.MustCompile(`Boot[0-9a-fA-F]{4}`).MatchString(v.Name) {
			variable, _, _ := efilib.ReadVariable(v.Name, v.GUID)
			option, err := efilib.ReadLoadOption(bytes.NewReader(variable))
			if err != nil {
				continue
			}
			// TODO: Find a way to identify the old VS new partition UUID and compare them before removing?
			if option.Description == bootEntryName {
				g.config.Logger.Debugf("Entry for %s already exists, removing it: %s", bootEntryName, option.String())
				_, attrs, err := efilib.ReadVariable(v.Name, v.GUID)
				if err != nil {
					g.config.Logger.Errorf("failed to remove efi entry %s: %s", v.Name, err.Error())
					return err
				}
				err = efilib.WriteVariable(v.Name, v.GUID, attrs, nil)
				if err != nil {
					g.config.Logger.Errorf("failed to remove efi entry %s: %s", v.Name, err.Error())
					return err
				}
			}
		}
	}
	return nil
}

// CreateBootEntry will create an entry in the efi vars for our shim and set it to boot first in the bootorder
func (g Grub) CreateBootEntry(shimName string, relativeTo string, efiVariables eleefi.Variables) error {
	g.config.Logger.Debugf("Creating boot entry for elemental pointing to shim /EFI/elemental/%s", shimName)
	bm, err := eleefi.NewBootManagerForVariables(efiVariables)
	if err != nil {
		return err
	}

	// HINT: FindOrCreate does not find older entries if the partition UUID has changed, i.e. on a reinstall.
	bootEntryNumber, err := bm.FindOrCreateEntry(eleefi.BootEntry{
		Filename:    shimName,
		Label:       bootEntryName,
		Description: bootEntryName,
	}, relativeTo)
	if err != nil {
		g.config.Logger.Errorf("error creating boot entry: %s", err.Error())
		return err
	}
	// Commit the new boot order by prepending our entry to the current boot order
	err = bm.PrependAndSetBootOrder([]int{bootEntryNumber})
	if err != nil {
		g.config.Logger.Errorf("error setting boot order: %s", err.Error())
		return err
	}
	g.config.Logger.Infof("Entry created for %s in the EFI boot manager", bootEntryName)
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
