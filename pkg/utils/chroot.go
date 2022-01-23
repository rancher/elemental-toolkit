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
	"errors"
	"fmt"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"os"
	"sort"
	"strings"
)

// Chroot represents the struct that will allow us to run commands inside a given chroot
type Chroot struct {
	path          string
	defaultMounts []string
	extraMounts   map[string]string
	activeMounts  []string
	config        *v1.RunConfig
}

func NewChroot(path string, config *v1.RunConfig) *Chroot {
	return &Chroot{
		path:          path,
		defaultMounts: []string{"/dev", "/dev/pts", "/proc", "/sys"},
		extraMounts:   map[string]string{},
		activeMounts:  []string{},
		config:        config,
	}
}

// Sets additional bind mounts for the chroot enviornment. They are represented
// in a map where the key is the path outside the chroot and the value is the
// path inside the chroot.
func (c *Chroot) SetExtraMounts(extraMounts map[string]string) {
	c.extraMounts = extraMounts
}

// Prepare will mount the defaultMounts as bind mounts, to be ready when we run chroot
func (c *Chroot) Prepare() error {
	var err error
	keys := []string{}
	mountOptions := []string{"bind"}

	if len(c.activeMounts) > 0 {
		return errors.New("There are already active mountpoints for this instance")
	}

	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	for _, mnt := range c.defaultMounts {
		mountPoint := fmt.Sprintf("%s%s", strings.TrimSuffix(c.path, "/"), mnt)
		err = c.config.Fs.MkdirAll(mountPoint, 0755)
		if err != nil {
			return err
		}
		err = c.config.Mounter.Mount(mnt, mountPoint, "bind", mountOptions)
		if err != nil {
			return err
		}
		c.activeMounts = append(c.activeMounts, mountPoint)
	}

	for k := range c.extraMounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		mountPoint := fmt.Sprintf("%s%s", strings.TrimSuffix(c.path, "/"), c.extraMounts[k])
		err = c.config.Fs.MkdirAll(mountPoint, 0755)
		if err != nil {
			return err
		}
		err = c.config.Mounter.Mount(k, mountPoint, "bind", mountOptions)
		if err != nil {
			return err
		}
		c.activeMounts = append(c.activeMounts, mountPoint)
	}
	return nil
}

// Close will unmount all active mounts created in Prepare on reverse order
func (c *Chroot) Close() error {
	failures := []string{}
	for len(c.activeMounts) > 0 {
		curr := c.activeMounts[len(c.activeMounts)-1]
		c.activeMounts = c.activeMounts[:len(c.activeMounts)-1]
		err := c.config.Mounter.Unmount(curr)
		if err != nil {
			c.config.Logger.Errorf("Error unmounting %s", curr)
			failures = append(failures, curr)
		}
	}
	if len(failures) > 0 {
		c.activeMounts = failures
		return errors.New(fmt.Sprintf("Failed closing chroot environment. Unmount failures: %v", failures))
	}
	return nil
}

// Run executes a command inside a chroot
func (c *Chroot) Run(command string, args ...string) (out []byte, err error) {
	// Store current root
	oldRootF, err := os.Open("/") // Can't use afero here because doesn't support chdir done below
	defer oldRootF.Close()
	if err != nil {
		c.config.Logger.Errorf("Cant open /")
		return nil, err
	}
	if len(c.activeMounts) == 0 {
		err = c.Prepare()
		if err != nil {
			c.config.Logger.Errorf("Cant mount default mounts")
			return nil, err
		}
		defer func() {
			tmpErr := c.Close()
			if err == nil {
				err = tmpErr
			}
		}()
	}
	err = c.config.Syscall.Chroot(c.path)
	if err != nil {
		c.config.Logger.Errorf("Cant chroot %s", c.path)
		return nil, err
	}

	// Restore to old root
	defer func() {
		tmpErr := oldRootF.Chdir()
		if tmpErr != nil {
			c.config.Logger.Errorf("Cant change to old root dir")
			if err == nil {
				err = tmpErr
			}
		} else {
			tmpErr = c.config.Syscall.Chroot(".")
			if tmpErr != nil {
				c.config.Logger.Errorf("Cant chroot back to old root")
				if err == nil {
					err = tmpErr
				}
			}
		}
	}()

	// run command in the chroot
	out, err = c.config.Runner.Run(command, args...)
	if err != nil {
		c.config.Logger.Errorf("Cant run command on chroot")
		return out, err
	}

	return out, err
}
