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
	mountUtils "k8s.io/mount-utils"
	"os"
	"strings"
)

type Chroot struct {
	path string
	defaultMounts []string
	mounter mountUtils.Interface
}

func NewChroot(mounter mountUtils.Interface,path string) *Chroot {
	return &Chroot{
		path: path,
		defaultMounts: []string{"/dev", "/dev/pts", "/proc", "/sys"},
		mounter: mounter,
	}
}

func (c Chroot) Prepare() error {
	mountOptions := []string{"bind"}
	for _, mnt := range c.defaultMounts {
		mountPoint := fmt.Sprintf("%s%s", strings.TrimSuffix(c.path, "/"), mnt)
		err := os.Mkdir(mountPoint, 0644)
		err = c.mounter.Mount(mnt, mountPoint, "bind", mountOptions)
		if err != nil {return err}
	}
	return nil
}

func (c Chroot) Close() error {
	for _, mnt := range c.defaultMounts {
		err := c.mounter.Unmount(fmt.Sprintf("%s%s", strings.TrimSuffix(c.path, "/"), mnt))
		if err != nil {return err}
	}
	return nil
}

// Run executes a command inside a chroot
func (c Chroot) Run(runner v1.Runner, sysCallInterface v1.SyscallInterface, command string, args ...string)  ([]byte, error){
	var out []byte
	var err error
	// Store current dir
	oldRootF, err := os.Open("/")
	defer oldRootF.Close()
	if err != nil {fmt.Printf("Cant open /");return out, err}
	err = c.Prepare()
	if err != nil {fmt.Printf("Cant mount default mounts");return nil, err}
	err = sysCallInterface.Chroot(c.path)
	if err != nil {fmt.Printf("Cant chroot %s", c.path);return out, err}
	// run commands in the chroot
	out, err = runner.Run(command, args...)
	if err != nil {fmt.Printf("Cant run command on chroot");return out, err}
	// Restore to old dir
	err = oldRootF.Chdir()
	if err != nil {fmt.Printf("Cant change to old dir");return out, err}
	err = sysCallInterface.Chroot(".")
	if err != nil {fmt.Printf("Cant chroot back to oldir");return out, err}
	return out, err
}
