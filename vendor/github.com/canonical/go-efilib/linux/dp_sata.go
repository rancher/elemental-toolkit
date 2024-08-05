// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	efi "github.com/canonical/go-efilib"
)

func handleSATADevicePathNode(state *devicePathBuilderState) error {
	if state.SysfsComponentsRemaining() < 6 {
		return errors.New("invalid path: insufficient components")
	}

	state.AdvanceSysfsPath(6)

	params, err := handleATAPath(state.SysfsPath())
	if err != nil {
		return err
	}

	// Each SATA device is represented in the SCSI layer by setting the
	// channel to the port multiplier port number and the LUN as the LUN (see
	// drivers/ata/libata-scsi.c:ata_scsi_scan_host).

	pmp := params.channel
	if pmp > 0x7fff {
		return errors.New("invalid PMP")
	}

	// The target is always zero for SATA devices, as each port only has
	// a single device.
	if params.target != 0 {
		return errors.New("invalid SCSI target")
	}

	// We need to determine if the device is connected via a port
	// multiplier because we have to set the PMP address to 0xffff
	// if it isn't. Unfortunately, it is zero indexed so checking
	// that it is zero isn't sufficient.
	//
	// The kernel will expose a single host link%d device if there
	// is no port multiplier, or one of more PMP link%d.%d devices
	// if there is a port multiplier attached (see
	// drivers/ata/libata-pmp.c:sata_pmp_init_links and
	// drivers/ata/libata-transport.c:ata_tlink_add).
	_, err = os.Stat(filepath.Join(state.SysfsPath(), "../../../../..", fmt.Sprintf("link%d.%d", params.printId, pmp)))
	switch {
	case os.IsNotExist(err):
		// No port multiplier is connected.
		pmp = 0xffff
	case err != nil:
		return err
	default:
		// A port multiplier is connected.
	}

	state.Path = append(state.Path, &efi.SATADevicePathNode{
		// The kernel provides a one-indexed number and the firmware is zero-indexed.
		HBAPortNumber:            uint16(params.port) - 1,
		PortMultiplierPortNumber: uint16(pmp),
		LUN:                      uint16(params.lun)})
	return nil
}
