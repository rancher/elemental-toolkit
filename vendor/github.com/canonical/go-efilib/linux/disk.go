// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"errors"
	"math"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/xerrors"

	efi "github.com/canonical/go-efilib"
	internal_unix "github.com/canonical/go-efilib/internal/unix"
)

func getSectorSize(f *os.File) (int64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}

	if fi.Mode().IsRegular() {
		return 512, nil
	}

	if fi.Mode()&os.ModeDevice == 0 {
		return 0, errors.New("not a regular file or device")
	}

	sz, err := unix.IoctlGetInt(int(f.Fd()), unix.BLKSSZGET)
	if err != nil {
		return 0, err
	}
	return int64(sz), nil
}

func getDeviceSize(f *os.File) (int64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}

	if fi.Mode().IsRegular() {
		return fi.Size(), nil
	}

	if fi.Mode()&os.ModeDevice == 0 {
		return 0, errors.New("not a regular file or device")
	}

	sz, err := internal_unix.IoctlGetUint64(int(f.Fd()), unix.BLKGETSIZE64)
	switch {
	case err == syscall.ENOTTY:
		n, err := internal_unix.IoctlGetUint(int(f.Fd()), unix.BLKGETSIZE)
		if err != nil {
			return 0, err
		}
		if int64(n) > int64(math.MaxInt64>>9) {
			return 0, errors.New("overflow")
		}
		return int64(n << 9), nil
	case err != nil:
		return 0, err
	case sz > math.MaxInt64:
		return 0, errors.New("overflow")
	default:
		return int64(sz), nil
	}
}

// NewHardDriveDevicePathNodeFromDevice constructs a HardDriveDevicePathNode for the
// specified partition on the device or file at the supplied path.
func NewHardDriveDevicePathNodeFromDevice(dev string, part int) (*efi.HardDriveDevicePathNode, error) {
	f, err := osOpen(dev)
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

	return efi.NewHardDriveDevicePathNodeFromDevice(f, sz, ssz, part)
}
