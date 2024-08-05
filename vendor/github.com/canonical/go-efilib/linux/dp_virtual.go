// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

func handleVirtualDevicePathNode(state *devicePathBuilderState) error {
	if state.PeekUnhandledSysfsComponents(1) == "virtual" {
		return errUnsupportedDevice("virtual devices are not supported")
	}
	return errSkipDevicePathNodeHandler
}
