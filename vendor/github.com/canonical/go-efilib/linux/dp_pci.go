// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	efi "github.com/canonical/go-efilib"
)

var classRE = regexp.MustCompile(`^0x([[:xdigit:]]+)$`)

// pciRE matches "nnnn:bb:dd:f" where "nnnn" is the domain, "bb" is the bus number,
// "dd" is the device number and "f" is the function. It captures the device and
// function.
var pciRE = regexp.MustCompile(`^[[:xdigit:]]{4}:[[:xdigit:]]{2}:([[:xdigit:]]{2})\.([[:digit:]]{1})$`)

func handlePCIDevicePathNode(state *devicePathBuilderState) error {
	component := state.PeekUnhandledSysfsComponents(1)

	m := pciRE.FindStringSubmatch(component)
	if len(m) == 0 {
		return fmt.Errorf("invalid component: %s", component)
	}

	devNum, _ := strconv.ParseUint(m[1], 16, 8)
	fun, _ := strconv.ParseUint(m[2], 10, 8)

	state.AdvanceSysfsPath(1)

	classBytes, err := os.ReadFile(filepath.Join(state.SysfsPath(), "class"))
	if err != nil {
		return fmt.Errorf("cannot read device class: %w", err)
	}

	var class []byte
	n, err := fmt.Sscanf(string(classBytes), "0x%x", &class)
	if err != nil {
		return fmt.Errorf("cannot decode device class: %w", err)
	}
	if n != 1 {
		return errors.New("invalid device class")
	}

	vendorBytes, err := os.ReadFile(filepath.Join(state.SysfsPath(), "vendor"))
	if err != nil {
		return fmt.Errorf("cannot read device vendor: %w", err)
	}

	var vendor uint16
	n, err = fmt.Sscanf(string(vendorBytes), "0x%04x", &vendor)
	if err != nil {
		return fmt.Errorf("cannot decode device vendor: %w", err)
	}
	if n != 1 {
		return errors.New("invalid device vendor")
	}

	switch {
	case vendor == 0x1af4:
		state.Interface = interfaceTypeVirtio
	case bytes.HasPrefix(class, []byte{0x01, 0x00}):
		state.Interface = interfaceTypeSCSI
	case bytes.HasPrefix(class, []byte{0x01, 0x01}):
		state.Interface = interfaceTypeIDE
	case bytes.HasPrefix(class, []byte{0x01, 0x06}):
		state.Interface = interfaceTypeSATA
	case bytes.HasPrefix(class, []byte{0x01, 0x08}):
		state.Interface = interfaceTypeNVME
	case bytes.HasPrefix(class, []byte{0x06, 0x04}):
		state.Interface = interfaceTypePCI
	default:
		return errUnsupportedDevice("unhandled device class: " + string(classBytes))
	}

	state.Path = append(state.Path, &efi.PCIDevicePathNode{
		Function: uint8(fun),
		Device:   uint8(devNum)})
	return nil
}
