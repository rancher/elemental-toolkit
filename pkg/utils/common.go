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
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
	"io"
	"os/exec"
	"strings"
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

// FindLabel will try to get the partition that has the label given in the current disk
func FindLabel(runner v1.Runner, label string) (string, error) {
	out, err := runner.Run("blkid", "-L", label)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetDeviceByLabel will try to return the device that matches the given label
func GetDeviceByLabel(runner v1.Runner, label string) (string, error) {
	out, err := runner.Run("blkid", "-t", fmt.Sprintf("LABEL=%s", label), "-o", "device")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(out)) == "" {
		return "", errors.New("no device found")
	}
	return strings.TrimSpace(string(out)), nil
}

// Copies source file to target file using afero.Fs interface
func CopyFile(fs afero.Fs, source string, target string) error {
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
