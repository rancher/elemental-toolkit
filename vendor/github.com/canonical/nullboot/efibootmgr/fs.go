// This file is part of nullboot
// Copyright 2021 Canonical Ltd.
// SPDX-License-Identifier: GPL-3.0-only

package efibootmgr

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// File abstracts an open file.
type File interface {
	io.Closer
	io.Writer
	io.Reader
	io.ReaderAt
	io.Seeker

	Name() string
	Stat() (os.FileInfo, error)
}

// FS abstracts away the filesystem.
//
// So we really wanted to use afero because it does all the magic for us, but it doubles
// our binary size, so that seems a tad much.
type FS interface {
	// Create behaves like os.Create()
	Create(path string) (File, error)
	// MkdirAll behaves like os.MkdirAll()
	MkdirAll(path string, perm os.FileMode) error
	// Open behaves like os.Open()
	Open(path string) (File, error)
	// ReadDir behaves like os.ReadDir()
	ReadDir(path string) ([]os.DirEntry, error)
	// Readlink behaves like os.Readlink()
	Readlink(path string) (string, error)
	// Remove behaves like os.Remove()
	Remove(path string) error
	// Rename behaves like os.Rename()
	Rename(oldname, newname string) error
	// Stat behaves like os.Stat()
	Stat(path string) (os.FileInfo, error)
	// TempFile behaves like ioutil.TempFile()
	TempFile(dir, prefix string) (File, error)
}

// realFS implements FS using the os package
type realFS struct{}

func (realFS) Create(path string) (File, error)             { return os.Create(path) }
func (realFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (realFS) Open(path string) (File, error)               { return os.Open(path) }
func (realFS) ReadDir(path string) ([]os.DirEntry, error)   { return os.ReadDir(path) }
func (realFS) Readlink(path string) (string, error)         { return os.Readlink(path) }
func (realFS) Remove(path string) error                     { return os.Remove(path) }
func (realFS) Rename(oldname, newname string) error         { return os.Rename(oldname, newname) }
func (realFS) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (realFS) TempFile(dir, prefix string) (File, error)    { return ioutil.TempFile(dir, prefix) }

// appFs is our default FS
var appFs FS = realFS{}

// MaybeUpdateFile copies src to dest if they are different
// It returns true if the destination file was successfully updated. If the return value
// is false, the state of the destination is unspecified. It might not exist, exist
// with partial data or exist with old data, amongst others.
func MaybeUpdateFile(dst string, src string) (updated bool, err error) {
	srcFile, err := appFs.Open(src)
	if err != nil {
		return false, fmt.Errorf("Could not open source file: %w", err)
	}
	defer srcFile.Close()

	if needUpdate, err := needUpdateFile(dst, src, srcFile); !needUpdate {
		return false, err
	}

	dstFile, err := appFs.TempFile(filepath.Dir(dst), "."+filepath.Base(dst)+".")
	if err != nil {
		return false, fmt.Errorf("Could not open %s for writing: %w", dst, err)
	}
	defer func() {
		name := dstFile.Name()
		dstFile.Close()
		if err != nil {
			appFs.Remove(name)
		}
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return false, fmt.Errorf("Could not copy %s to %s: %w", src, dst, err)
	}

	if err := appFs.Rename(dstFile.Name(), dst); err != nil {
		return false, fmt.Errorf("cannot rename %s to %s: %w", dstFile.Name(), dst, err)
	}

	return true, nil
}

func needUpdateFile(dst string, src string, srcFile File) (bool, error) {
	// To keep things simple, but not have the files in memory, just hash them
	dstHash := sha256.New()
	srcHash := sha256.New()

	dstFile, err := appFs.Open(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("Could not open destination file: %w", err)
	}

	defer dstFile.Close()

	if _, err := io.Copy(dstHash, dstFile); err != nil {
		return false, fmt.Errorf("Could not hash destination file %s: %w", dst, err)
	}
	if _, err := io.Copy(srcHash, srcFile); err != nil {
		return false, fmt.Errorf("Could not hash source file %s: %w", src, err)
	}
	if bytes.Equal(dstHash.Sum(nil), srcHash.Sum(nil)) {
		return false, nil
	}

	if _, err := srcFile.Seek(0, io.SeekStart); err != nil {
		return false, fmt.Errorf("Could not seek in source file %s: %w", src, err)
	}

	return true, nil
}
