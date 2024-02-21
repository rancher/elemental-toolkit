/*
Copyright © 2022 - 2024 SUSE LLC

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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/distribution/distribution/reference"
	"github.com/joho/godotenv"
	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
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
		sourceFile, err = fs.OpenFile(source, os.O_RDONLY, constants.FilePerm)
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
	for _, dir := range []string{"/run", "/dev", "/boot", "/oem", "/system", "/etc/elemental/config.d"} {
		err := MkdirAll(fs, filepath.Join(target, dir), constants.DirPerm)
		if err != nil {
			return err
		}
	}
	for _, dir := range []string{"/proc", "/sys"} {
		err := MkdirAll(fs, filepath.Join(target, dir), constants.NoWriteDirPerm)
		if err != nil {
			return err
		}
	}
	err := MkdirAll(fs, filepath.Join(target, "/tmp"), constants.DirPerm)
	if err != nil {
		return err
	}
	// Set /tmp permissions regardless the umask setup
	err = fs.Chmod(filepath.Join(target, "/tmp"), constants.TempDirPerm)
	if err != nil {
		return err
	}
	return nil
}

// SyncData rsync's source folder contents to a target folder content,
// both are expected to exist before hand.
func SyncData(log v1.Logger, runner v1.Runner, fs v1.FS, source string, target string, excludes ...string) error {
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

	log.Infof("Starting rsync...")

	args := []string{"--progress", "--partial", "--human-readable", "--archive", "--xattrs", "--acls", "--delete"}
	for _, e := range excludes {
		args = append(args, fmt.Sprintf("--exclude=%s", e))
	}

	args = append(args, source, target)

	done := displayProgress(log, 5*time.Second, "Syncing data...")

	_, err := runner.Run(constants.Rsync, args...)

	close(done)

	if err != nil {
		log.Errorf("rsync finished with errors: %s", err.Error())
		return err
	}

	log.Info("Finished syncing")
	return nil
}

func displayProgress(log v1.Logger, tick time.Duration, message string) chan bool {
	ticker := time.NewTicker(tick)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				log.Debug(message)
			}
		}
	}()

	return done
}

// Reboot reboots the system after the given delay (in seconds) time passed.
func Reboot(runner v1.Runner, delay time.Duration) error {
	time.Sleep(delay * time.Second)
	_, err := runner.Run("reboot", "-f")
	return err
}

// Shutdown halts the system after the given delay (in seconds) time passed.
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

// LoadEnvFile will try to parse the file given and return a map with the key/values
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

// WriteEnvFile will write the given environment file with the given key/values
func WriteEnvFile(fs v1.FS, envs map[string]string, filename string) error {
	var bkFile string

	rawPath, err := fs.RawPath(filename)
	if err != nil {
		return err
	}

	if ok, _ := Exists(fs, filename, true); ok {
		bkFile = filename + ".bk"
		err = fs.Rename(filename, bkFile)
		if err != nil {
			return err
		}
	}

	err = godotenv.Write(envs, rawPath)
	if err != nil {
		if bkFile != "" {
			// try to restore renamed file
			_ = fs.Rename(bkFile, filename)
		}
		return err
	}
	if bkFile != "" {
		_ = fs.Remove(bkFile)
	}
	return nil
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
func GetSource(config v1.Config, source string, destination string) error {
	local, err := IsLocalURI(source)
	if err != nil {
		config.Logger.Errorf("Not a valid url: %s", source)
		return err
	}

	err = vfs.MkdirAll(config.Fs, filepath.Dir(destination), constants.DirPerm)
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

// FindFile attempts to find a file from a list of patterns on top of a given root path.
// Returns first match if any and returns error otherwise.
func FindFile(vfs v1.FS, rootDir string, patterns ...string) (string, error) {
	var err error
	var found string

	for _, pattern := range patterns {
		found, err = findFile(vfs, rootDir, pattern)
		if err != nil {
			return "", err
		} else if found != "" {
			break
		}
	}
	if found == "" {
		return "", fmt.Errorf("failed to find binary matching %v", patterns)
	}
	return found, nil
}

// findFile attempts to find a file from a given pattern on top of a root path.
// Returns empty path if no file is found.
func findFile(vfs v1.FS, rootDir, pattern string) (string, error) {
	var foundFile string
	base := filepath.Join(rootDir, getBaseDir(pattern))
	if ok, _ := Exists(vfs, base); ok {
		err := WalkDirFs(vfs, base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			match, err := filepath.Match(filepath.Join(rootDir, pattern), path)
			if err != nil {
				return err
			}
			if match {
				foundFile, err = resolveLink(vfs, path, rootDir, d, constants.MaxLinkDepth)
				if err != nil {
					return err
				}
				return io.EOF
			}
			return nil
		})
		if err != nil && err != io.EOF {
			return "", err
		}
	}
	return foundFile, nil
}

// FindKernel finds for kernel files inside a given root tree path.
// Returns kernel file and version. It assumes kernel files match certain patterns
func FindKernel(fs v1.FS, rootDir string) (string, string, error) {
	var kernel, version string
	var err error

	kernel, err = FindFile(fs, rootDir, constants.GetKernelPatterns()...)
	if err != nil {
		return "", "", fmt.Errorf("No Kernel file found: %v", err)
	}
	files, err := fs.ReadDir(filepath.Join(rootDir, constants.KernelModulesDir))
	if err != nil {
		return "", "", fmt.Errorf("failed reading modules directory: %v", err)
	}
	for _, f := range files {
		if strings.Contains(kernel, f.Name()) {
			version = f.Name()
			break
		}
	}
	if version == "" {
		return "", "", fmt.Errorf("could not determine the version of kernel %s", kernel)
	}
	return kernel, version, nil
}

// FindInitrd finds for initrd files inside a given root tree path.
// It assumes initrd files match certain patterns
func FindInitrd(fs v1.FS, rootDir string) (string, error) {
	initrd, err := FindFile(fs, rootDir, constants.GetInitrdPatterns()...)
	if err != nil {
		return "", fmt.Errorf("No initrd file found: %v", err)
	}
	return initrd, nil
}

// FindKernelInitrd finds for kernel and intird files inside a given root tree path.
// It assumes kernel and initrd files match certain patterns.
// This is a comodity method of a combination of FindKernel and FindInitrd.
func FindKernelInitrd(fs v1.FS, rootDir string) (kernel string, initrd string, err error) {
	kernel, _, err = FindKernel(fs, rootDir)
	if err != nil {
		return "", "", err
	}
	initrd, err = FindInitrd(fs, rootDir)
	if err != nil {
		return "", "", err
	}
	return kernel, initrd, nil
}

// getBaseDir returns the base directory of a shell path pattern
func getBaseDir(path string) string {
	magicChars := `*?[`
	i := strings.IndexAny(path, magicChars)
	if i > 0 {
		return filepath.Dir(path[:i])
	}
	return path
}

// resolveLink attempts to resolve a symlink, if any. Returns the original given path
// if not a symlink. In case of error returns error and the original given path.
func resolveLink(vfs v1.FS, path string, rootDir string, d fs.DirEntry, depth int) (string, error) {
	var err error
	var resolved string
	var f fs.FileInfo

	f, err = d.Info()
	if err != nil {
		return path, err
	}

	if f.Mode()&os.ModeSymlink == os.ModeSymlink {
		if depth <= 0 {
			return path, fmt.Errorf("can't resolve this path '%s', too many nested links", path)
		}
		resolved, err = readlink(vfs, path)
		if err == nil {
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(path), resolved)
			} else {
				resolved = filepath.Join(rootDir, resolved)
			}
			if f, err = vfs.Lstat(resolved); err == nil {
				return resolveLink(vfs, resolved, rootDir, &statDirEntry{f}, depth-1)
			}
			return path, err
		}
		return path, err
	}
	return path, nil
}

// ResolveLink attempts to resolve a symlink, if any. Returns the original given path
// if not a symlink or if it can't be resolved.
func ResolveLink(vfs v1.FS, path string, rootDir string, depth int) (string, error) {
	f, err := vfs.Lstat(path)
	if err != nil {
		return path, err
	}

	return resolveLink(vfs, path, rootDir, &statDirEntry{f}, depth)
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

// CreateRAWFile creates raw file of the given size in MB
func CreateRAWFile(fs v1.FS, filename string, size uint) error {
	f, err := fs.Create(filename)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateFile)
	}
	err = f.Truncate(int64(size * 1024 * 1024))
	if err != nil {
		f.Close()
		_ = fs.RemoveAll(filename)
		return elementalError.NewFromError(err, elementalError.TruncateFile)
	}
	err = f.Close()
	if err != nil {
		_ = fs.RemoveAll(filename)
		return elementalError.NewFromError(err, elementalError.CloseFile)
	}
	return nil
}
