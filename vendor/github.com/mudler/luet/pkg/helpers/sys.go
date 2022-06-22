// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package helpers

import (
	"os"
	"os/exec"
	"os/user"
	"syscall"

	"github.com/pkg/errors"
)

// This allows a multi-platform switch in the future
func Exec(cmd string, args []string, env []string) error {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return errors.Wrap(err, "Could not find binary in path: "+cmd)
	}
	return syscall.Exec(path, args, env)
}

func GetHomeDir() (ans string) {
	// os/user doesn't work in from scratch environments
	u, err := user.Current()
	if err == nil {
		ans = u.HomeDir
	} else {
		ans = ""
	}
	if os.Getenv("HOME") != "" {
		ans = os.Getenv("HOME")
	}
	return ans
}
