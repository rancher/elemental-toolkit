// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"regexp"
)

var virtioRE = regexp.MustCompile(`^virtio[[:digit:]]`)

func handleVirtioDevicePathNode(builder devicePathBuilder) error {
	if !virtioRE.MatchString(builder.next(1)) {
		return errSkipDevicePathNodeHandler
	}

	builder.advance(1)
	return nil
}

func init() {
	registerDevicePathNodeHandler("virtio", handleVirtioDevicePathNode, prependHandler, interfaceTypeSCSI)
}
