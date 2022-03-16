/*
Copyright © 2022 SUSE LLC

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
	"io"
	uri "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/twpayne/go-vfs"
	"github.com/zloylos/grsync"
)

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
	part, err := GetFullDeviceByLabel(runner, label, attempts)
	if err != nil {
		return "", err
	}
	return part.Path, nil
}

// GetFullDeviceByLabel works like GetDeviceByLabel, but it will try to get as much info as possible from the existing
// partition and return a v1.Partition object
func GetFullDeviceByLabel(runner v1.Runner, label string, attempts int) (*v1.Partition, error) {
	for tries := 0; tries < attempts; tries++ {
		_, _ = runner.Run("udevadm", "settle")
		parts, err := GetAllPartitions(runner)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			if part.Label == label {
				return part, nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return nil, errors.New("no device found")
}

// GetPartitionFS returns the partition filesystem for the given device.
// If the device is not a partition returns an error.
func GetPartitionFS(runner v1.Runner, device string) (string, error) {
	dev, err := GetDevicePartitions(runner, device)
	if err != nil {
		return "", err
	}
	if len(dev) != 1 {
		return "", fmt.Errorf("%s does not hold a partition", device)
	}
	return dev[0].FS, nil
}

// CopyFile Copies source file to target file using Fs interface
func CopyFile(fs v1.FS, source string, target string) (err error) {
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

// Copies source file to target file using Fs interface
func CreateDirStructure(fs v1.FS, target string) error {
	for _, dir := range []string{"/run", "/sys", "/proc", "/dev", "/tmp", "/boot", "/usr/local", "/oem"} {
		err := MkdirAll(fs, filepath.Join(target, dir), cnst.DirPerm)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncData rsync's source folder contents to a target folder content,
// both are expected to exist before hand.
func SyncData(fs v1.FS, source string, target string, excludes ...string) error {
	if !strings.HasSuffix(source, "/") {
		source = fmt.Sprintf("%s/", source)
	}

	if !strings.HasSuffix(target, "/") {
		target = fmt.Sprintf("%s/", target)
	}

	if fs != nil {
		if s, err := fs.RawPath(source); err == nil {
			source = s
		}
		if t, err := fs.RawPath(target); err == nil {
			target = t
		}
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

	err := task.Run()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.Join([]string{task.Log().Stderr, task.Log().Stdout}, "\n"))
	}

	return nil
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
func CosignVerify(fs v1.FS, runner v1.Runner, image string, publicKey string, debug bool) (string, error) {
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
	tmpDir, err := TempDir(fs, "", "cosign-tuf-")
	if err != nil {
		return "", err
	}
	_ = os.Setenv("TUF_ROOT", tmpDir)
	defer func(fs v1.FS, path string) {
		_ = fs.RemoveAll(path)
	}(fs, tmpDir)
	defer func() {
		_ = os.Unsetenv("TUF_ROOT")
	}()

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

// LoadEnvFile will try to parse the file given and return a map with the kye/values
func LoadEnvFile(fs v1.FS, file string) (map[string]string, error) {
	var envMap map[string]string
	var err error

	f, err := fs.Open(file)
	if err != nil {
		return envMap, err
	}
	defer f.Close()

	envMap, err = godotenv.Parse(f)
	if err != nil {
		return envMap, err
	}

	return envMap, err
}

// GetUpgradeTempDir returns the dir for storing upgrade related temporal files
// It will respect TMPDIR and use that if exists, fallback to try the persistent partition if its mounted
// and finally the default /tmp/ dir
func GetUpgradeTempDir(config *v1.RunConfig) string {
	// if we got a TMPDIR var, respect and use that
	dir := os.Getenv("TMPDIR")
	if dir != "" {
		return filepath.Join(dir, "elemental-upgrade")
	}
	// Check persistent and if its mounted
	persistent, err := GetFullDeviceByLabel(config.Runner, config.PersistentLabel, 5)
	if err == nil && persistent.MountPoint != "" {
		return filepath.Join(persistent.MountPoint, "elemental-upgrade")
	}
	return filepath.Join("/", "tmp", "elemental-upgrade")
}

// IsLocalURL returns true if the url has no scheme or it has "file" scheme
// and returns false otherwise. Error is not nil if the url can't be parsed.
func IsLocalURL(url string) (bool, error) {
	u, err := uri.Parse(url)
	if err != nil {
		return false, err
	}
	if u.Scheme == "file" || u.Scheme == "" {
		return true, nil
	}
	return false, nil
}

// GetSource copies given source to destination, if source is a local path it simply
// copies files, if source is a remote URL it tries to download URL to destination.
func GetSource(config *v1.RunConfig, source string, destination string) error {
	local, err := IsLocalURL(source)
	if err != nil {
		config.Logger.Errorf("Not a valid url: %s", source)
		return err
	}

	err = vfs.MkdirAll(config.Fs, filepath.Dir(destination), cnst.DirPerm)
	if err != nil {
		return err
	}
	if local {
		u, _ := uri.Parse(source)
		err = CopyFile(config.Fs, u.Path, destination)
		if err != nil {
			return err
		}
	} else {
		err = config.Client.GetURL(config.Logger, source, destination)
		if err != nil {
			return err
		}
	}
	return nil
}
