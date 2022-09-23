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

package keyring

import (
	"golang.org/x/sys/unix"
	"golang.org/x/xerrors"
)

const (
	userKeyType = "user"
	userKeyring = -4
)

func formatDesc(devicePath, purpose, prefix string) string {
	return prefix + ":" + devicePath + ":" + purpose
}

func AddKeyToUserKeyring(key []byte, devicePath, purpose, prefix string) error {
	_, err := unix.AddKey(userKeyType, formatDesc(devicePath, purpose, prefix), key, userKeyring)
	return err
}

func GetKeyFromUserKeyring(devicePath, purpose, prefix string) ([]byte, error) {
	id, err := unix.KeyctlSearch(userKeyring, userKeyType, formatDesc(devicePath, purpose, prefix), 0)
	if err != nil {
		return nil, xerrors.Errorf("cannot find key: %w", err)
	}

	sz, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, nil, 0)
	if err != nil {
		return nil, xerrors.Errorf("cannot determine size of key payload: %w", err)
	}

	key := make([]byte, sz)
	if _, err = unix.KeyctlBuffer(unix.KEYCTL_READ, id, key, 0); err != nil {
		return nil, xerrors.Errorf("cannot read key payload: %w", err)
	}

	return key, nil
}

func RemoveKeyFromUserKeyring(devicePath, purpose, prefix string) error {
	id, err := unix.KeyctlSearch(userKeyring, userKeyType, formatDesc(devicePath, purpose, prefix), 0)
	if err != nil {
		return xerrors.Errorf("cannot find key: %w", err)
	}

	_, err = unix.KeyctlInt(unix.KEYCTL_UNLINK, id, userKeyring, 0, 0)
	return err
}
