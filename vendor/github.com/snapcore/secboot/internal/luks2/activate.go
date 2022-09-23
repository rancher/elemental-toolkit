// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2020 Canonical Ltd
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

package luks2

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/snapcore/snapd/osutil"
)

var (
	systemdCryptsetupPath = "/lib/systemd/systemd-cryptsetup"
)

// Activate unlocks the LUKS device at sourceDevicePath using systemd-cryptsetup and creates a device
// mapping with the supplied volumeName. The device is unlocked using the supplied key.
func Activate(volumeName, sourceDevicePath string, key []byte) error {
	cmd := exec.Command(systemdCryptsetupPath, "attach", volumeName, sourceDevicePath, "/dev/stdin", "luks,tries=1")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "SYSTEMD_LOG_TARGET=console")
	cmd.Stdin = bytes.NewReader(key)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemd-cryptsetup failed with: %v", osutil.OutputErr(output, err))
	}

	return nil
}

// Deactivate detaches the LUKS volume with the supplied name.
func Deactivate(volumeName string) error {
	cmd := exec.Command(systemdCryptsetupPath, "detach", volumeName)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "SYSTEMD_LOG_TARGET=console")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemd-cryptsetup failed with: %v", osutil.OutputErr(output, err))
	}

	return nil
}
