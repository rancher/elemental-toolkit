// nolint:goheader

/*
Copyright © 2022 spf13/afero
Copyright © 2022 - 2026 SUSE LLC

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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

// DirSize returns the accumulated size of all files in folder. Result in bytes
func DirSize(fs types.FS, path string, excludes ...string) (int64, error) {
	var size int64
	err := vfs.Walk(fs, path, func(loopPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, exclude := range excludes {
				if strings.HasPrefix(loopPath, exclude) {
					return filepath.SkipDir
				}
			}
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// DirSizeMB returns the accumulated size of all files in folder. Result in Megabytes
func DirSizeMB(fs types.FS, path string, excludes ...string) (uint, error) {
	size, err := DirSize(fs, path, excludes...)
	if err != nil {
		return 0, err
	}

	MB := int64(1024 * 1024)
	sizeMB := (size/MB*MB + MB) / MB
	if sizeMB > 0 {
		return uint(sizeMB), nil
	}
	return 0, fmt.Errorf("Negative size calculation: %d", sizeMB)
}

// Check if a file or directory exists. noFollow flag determines to
// not follow symlinks to check files existance.
func Exists(fs types.FS, path string, noFollow ...bool) (bool, error) {
	var err error
	if len(noFollow) > 0 && noFollow[0] {
		_, err = fs.Lstat(path)
	} else {
		_, err = fs.Stat(path)
	}
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// RemoveAll removes the specified path.
// It silently drop NotExists errors.
func RemoveAll(fs types.FS, path string) error {
	err := fs.RemoveAll(path)
	if !os.IsNotExist(err) {
		return err
	}

	return nil
}

// IsDir check if the path is a dir
func IsDir(fs types.FS, path string) (bool, error) {
	fi, err := fs.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

// MkdirAll directory and all parents if not existing
func MkdirAll(fs types.FS, name string, mode os.FileMode) (err error) {
	if _, isReadOnly := fs.(*vfs.ReadOnlyFS); isReadOnly {
		return permError("mkdir", name)
	}
	if name, err = fs.RawPath(name); err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return os.MkdirAll(name, mode)
}

// readlink calls fs.Readlink but trims temporary prefix on Readlink result
func readlink(fs types.FS, name string) (string, error) {
	res, err := fs.Readlink(name)
	if err != nil {
		return res, err
	}
	raw, err := fs.RawPath(name)
	return strings.TrimPrefix(res, strings.TrimSuffix(raw, name)), err
}

// permError returns an *os.PathError with Err syscall.EPERM.
func permError(op, path string) error {
	return &os.PathError{
		Op:   op,
		Path: path,
		Err:  syscall.EPERM,
	}
}

// Random number state.
// We generate random temporary file names so that there's a good
// chance the file doesn't exist yet - keeps the number of tries in
// TempFile to a minimum.
var (
	randSeed uint32
	randmu   sync.Mutex
)

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextRandom() string {
	randmu.Lock()
	r := randSeed
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	randSeed = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

// TempDir creates a temp file in the virtual fs
// Took from afero.FS code and adapted
func TempDir(fs types.FS, dir, prefix string) (name string, err error) {
	if dir == "" {
		dir = os.TempDir()
	}
	// This skips adding random stuff to the created temp dir so the temp dir created is predictable for testing
	if _, isTestFs := fs.(*vfst.TestFS); isTestFs {
		err = MkdirAll(fs, filepath.Join(dir, prefix), 0700)
		if err != nil {
			return "", err
		}
		name = filepath.Join(dir, prefix)
		return
	}
	nconflict := 0
	for i := 0; i < 10000; i++ {
		try := filepath.Join(dir, prefix+nextRandom())
		err = MkdirAll(fs, try, 0700)
		if os.IsExist(err) {
			if nconflict++; nconflict > 10 {
				randmu.Lock()
				randSeed = reseed()
				randmu.Unlock()
			}
			continue
		}
		if err == nil {
			name = try
		}
		break
	}
	return
}

// TempFile creates a temp file in the virtual fs
// Took from afero.FS code and adapted
func TempFile(fs types.FS, dir, pattern string) (f *os.File, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	var prefix, suffix string
	if pos := strings.LastIndex(pattern, "*"); pos != -1 {
		prefix, suffix = pattern[:pos], pattern[pos+1:]
	} else {
		prefix = pattern
	}

	nconflict := 0
	for i := 0; i < 10000; i++ {
		name := filepath.Join(dir, prefix+nextRandom()+suffix)
		f, err = fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			if nconflict++; nconflict > 10 {
				randmu.Lock()
				randSeed = reseed()
				randmu.Unlock()
			}
			continue
		}
		break
	}
	return
}

// Walkdir with an FS implementation
type statDirEntry struct {
	info fs.FileInfo
}

func (d *statDirEntry) Name() string               { return d.info.Name() }
func (d *statDirEntry) IsDir() bool                { return d.info.IsDir() }
func (d *statDirEntry) Type() fs.FileMode          { return d.info.Mode().Type() }
func (d *statDirEntry) Info() (fs.FileInfo, error) { return d.info, nil }

// WalkDirFs is the same as filepath.WalkDir but accepts a types.Fs so it can be run on any types.Fs type
func WalkDirFs(fs types.FS, root string, fn fs.WalkDirFunc) error {
	info, err := fs.Stat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = walkDir(fs, root, &statDirEntry{info}, fn)
	}
	if err == filepath.SkipDir {
		return nil
	}
	return err
}

func walkDir(fs types.FS, path string, d fs.DirEntry, walkDirFn fs.WalkDirFunc) error {
	if err := walkDirFn(path, d, nil); err != nil || !d.IsDir() {
		if err == filepath.SkipDir && d.IsDir() {
			// Successfully skipped directory.
			err = nil
		}
		return err
	}

	dirs, err := readDir(fs, path)
	if err != nil {
		// Second call, to report ReadDir error.
		err = walkDirFn(path, d, err)
		if err != nil {
			return err
		}
	}

	for _, d1 := range dirs {
		path1 := filepath.Join(path, d1.Name())
		if err := walkDir(fs, path1, d1, walkDirFn); err != nil {
			if err == filepath.SkipDir {
				break
			}
			return err
		}
	}
	return nil
}

func readDir(vfs types.FS, dirname string) ([]fs.DirEntry, error) {
	dirs, err := vfs.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	return dirs, nil
}
