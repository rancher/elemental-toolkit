// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"os"

	"golang.org/x/sys/unix"
)

var (
	removeVarFile = os.Remove
	unixStatfs    = unix.Statfs
)
