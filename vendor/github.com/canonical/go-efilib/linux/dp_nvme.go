// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"golang.org/x/xerrors"

	"github.com/canonical/go-efilib"
)

// nvmeNSRe matches "nvme/nvme<ctrl_id>/nvme<ctrl_id>n<ns_id>", capturing ns_id
var nvmeNSRe = regexp.MustCompile(`^nvme\/nvme[[:digit:]]+\/nvme[[:digit:]]+n([[:digit:]]+)$`)

func handleNVMEDevicePathNode(builder devicePathBuilder) error {
	if builder.numRemaining() < 3 {
		return errors.New("invalid path: not enough components")
	}

	components := builder.next(3)
	m := nvmeNSRe.FindStringSubmatch(components)
	if len(m) == 0 {
		return errors.New("invalid path")
	}

	builder.advance(3)

	nsid, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return xerrors.Errorf("cannot parse nsid: %w", err)
	}

	var euid [8]uint8

	euidBuf, err := ioutil.ReadFile(filepath.Join(builder.absPath(components), "eui"))
	if os.IsNotExist(err) {
		euidBuf, err = ioutil.ReadFile(filepath.Join(builder.absPath(components), "device", "eui"))
	}
	switch {
	case os.IsNotExist(err):
		// Nothing to do
	case err != nil:
		return xerrors.Errorf("cannot determine euid: %w", err)
	default:
		n, err := fmt.Sscanf(string(euidBuf), "%02x %02x %02x %02x %02x %02x %02x %02x",
			&euid[0], &euid[1], &euid[2], &euid[3], &euid[4], &euid[5], &euid[6], &euid[7])
		if err != nil {
			return xerrors.Errorf("cannot parse euid: %w", err)
		}
		if n != 8 {
			return errors.New("invalid euid")
		}
	}

	builder.append(&efi.NVMENamespaceDevicePathNode{
		NamespaceID:   uint32(nsid),
		NamespaceUUID: uint64(binary.LittleEndian.Uint64(euid[:]))})
	return nil
}

func init() {
	registerDevicePathNodeHandler("nvme", handleNVMEDevicePathNode, 0, interfaceTypeNVME)
}
