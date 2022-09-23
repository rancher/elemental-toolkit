// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2021 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package secboot

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/snapcore/secboot/internal/keyring"

	"golang.org/x/xerrors"
)

const (
	keyringPurposeAuxiliary  = "aux"
	keyringPurposeDiskUnlock = "unlock"
)

var ErrKernelKeyNotFound = errors.New("cannot find key in kernel keyring")

func keyringPrefixOrDefault(prefix string) string {
	if prefix == "" {
		return "ubuntu-fde"
	}
	return prefix
}

// GetDiskUnlockKeyFromKernel retrieves the key that was used to unlock the
// encrypted container at the specified path. The value of prefix must match
// the prefix that was supplied via ActivateVolumeOptions during unlocking.
//
// If remove is true, the key will be removed from the kernel keyring prior
// to returning.
//
// If no key is found, a ErrKernelKeyNotFound error will be returned.
func GetDiskUnlockKeyFromKernel(prefix, devicePath string, remove bool) (DiskUnlockKey, error) {
	key, err := keyring.GetKeyFromUserKeyring(devicePath, keyringPurposeDiskUnlock, keyringPrefixOrDefault(prefix))
	if err != nil {
		var e syscall.Errno
		if xerrors.As(err, &e) && e == syscall.ENOKEY {
			return nil, ErrKernelKeyNotFound
		}
		return nil, err

	}

	if remove {
		if err := keyring.RemoveKeyFromUserKeyring(devicePath, keyringPurposeDiskUnlock, keyringPrefixOrDefault(prefix)); err != nil {
			fmt.Fprintf(os.Stderr, "secboot: cannot remove key from keyring: %v\n", err)
		}
	}

	return key, nil
}

// GetAuxiliaryKeyFromKernel retrieves the auxiliary key associated with the
// KeyData that was used to unlock the encrypted container at the specified path.
// The value of prefix must match the prefix that was supplied via
// ActivateVolumeOptions during unlocking.
//
// If remove is true, the key will be removed from the kernel keyring prior
// to returning.
//
// If no key is found, a ErrKernelKeyNotFound error will be returned.
func GetAuxiliaryKeyFromKernel(prefix, devicePath string, remove bool) (AuxiliaryKey, error) {
	key, err := keyring.GetKeyFromUserKeyring(devicePath, keyringPurposeAuxiliary, keyringPrefixOrDefault(prefix))
	if err != nil {
		var e syscall.Errno
		if xerrors.As(err, &e) && e == syscall.ENOKEY {
			return nil, ErrKernelKeyNotFound
		}
		return nil, err

	}

	if remove {
		if err := keyring.RemoveKeyFromUserKeyring(devicePath, keyringPurposeAuxiliary, keyringPrefixOrDefault(prefix)); err != nil {
			fmt.Fprintf(os.Stderr, "secboot: cannot remove key from keyring: %v\n", err)
		}
	}

	return key, nil
}
