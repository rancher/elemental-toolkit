// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"os"
	"path/filepath"
)

var (
	mountsPath = "/proc/self/mountinfo"
	sysfsPath  = "/sys"

	filepathEvalSymlinks = filepath.EvalSymlinks
	osOpen               = os.Open
)
