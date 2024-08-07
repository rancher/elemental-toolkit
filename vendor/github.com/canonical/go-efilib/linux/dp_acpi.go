// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	efi "github.com/canonical/go-efilib"
)

// acpiIdRE matches a ACPI or PNP ID, capturing the vendor and product.
var acpiIdRE = regexp.MustCompile(`^([[:upper:][:digit:]]{3,4})([[:xdigit:]]{4})$`)

// acpiModaliasRE matches a modalias for an ACPI node, capturing the CID.
var acpiModaliasRE = regexp.MustCompile(`^acpi:[[:alnum:]]+:([[:alnum:]]*)`)

func maybeUseSimpleACPIDevicePathNode(node *efi.ACPIExtendedDevicePathNode) efi.DevicePathNode {
	if node.HIDStr != "" || node.UIDStr != "" || node.CIDStr != "" {
		return node
	}
	if node.CID != 0 && node.CID != node.HID {
		return node
	}
	return &efi.ACPIDevicePathNode{HID: node.HID, UID: node.UID}
}

func decodeACPIOrPNPId(str string) (efi.EISAID, string) {
	m := acpiIdRE.FindStringSubmatch(str)
	if len(m) == 0 {
		return 0, str
	}

	vendor := m[1]
	p, _ := hex.DecodeString(m[2])
	product := binary.BigEndian.Uint16(p)

	if len(vendor) != 3 {
		return 0, fmt.Sprintf("%s%04x", vendor, product)
	}

	id, _ := efi.NewEISAID(vendor, product)
	return id, ""
}

func newACPIExtendedDevicePathNode(path string) (*efi.ACPIExtendedDevicePathNode, error) {
	node := new(efi.ACPIExtendedDevicePathNode)

	hidBytes, err := os.ReadFile(filepath.Join(path, "hid"))
	if err != nil {
		return nil, err
	}

	hid, hidStr := decodeACPIOrPNPId(strings.TrimSpace(string(hidBytes)))
	node.HID = hid
	node.HIDStr = hidStr

	modalias, err := os.ReadFile(filepath.Join(path, "modalias"))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, err
	default:
		m := acpiModaliasRE.FindSubmatch(modalias)
		if len(m) == 0 {
			return nil, errors.New("invalid modalias")
		}
		if len(m[1]) > 0 {
			cid, cidStr := decodeACPIOrPNPId(string(m[1]))
			node.CID = cid
			node.CIDStr = cidStr
		}
	}

	uidBytes, err := os.ReadFile(filepath.Join(path, "uid"))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, err
	default:
		uidStr := strings.TrimSpace(string(uidBytes))
		uid, err := strconv.ParseUint(uidStr, 10, 32)
		if err != nil {
			node.UIDStr = uidStr
		} else {
			node.UID = uint32(uid)
		}
	}

	return node, nil
}

func handleACPIDevicePathNode(state *devicePathBuilderState) error {
	state.AdvanceSysfsPath(1)

	subsystem, err := filepath.EvalSymlinks(filepath.Join(state.SysfsPath(), "subsystem"))
	switch {
	case os.IsNotExist(err):
		return errSkipDevicePathNodeHandler
	case err != nil:
		return err
	}

	if subsystem != filepath.Join(sysfsPath, "bus", "acpi") {
		return errSkipDevicePathNodeHandler
	}

	return nil
}
