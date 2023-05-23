// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"

	"golang.org/x/xerrors"

	"github.com/canonical/go-efilib"
)

var classRE = regexp.MustCompile(`^0x([[:xdigit:]]+)$`)

// pciRE matches "nnnn:bb:dd:f" where "nnnn" is the domain, "bb" is the bus number,
// "dd" is the device number and "f" is the function. It captures the device and
// function.
var pciRE = regexp.MustCompile(`^[[:xdigit:]]{4}:[[:xdigit:]]{2}:([[:xdigit:]]{2})\.([[:digit:]]{1})$`)

func handlePCIDevicePathNode(builder devicePathBuilder) error {
	component := builder.next(1)

	m := pciRE.FindStringSubmatch(component)
	if len(m) == 0 {
		return fmt.Errorf("invalid component: %s", component)
	}

	devNum, _ := strconv.ParseUint(m[1], 16, 8)
	fun, _ := strconv.ParseUint(m[2], 10, 8)

	classBytes, err := ioutil.ReadFile(filepath.Join(builder.absPath(component), "class"))
	if err != nil {
		return xerrors.Errorf("cannot read device class: %w", err)
	}

	var class []byte
	if n, err := fmt.Sscanf(string(classBytes), "0x%x", &class); err != nil || n != 1 {
		return errors.New("cannot decode device class")
	}

	builder.advance(1)

	switch {
	case bytes.HasPrefix(class, []byte{0x01, 0x00}):
		builder.setInterfaceType(interfaceTypeSCSI)
	case bytes.HasPrefix(class, []byte{0x01, 0x01}):
		builder.setInterfaceType(interfaceTypeIDE)
	case bytes.HasPrefix(class, []byte{0x01, 0x06}):
		builder.setInterfaceType(interfaceTypeSATA)
	case bytes.HasPrefix(class, []byte{0x01, 0x08}):
		builder.setInterfaceType(interfaceTypeNVME)
	case bytes.HasPrefix(class, []byte{0x06, 0x04}):
		builder.setInterfaceType(interfaceTypePCI)
	default:
		return errUnsupportedDevice("unhandled device class: " + string(classBytes))
	}

	builder.append(&efi.PCIDevicePathNode{
		Function: uint8(fun),
		Device:   uint8(devNum)})
	return nil
}

func init() {
	registerDevicePathNodeHandler("pci", handlePCIDevicePathNode, 0, interfaceTypePCI)
}
