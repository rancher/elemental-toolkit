// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"fmt"
	"regexp"
)

// vmbusrootRE matches a VMBUS:XX component.
var vmbusrootRE = regexp.MustCompile(`^VMBUS:[[:xdigit:]]{2}$`)

func handleVMBusRootDevicePathNode(builder devicePathBuilder) error {
	component := builder.next(1)

	if !vmbusrootRE.MatchString(component) {
		return errSkipDevicePathNodeHandler
	}

	node, err := newACPIExtendedDevicePathNode(builder.absPath(component))
	if err != nil {
		return err
	}
	if node.HID != 0 || node.CID != 0 || node.HIDStr != "VMBUS" || node.CIDStr != "" {
		return fmt.Errorf("unexpected node properties: %v", node)
	}

	// The hardware ID exposed by the kernel seems to be capitalized, but the
	// one exposed from the firmware on an instance I've tested on isn't. Fix
	// up here - I'm not sure if this is right (is it always "VMBus"?), but the
	// device path does need to be an exact match for lookups because the firmware
	// essentially just does a memcmp.
	node.HIDStr = "VMBus"

	builder.advance(1)

	builder.setInterfaceType(interfaceTypeVMBus)
	builder.append(maybeUseSimpleACPIDevicePathNode(node))
	return nil
}

func init() {
	registerDevicePathNodeHandler("vmbus-root", handleVMBusRootDevicePathNode, prependHandler)
}
