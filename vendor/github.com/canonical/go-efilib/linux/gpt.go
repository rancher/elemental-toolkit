// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"os"

	"golang.org/x/xerrors"

	efi "github.com/canonical/go-efilib"
)

// ReadPartitionTable reads a complete GUID partition table from the supplied
// device path.
//
// This function expects the device to have a valid protective MBR.
//
// If role is efi.PrimaryPartitionTable, this will read the primary partition
// table that is located immediately after the protective MBR. If role is
// efi.BackupPartitionTable, this will read the backup partition table that is
// located at the end of the device.
//
// If checkCrc is true and either CRC check fails for the requested table, an
// error will be returned. Setting checkCrc to false disables the CRC checks.
//
// Note that whilst this function checks the integrity of the header and
// partition table entries, it does not check the contents of the partition
// table entries.
//
// If role is efi.BackupPartitionTable and the backup table is not located at
// the end of the device, this will return efi.ErrInvalidBackupPartitionTableLocation
// along with the valid table.
func ReadPartitionTable(path string, role efi.PartitionTableRole, checkCrc bool) (*efi.PartitionTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sz, err := getDeviceSize(f)
	if err != nil {
		return nil, xerrors.Errorf("cannot determine device size: %w", err)
	}

	ssz, err := getSectorSize(f)
	if err != nil {
		return nil, xerrors.Errorf("cannot determine logical sector size: %w", err)
	}

	return efi.ReadPartitionTable(f, sz, ssz, role, checkCrc)
}
