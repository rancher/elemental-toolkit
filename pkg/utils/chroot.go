/*
Copyright © 2021 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"os"
	"strings"
)

// Chroot represents the struct that will allow us to run commands inside a given chroot
type Chroot struct {
	path          string
	defaultMounts []string
	config        *v1.RunConfig
}

func NewChroot(path string, config *v1.RunConfig) *Chroot {
	return &Chroot{
		path:          path,
		defaultMounts: []string{"/dev", "/dev/pts", "/proc", "/sys"},
		config:        config,
	}
}

// Prepare will mount the defaultMounts as bind mounts, to be ready when we run chroot
func (c Chroot) Prepare() error {
	mountOptions := []string{"bind"}
	for _, mnt := range c.defaultMounts {
		mountPoint := fmt.Sprintf("%s%s", strings.TrimSuffix(c.path, "/"), mnt)
		err := c.config.Fs.Mkdir(mountPoint, 0644)
		err = c.config.Mounter.Mount(mnt, mountPoint, "bind", mountOptions)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close will unount the defaultMounts that we mounted on Prepare so everything is left clean
func (c Chroot) Close() error {
	for _, mnt := range c.defaultMounts {
		err := c.config.Mounter.Unmount(fmt.Sprintf("%s%s", strings.TrimSuffix(c.path, "/"), mnt))
		if err != nil {
			return err
		}
	}
	return nil
}

// Run executes a command inside a chroot
func (c Chroot) Run(command string, args ...string) ([]byte, error) {
	var out []byte
	var err error
	// Store current dir
	oldRootF, err := os.Open("/") // Can't use afero here because doesn't support chdir done below
	defer oldRootF.Close()
	if err != nil {
		c.config.Logger.Errorf("Cant open /")
		return out, err
	}
	err = c.Prepare()
	if err != nil {
		c.config.Logger.Errorf("Cant mount default mounts")
		return nil, err
	}
	err = c.config.Syscall.Chroot(c.path)
	if err != nil {
		c.config.Logger.Errorf("Cant chroot %s", c.path)
		return out, err
	}
	// run commands in the chroot
	out, err = c.config.Runner.Run(command, args...)
	if err != nil {
		c.config.Logger.Errorf("Cant run command on chroot")
		return out, err
	}
	// Restore to old dir
	err = oldRootF.Chdir()
	if err != nil {
		c.config.Logger.Errorf("Cant change to old dir")
		return out, err
	}
	err = c.config.Syscall.Chroot(".")
	if err != nil {
		c.config.Logger.Errorf("Cant chroot back to old dir")
		return out, err
	}
	return out, err
}
