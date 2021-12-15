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
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"io"
	"os"
	"os/exec"
	"strings"
)

// GetUrl is a simple method that will try to get an url to a destination, no matter if its an http url, ftp, tftp or a file
func GetUrl(client v1.HTTPClient, logger v1.Logger, url string, destination string) error {
	var source io.Reader
	var err error

	switch {
	case strings.HasPrefix(url, "http"), strings.HasPrefix(url, "ftp"), strings.HasPrefix(url, "tftp"):
		logger.Infof("Downloading from %s to %s", url, destination)
		resp, err := client.Get(url)
		if err != nil {
			return err
		}
		source = resp.Body
		defer resp.Body.Close()
	default:
		logger.Infof("Copying from %s to %s", url, destination)
		file, err := os.Open(url)
		if err != nil {
			return err
		}
		source = file
		defer file.Close()
	}

	dest, err := os.Create(destination)
	defer dest.Close()
	if err != nil {
		return err
	}
	nBytes, err := io.Copy(dest, source)
	if err != nil {
		return err
	}
	logger.Infof("Copied %d bytes", nBytes)

	return nil
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

// FindLabel will try to get the partition that has the label given in the current disk
func FindLabel(runner v1.Runner, label string) (string, error) {
	out, err := runner.Run("blkid", "-L", label)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
