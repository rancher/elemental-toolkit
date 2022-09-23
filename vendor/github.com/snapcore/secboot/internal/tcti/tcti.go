// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
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

package tcti

import (
	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/linux"
)

const (
	// FIXME: This is fine during initial install and early boot, but we should strive to use the resource manager at other times.
	tpmPath = "/dev/tpm0"
)

// OpenDefaultTcti connects to the default TPM character device. This can be overridden for tests to connect to a simulator device.
var OpenDefault = func() (tpm2.TCTI, error) {
	return linux.OpenDevice(tpmPath)
}
