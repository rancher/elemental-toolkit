/*
Copyright © 2022 - 2023 SUSE LLC

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
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/distribution/distribution/reference"
	"github.com/joho/godotenv"
	"github.com/twpayne/go-vfs"
	"github.com/zloylos/grsync"

	cnst "github.com/rancher/elemental-cli/pkg/constants"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
)

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
		parts, err := GetAllPartitions()
		if err != nil {
			return nil, err
		}
		part := parts.GetByLabel(label)
		if part != nil {
			return part, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, errors.New("no device found")
}

// CopyFile Copies source file to target file using Fs interface. If target
// is  directory source is copied into that directory using source name file.
// File mode is preserved
func CopyFile(fs v1.FS, source string, target string) error {
	return ConcatFiles(fs, []string{source}, target)
}

// ConcatFiles Copies source files to target file using Fs interface.
// Source files are concatenated into target file in the given order.
// If target is a directory source is copied into that directory using
// 1st source name file. The result keeps the file mode of the 1st source.
func ConcatFiles(fs v1.FS, sources []string, target string) (err error) {
	if len(sources) == 0 {
		return fmt.Errorf("Empty sources list")
	}
	if dir, _ := IsDir(fs, target); dir {
		target = filepath.Join(target, filepath.Base(sources[0]))
	}
	fInf, err := fs.Stat(sources[0])
	if err != nil {
		return err
	}

	targetFile, err := fs.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = targetFile.Close()
		} else {
			_ = fs.Remove(target)
		}
	}()

	var sourceFile *os.File
	for _, source := range sources {
		sourceFile, err = fs.Open(source)
		if err != nil {
			return err
		}
		_, err = io.Copy(targetFile, sourceFile)
		if err != nil {
			return err
		}
		err = sourceFile.Close()
		if err != nil {
			return err
		}
	}

	return fs.Chmod(target, fInf.Mode())
}

// CreateDirStructure creates essentials directories under the root tree that might not be present
// within a container image (/dev, /run, etc.)
func CreateDirStructure(fs v1.FS, target string) error {
	for _, dir := range []string{"/run", "/dev", "/boot", "/usr/local", "/oem"} {
		err := MkdirAll(fs, filepath.Join(target, dir), cnst.DirPerm)
		if err != nil {
			return err
		}
	}
	for _, dir := range []string{"/proc", "/sys"} {
		err := MkdirAll(fs, filepath.Join(target, dir), cnst.NoWriteDirPerm)
		if err != nil {
			return err
		}
	}
	err := MkdirAll(fs, filepath.Join(target, "/tmp"), cnst.DirPerm)
	if err != nil {
		return err
	}
	// Set /tmp permissions regardless the umask setup
	err = fs.Chmod(filepath.Join(target, "/tmp"), cnst.TempDirPerm)
	if err != nil {
		return err
	}
	return nil
}

// SyncData rsync's source folder contents to a target folder content,
// both are expected to exist before hand.
func SyncData(log v1.Logger, fs v1.FS, source string, target string, excludes ...string) error {
	if fs != nil {
		if s, err := fs.RawPath(source); err == nil {
			source = s
		}
		if t, err := fs.RawPath(target); err == nil {
			target = t
		}
	}

	if !strings.HasSuffix(source, "/") {
		source = fmt.Sprintf("%s/", source)
	}

	if !strings.HasSuffix(target, "/") {
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

	quit := make(chan bool)
	go func() {
		for {
			select {
			case <-quit:
				return
			case <-time.After(5 * time.Second):
				state := task.State()
				log.Debugf(
					"progress rsync %s to %s: %.2f / rem. %d / tot. %d / sp. %s",
					source,
					target,
					state.Progress,
					state.Remain,
					state.Total,
					state.Speed,
				)
			}
		}
	}()

	err := task.Run()
	quit <- true
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
	// protect against options passed together in the same string , i.e. "-x add" instead of "-x", "add"
	var optionsExpanded []string
	for _, op := range options {
		optionsExpanded = append(optionsExpanded, strings.Split(op, " ")...)
	}
	args = append(args, optionsExpanded...)
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

func IsMounted(config *v1.Config, part *v1.Partition) (bool, error) {
	if part == nil {
		return false, fmt.Errorf("nil partition")
	}

	if part.MountPoint == "" {
		return false, nil
	}
	// Using IsLikelyNotMountPoint seams to be safe as we are not checking
	// for bind mounts here
	notMnt, err := config.Mounter.IsLikelyNotMountPoint(part.MountPoint)
	if err != nil {
		return false, err
	}
	return !notMnt, nil
}

// GetTempDir returns the dir for storing related temporal files
// It will respect TMPDIR and use that if exists, fallback to try the persistent partition if its mounted
// and finally the default /tmp/ dir
// suffix is what is appended to the dir name elemental-suffix. If empty it will randomly generate a number
func GetTempDir(config *v1.Config, suffix string) string {
	// if we got a TMPDIR var, respect and use that
	if suffix == "" {
		random := rand.New(rand.NewSource(time.Now().UnixNano()))
		suffix = strconv.Itoa(int(random.Uint32()))
	}
	elementalTmpDir := fmt.Sprintf("elemental-%s", suffix)
	dir := os.Getenv("TMPDIR")
	if dir != "" {
		config.Logger.Debugf("Got tmpdir from TMPDIR var: %s", dir)
		return filepath.Join(dir, elementalTmpDir)
	}
	parts, err := GetAllPartitions()
	if err != nil {
		config.Logger.Debug("Could not get partitions, defaulting to /tmp")
		return filepath.Join("/", "tmp", elementalTmpDir)
	}
	// Check persistent and if its mounted
	state, _ := config.LoadInstallState()
	ep := v1.NewElementalPartitionsFromList(parts, state)
	persistent := ep.Persistent
	if persistent != nil {
		if mnt, _ := IsMounted(config, persistent); mnt {
			config.Logger.Debugf("Using tmpdir on persistent volume: %s", persistent.MountPoint)
			return filepath.Join(persistent.MountPoint, "tmp", elementalTmpDir)
		}
	}
	config.Logger.Debug("Could not get any valid tmpdir, defaulting to /tmp")
	return filepath.Join("/", "tmp", elementalTmpDir)
}

// IsLocalURI returns true if the uri has "file" scheme or no scheme and URI is
// not prefixed with a domain (container registry style). Returns false otherwise.
// Error is not nil only if the url can't be parsed.
func IsLocalURI(uri string) (bool, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return false, err
	}
	if u.Scheme == "file" {
		return true, nil
	}
	if u.Scheme == "" {
		// Check first part of the path is not a domain (e.g. registry.suse.com/elemental)
		// reference.ParsedNamed expects a <domain>[:<port>]/<path>[:<tag>] form.
		if _, err = reference.ParseNamed(uri); err != nil {
			return true, nil
		}
	}
	return false, nil
}

// IsHTTPURI returns true if the uri has "http" or "https" scheme, returns false otherwise.
// Error is not nil only if the url can't be parsed.
func IsHTTPURI(uri string) (bool, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return false, err
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return true, nil
	}
	return false, nil
}

// GetSource copies given source to destination, if source is a local path it simply
// copies files, if source is a remote URL it tries to download URL to destination.
func GetSource(config *v1.Config, source string, destination string) error {
	local, err := IsLocalURI(source)
	if err != nil {
		config.Logger.Errorf("Not a valid url: %s", source)
		return err
	}

	err = vfs.MkdirAll(config.Fs, filepath.Dir(destination), cnst.DirPerm)
	if err != nil {
		return err
	}
	if local {
		u, _ := url.Parse(source)
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

// ValidContainerReferece returns true if the given string matches
// a container registry reference, false otherwise
func ValidContainerReference(ref string) bool {
	if _, err := reference.ParseNormalizedNamed(ref); err != nil {
		return false
	}
	return true
}

// ValidTaggedContainerReferece returns true if the given string matches
// a container registry reference including a tag, false otherwise.
func ValidTaggedContainerReference(ref string) bool {
	n, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return false
	}
	if reference.IsNameOnly(n) {
		return false
	}
	return true
}

// FindFileWithPrefix looks for a file in the given path matching one of the given
// prefixes. Returns the found file path including the given path. It does not
// check subfolders recusively
func FindFileWithPrefix(fs v1.FS, path string, prefixes ...string) (string, error) {
	files, err := fs.ReadDir(path)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		for _, p := range prefixes {
			if strings.HasPrefix(f.Name(), p) {
				if f.Mode()&os.ModeSymlink == os.ModeSymlink {
					found, err := fs.Readlink(filepath.Join(path, f.Name()))
					if err == nil {
						if !filepath.IsAbs(found) {
							found = filepath.Join(path, found)
						}
						if exists, _ := Exists(fs, found); exists {
							return found, nil
						}
					}
				} else {
					return filepath.Join(path, f.Name()), nil
				}
			}
		}
	}
	return "", fmt.Errorf("No file found with prefixes: %v", prefixes)
}

// CalcFileChecksum opens the given file and returns the sha256 checksum of it.
func CalcFileChecksum(fs v1.FS, fileName string) (string, error) {
	f, err := fs.Open(fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// IdentifySourceSystem tries to find the os-release file in a given dir and identify the system based on the data in there
func IdentifySourceSystem(vfs v1.FS, path string) (string, error) {
	var system string
	var found bool
	err := WalkDirFs(vfs, path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "os-release" {
			osRelease, err := parseOsRelease(vfs, path)
			if err != nil {
				return err
			}
			switch osRelease["ID"] {
			case cnst.Fedora:
				system = cnst.Fedora
			case cnst.Ubuntu:
				system = cnst.Ubuntu
			default:
				system = cnst.Suse
			}
			found = true
		}
		return err
	})
	if !found {
		err = fmt.Errorf("could not find os-release file under %s", path)
	}
	return system, err
}

func parseOsRelease(fs v1.FS, filename string) (osrelease map[string]string, err error) {
	var lines []string
	osrelease = map[string]string{}
	file, err := fs.Open(filename)
	if err != nil {
		return
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	for _, line := range lines {
		//key, value, err := parseOsReleaseLine(v)
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' {
			continue
		}
		splitted := strings.SplitN(line, "=", 2)
		if len(splitted) != 2 {
			continue
		}

		key := strings.Trim(strings.TrimSpace(splitted[0]), "\"")
		value := strings.Trim(strings.TrimSpace(splitted[1]), "\"")
		osrelease[key] = value
	}
	return
}
