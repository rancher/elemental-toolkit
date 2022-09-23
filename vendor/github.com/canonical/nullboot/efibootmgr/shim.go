// This file is part of nullboot
// Copyright 2021 Canonical Ltd.
// SPDX-License-Identifier: GPL-3.0-only

package efibootmgr

import (
	"fmt"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// BootEntry is a boot entry.
type BootEntry struct {
	Filename    string
	Label       string
	Options     string
	Description string
}

// architectureMaps maps from GOARCH to host
var architectureMap = map[string]string{
	"386":      "ia32",
	"amd64":    "x64",
	"arm":      "arm",
	"arm64":    "aa64",
	"riscv":    "riscv32",
	"riscv64":  "riscv64",
	"riscv128": "riscv128",
}

// appArchitecture can be overriden in a test case for testing purposes
var appArchitecture = ""

// GetEfiArchitecture returns the EFI architecture for the target system
func GetEfiArchitecture() string {
	if appArchitecture != "" {
		return appArchitecture
	}
	return architectureMap[runtime.GOARCH]
}

// WriteShimFallbackToFile opens the specified path in UTF-16LE and then calls WriteShimFallback
func WriteShimFallbackToFile(path string, entries []BootEntry) error {
	file, err := appFs.TempFile(filepath.Dir(path), "."+filepath.Base(path)+".")
	if err != nil {
		return fmt.Errorf("could not open %s: %w", path, err)
	}
	defer func() {
		name := file.Name()
		file.Close()
		if err != nil {
			appFs.Remove(name)
		}
	}()
	writer := transform.NewWriter(file, unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder())
	if err = WriteShimFallback(writer, entries); err != nil {
		return err
	}
	if err := appFs.Rename(file.Name(), path); err != nil {
		return err
	}

	return err
}

// WriteShimFallback writes out a BOOT*.CSV for the shim fallback loader to the specified writer.
// The output of this function is unencoded, use a transformed UTF-16 writer.
func WriteShimFallback(w io.Writer, entries []BootEntry) error {
	// sigh, fallback prepends entries to the boot order so last line comes first, so we
	// need to write out the lines in reverse boot order.
	for i := len(entries); i > 0; i-- {
		entry := entries[i-1]
		if strings.Contains(entry.Filename, ",") ||
			strings.Contains(entry.Label, ",") ||
			strings.Contains(entry.Options, ",") ||
			strings.Contains(entry.Description, ",") {
			return fmt.Errorf("entry '%s' contains ',' in one of the attributes, this is not supported", entry.Label)
		}

		// We have an empty space after Options, because if there is no space in the options, shim
		// does not seem to parse them at all.
		var options = entry.Options
		if options != "" {
			options += " "
		}
		_, err := fmt.Fprintf(w, "%s,%s,%s,%s\n", entry.Filename, entry.Label, options, entry.Description)
		if err != nil {
			return fmt.Errorf("Could not write entry '%s' to file: %w", entry.Label, err)
		}
	}

	return nil
}

// InstallShim installs the shim into the given ESP for the given vendor
// It returns true if it installed the shim.
func InstallShim(esp string, source string, vendor string) (bool, error) {
	if err := appFs.MkdirAll(path.Join(esp, "EFI", "BOOT"), 0644); err != nil {
		return false, fmt.Errorf("Could not create BOOT directory on ESP: %w", err)
	}
	if err := appFs.MkdirAll(path.Join(esp, "EFI", vendor), 0644); err != nil {
		return false, fmt.Errorf("Could not create vendor directory on ESP: %w", err)
	}

	updatedAny := false
	shim := "shim" + GetEfiArchitecture() + ".efi"
	fb := "fb" + GetEfiArchitecture() + ".efi"
	mm := "mm" + GetEfiArchitecture() + ".efi"
	removable := "BOOT" + strings.ToUpper(GetEfiArchitecture()) + ".EFI"
	copies := map[string]string{
		path.Join(esp, "EFI", "BOOT", removable): shim + ".signed",
		path.Join(esp, "EFI", "BOOT", fb):        fb,
		path.Join(esp, "EFI", "BOOT", mm):        mm,
		path.Join(esp, "EFI", vendor, shim):      shim + ".signed",
		path.Join(esp, "EFI", vendor, fb):        fb,
		path.Join(esp, "EFI", vendor, mm):        mm,
	}
	for dst, src := range copies {
		updated, err := MaybeUpdateFile(dst, path.Join(source, src))
		if err != nil {
			return false, fmt.Errorf("Could not update file: %v", err)
		}
		updatedAny = updatedAny || updated
	}
	return updatedAny, nil
}
