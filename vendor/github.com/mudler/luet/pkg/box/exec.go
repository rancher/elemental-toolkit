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

package box

import (
	b64 "encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"github.com/pkg/errors"
)

type Box interface {
	Run() error
	Exec() error
}

type DefaultBox struct {
	Name                  string
	Root                  string
	Env                   []string
	Cmd                   string
	Args                  []string
	HostMounts            []string
	Stdin, Stdout, Stderr bool
}

func NewBox(cmd string, args, hostmounts, env []string, rootfs string, stdin, stdout, stderr bool) Box {
	return &DefaultBox{
		Stdin:      stdin,
		Stdout:     stdout,
		Stderr:     stderr,
		Cmd:        cmd,
		Args:       args,
		Root:       rootfs,
		HostMounts: hostmounts,
		Env:        env,
	}
}

func (b *DefaultBox) Exec() error {

	if err := mountProc(b.Root); err != nil {
		return errors.Wrap(err, "Failed mounting proc on rootfs")
	}
	if err := mountDev(b.Root); err != nil {
		return errors.Wrap(err, "Failed mounting dev on rootfs")
	}

	for _, hostMount := range b.HostMounts {
		target := hostMount
		if strings.Contains(hostMount, ":") {
			dest := strings.Split(hostMount, ":")
			if len(dest) != 2 {
				return errors.New("Invalid arguments for mount, it can be: fullpath, or source:target")
			}
			hostMount = dest[0]
			target = dest[1]
		}
		if err := mountBind(hostMount, b.Root, target); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed mounting %s on rootfs", hostMount))
		}
	}

	if err := PivotRoot(b.Root); err != nil {
		return errors.Wrap(err, "Failed switching pivot on rootfs")
	}
	cmd := exec.Command(b.Cmd, b.Args...)

	if b.Stdin {
		cmd.Stdin = os.Stdin
	}

	if b.Stderr {
		cmd.Stderr = os.Stderr
	}

	if b.Stdout {
		cmd.Stdout = os.Stdout
	}

	cmd.Env = b.Env

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error running the %s command in box.Exec", b.Cmd))
	}
	return nil
}

func (b *DefaultBox) Run() error {

	if !fileHelper.Exists(b.Root) {
		return errors.New(b.Root + " does not exist")
	}

	// This matches with exec CLI command in luet
	// TODO: Pass by env var as well
	execCmd := []string{"exec", "--rootfs", b.Root, "--entrypoint", b.Cmd}

	if b.Stdin {
		execCmd = append(execCmd, "--stdin")
	}

	if b.Stderr {
		execCmd = append(execCmd, "--stderr")
	}

	if b.Stdout {
		execCmd = append(execCmd, "--stdout")
	}
	// Encode the command in base64 to avoid bad input from the args given
	execCmd = append(execCmd, "--decode")

	for _, m := range b.HostMounts {
		execCmd = append(execCmd, "--mount")
		execCmd = append(execCmd, m)
	}

	for _, e := range b.Env {
		execCmd = append(execCmd, "--env")
		execCmd = append(execCmd, e)
	}

	for _, a := range b.Args {
		execCmd = append(execCmd, b64.StdEncoding.EncodeToString([]byte(a)))
	}

	cmd := exec.Command("/proc/self/exe", execCmd...)
	if b.Stdin {
		cmd.Stdin = os.Stdin
	}

	if b.Stderr {
		cmd.Stderr = os.Stderr
	}

	if b.Stdout {
		cmd.Stdout = os.Stdout
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "Failed running Box command in box.Run")
	}
	return nil
}
