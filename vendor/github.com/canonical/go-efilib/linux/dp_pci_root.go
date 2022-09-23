// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// pcirootRE matches a pcixxxx.xx path component.
var pcirootRE = regexp.MustCompile(`^pci[[:xdigit:]]{4}:[[:xdigit:]]{2}$`)

func handlePCIRootDevicePathNode(builder devicePathBuilder) error {
	component := builder.next(1)

	if !pcirootRE.MatchString(component) {
		return errSkipDevicePathNodeHandler
	}

	node, err := newACPIExtendedDevicePathNode(filepath.Join(builder.absPath(component), "firmware_node"))
	if err != nil {
		return err
	}
	if node.HID.Vendor() != "PNP" || (node.HID.Product() != 0x0a03 && node.HID.Product() != 0x0a08) {
		return fmt.Errorf("unexpected hid: %v", node.HID)
	}
	node.HID = 0x0a0341d0

	if node.CID != 0 && (node.CID.Vendor() != "PNP" || (node.CID.Product() != 0x0a03 && node.CID.Product() != 0x0a08)) {
		return fmt.Errorf("unexpected cid: %v", node.CID)
	}

	builder.advance(1)

	builder.setInterfaceType(interfaceTypePCI)
	builder.append(maybeUseSimpleACPIDevicePathNode(node))

	return nil
}

func init() {
	registerDevicePathNodeHandler("pci-root", handlePCIRootDevicePathNode, prependHandler)
}
