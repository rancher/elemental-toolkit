// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

func handleVirtualDevicePathNode(builder devicePathBuilder) error {
	if builder.next(1) == "virtual" {
		return errUnsupportedDevice("virtual devices are not supported")
	}
	return errSkipDevicePathNodeHandler
}

func init() {
	registerDevicePathNodeHandler("virtual", handleVirtualDevicePathNode, 0)
}
