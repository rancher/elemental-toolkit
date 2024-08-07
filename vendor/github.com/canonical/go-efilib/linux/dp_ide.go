// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"errors"
	"fmt"
	"math"

	efi "github.com/canonical/go-efilib"
)

func handleIDEDevicePathNode(state *devicePathBuilderState) error {
	if state.SysfsComponentsRemaining() < 6 {
		return errors.New("invalid path: insufficient components")
	}

	state.AdvanceSysfsPath(6)

	params, err := handleATAPath(state.SysfsPath())
	if err != nil {
		return err
	}

	// PATA has a maximum of 2 ports.
	if params.port < 1 || params.port > 2 {
		return fmt.Errorf("invalid port: %d", params.port)
	}

	// Each PATA device is represented in the SCSI layer by setting the
	// target to the drive number, and the LUN as the LUN (see
	// drivers/ata/libata-scsi.c:ata_scsi_scan_host).

	// The channel is always 0 for PATA devices (no port multiplier).
	if params.channel != 0 {
		return errors.New("invalid SCSI channel")
	}
	if params.target > 1 {
		return errors.New("invalid drive")
	}
	if params.lun > math.MaxUint16 {
		return errors.New("invalid LUN")
	}

	state.Path = append(state.Path, &efi.ATAPIDevicePathNode{
		Controller: efi.ATAPIControllerRole(params.port - 1),
		Drive:      efi.ATAPIDriveRole(params.target),
		LUN:        uint16(params.lun)})
	return nil
}
