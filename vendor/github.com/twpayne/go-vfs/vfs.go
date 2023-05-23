// Package vfs provides an abstraction of the os and ioutil packages that is
// easy to test.
package vfs

import (
	"os"
	"time"
)

// An FS is an abstraction over commonly-used functions in the os and ioutil
// packages.
type FS interface {
	Chmod(name string, mode os.FileMode) error
	Chown(name string, uid, git int) error
	Chtimes(name string, atime, mtime time.Time) error
	Create(name string) (*os.File, error)
	Glob(pattern string) ([]string, error)
	Lchown(name string, uid, git int) error
	Lstat(name string) (os.FileInfo, error)
	Mkdir(name string, perm os.FileMode) error
	Open(name string) (*os.File, error)
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	PathSeparator() rune
	RawPath(name string) (string, error)
	ReadDir(dirname string) ([]os.FileInfo, error)
	ReadFile(filename string) ([]byte, error)
	Readlink(name string) (string, error)
	Remove(name string) error
	RemoveAll(name string) error
	Rename(oldpath, newpath string) error
	Stat(name string) (os.FileInfo, error)
	Symlink(oldname, newname string) error
	Truncate(name string, size int64) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
}
