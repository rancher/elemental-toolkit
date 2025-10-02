// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	efi "github.com/canonical/go-efilib"
)

// nvmeNSRe matches "nvme/nvme<ctrl_id>/nvme<ctrl_id>n<ns_id>", capturing ns_id
var nvmeNSRe = regexp.MustCompile(`^nvme\/nvme[[:digit:]]+\/nvme[[:digit:]]+n([[:digit:]]+)$`)

func handleNVMEDevicePathNode(state *devicePathBuilderState) error {
	if state.SysfsComponentsRemaining() < 3 {
		return errors.New("invalid path: not enough components")
	}

	components := state.PeekUnhandledSysfsComponents(3)
	m := nvmeNSRe.FindStringSubmatch(components)
	if len(m) == 0 {
		return errors.New("invalid path")
	}

	state.AdvanceSysfsPath(3)

	nsid, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return fmt.Errorf("cannot parse nsid: %w", err)
	}

	var euid efi.EUI64

	euidBuf, err := os.ReadFile(filepath.Join(state.SysfsPath(), "eui"))
	if os.IsNotExist(err) {
		euidBuf, err = os.ReadFile(filepath.Join(state.SysfsPath(), "device", "eui"))
	}
	switch {
	case os.IsNotExist(err):
		// Nothing to do
	case err != nil:
		return fmt.Errorf("cannot determine euid: %w", err)
	default:
		n, err := fmt.Sscanf(string(euidBuf), "%02x %02x %02x %02x %02x %02x %02x %02x",
			&euid[0], &euid[1], &euid[2], &euid[3], &euid[4], &euid[5], &euid[6], &euid[7])
		if err != nil {
			return fmt.Errorf("cannot parse euid: %w", err)
		}
		if n != 8 {
			return errors.New("invalid euid")
		}
	}

	state.Path = append(state.Path, &efi.NVMENamespaceDevicePathNode{
		NamespaceID:   uint32(nsid),
		NamespaceUUID: euid})
	return nil
}
