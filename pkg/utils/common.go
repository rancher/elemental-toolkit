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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/spf13/afero"
	"github.com/zloylos/grsync"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// tmpBlockdevices is a temporal struct to extract the output of lsblk json
type tmpBlockdevices struct {
	Blockdevices []v1.Partition
}

func CommandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// BootedFrom will check if we are booting from the given label
func BootedFrom(runner v1.Runner, label string) bool {
	out, _ := runner.Run("cat", "/proc/cmdline")
	return strings.Contains(string(out), label)
}

// GetDeviceByLabel will try to return the device that matches the given label.
// attempts value sets the number of attempts to find the device, it
// waits a second between attempts.
func GetDeviceByLabel(runner v1.Runner, label string, attempts int) (string, error) {
	for tries := 0; tries < attempts; tries++ {
		runner.Run("udevadm", "settle")
		out, err := runner.Run("blkid", "--label", label)
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return strings.TrimSpace(string(out)), nil
		}
		time.Sleep(1 * time.Second)
	}
	return "", errors.New("no device found")
}

// GetFullDeviceByLabel works like GetDeviceByLabel, but it will try to get as much info as possible from the existing
// partition and return a v1.Partition object
func GetFullDeviceByLabel(runner v1.Runner, label string, attempts int) (v1.Partition, error) {
	for tries := 0; tries < attempts; tries++ {
		_, _ = runner.Run("udevadm", "settle")
		out, err := runner.Run("blkid", "--label", label)
		device := strings.TrimSpace(string(out))
		if err == nil && device != "" {
			out, err = runner.Run("lsblk", "-b", "-n", "-J", "--output", "LABEL,SIZE,FSTYPE,MOUNTPOINT,PATH", device)
			if err == nil && strings.TrimSpace(string(out)) != "" {
				a := tmpBlockdevices{}
				err = json.Unmarshal(out, &a)
				if err != nil {
					return v1.Partition{}, err
				}
				return a.Blockdevices[0], nil
			} else {
				return v1.Partition{}, err
			}
		}
		time.Sleep(1 * time.Second)
	}
	return v1.Partition{}, errors.New("no device found")
}

// Copies source file to target file using afero.Fs interface
func CopyFile(fs afero.Fs, source string, target string) (err error) {
	sourceFile, err := fs.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = sourceFile.Close()
		}
	}()

	targetFile, err := fs.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = targetFile.Close()
		}
	}()

	_, err = io.Copy(targetFile, sourceFile)
	return err
}

// Copies source file to target file using afero.Fs interface
func CreateDirStructure(fs afero.Fs, target string) error {
	for _, dir := range []string{"sys", "proc", "dev", "tmp", "boot", "usr/local", "oem"} {
		err := fs.MkdirAll(fmt.Sprintf("%s/%s", target, dir), 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncData rsync's source folder contents to a target folder content,
// both are expected to exist before hand.
func SyncData(source string, target string, excludes ...string) error {
	if strings.HasSuffix(source, "/") == false {
		source = fmt.Sprintf("%s/", source)
	}

	if strings.HasSuffix(target, "/") == false {
		target = fmt.Sprintf("%s/", target)
	}

	task := grsync.NewTask(
		source,
		target,
		grsync.RsyncOptions{
			Quiet:   false,
			Archive: true,
			XAttrs:  true,
			ACLs:    true,
			Exclude: excludes,
		},
	)

	return task.Run()
}

// Reboot reboots the system afater the given delay (in seconds) time passed.
func Reboot(runner v1.Runner, delay time.Duration) error {
	time.Sleep(delay * time.Second)
	_, err := runner.Run("reboot", "-f")
	return err
}

// Shutdown halts the system afater the given delay (in seconds) time passed.
func Shutdown(runner v1.Runner, delay time.Duration) error {
	time.Sleep(delay * time.Second)
	_, err := runner.Run("poweroff", "-f")
	return err
}

// CosignVerify runs a cosign validation for the give image and given public key. If no
// key is provided then it attempts a keyless validation (experimental feature).
func CosignVerify(fs afero.Fs, runner v1.Runner, image string, publicKey string, debug bool) (string, error) {
	args := []string{}

	if debug {
		args = append(args, "-d=true")
	}
	if publicKey != "" {
		args = append(args, "-key", publicKey)
	} else {
		os.Setenv("COSIGN_EXPERIMENTAL", "1")
		defer os.Unsetenv("COSIGN_EXPERIMENTAL")
	}
	args = append(args, image)

	// Give each cosign its own tuf dir so it doesnt collide with others accessing the same files at the same time
	tmpDir, err := afero.TempDir(fs, "", "cosign-tuf-")
	if err != nil {
		return "", err
	}
	_ = os.Setenv("TUF_ROOT", tmpDir)
	defer fs.RemoveAll(tmpDir)
	defer os.Unsetenv("TUF_ROOT")

	out, err := runner.Run("cosign", args...)
	return string(out), err
}

// CreateSquashFS creates a squash file at destination from a source, with options
// TODO: Check validity of source maybe?
func CreateSquashFS(runner v1.Runner, logger v1.Logger, source string, destination string, options []string) error {
	// create args
	args := []string{source, destination}
	// append options passed to args in order to have the correct order
	args = append(args, options...)
	logger.Debug("Running command: mksquashfs with args: %s", args)
	out, err := runner.Run("mksquashfs", args...)
	if err != nil {
		logger.Debugf("Error running squashfs creation, stdout: %s", out)
		logger.Errorf("Error while creating squashfs from %s to %s: %s", source, destination, err)
		return err
	}
	return nil
}
