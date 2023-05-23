package vfs

import (
	"os"
	"syscall"
	"time"
)

// A ReadOnlyFS operates on an existing FS, but any methods that
// modify the FS return an error.
type ReadOnlyFS struct {
	fs FS
}

// NewReadOnlyFS returns a new *ReadOnlyFS operating on fs.
func NewReadOnlyFS(fs FS) *ReadOnlyFS {
	return &ReadOnlyFS{
		fs: fs,
	}
}

// Chmod implements os.Chmod.
func (r *ReadOnlyFS) Chmod(name string, mode os.FileMode) error {
	return permError("Chmod", name)
}

// Chown implements os.Chown.
func (r *ReadOnlyFS) Chown(name string, uid, gid int) error {
	return permError("Chown", name)
}

// Chtimes implements os.Chtimes.
func (r *ReadOnlyFS) Chtimes(name string, atime, mtime time.Time) error {
	return permError("Chtimes", name)
}

// Create implements os.Create.
func (r *ReadOnlyFS) Create(name string) (*os.File, error) {
	return nil, permError("Create", name)
}

// Glob implements filepath.Glob.
func (r *ReadOnlyFS) Glob(pattern string) ([]string, error) {
	return r.fs.Glob(pattern)
}

// Lchown implements os.Lchown.
func (r *ReadOnlyFS) Lchown(name string, uid, gid int) error {
	return permError("Lchown", name)
}

// Lstat implements os.Lstat.
func (r *ReadOnlyFS) Lstat(name string) (os.FileInfo, error) {
	return r.fs.Lstat(name)
}

// Mkdir implements os.Mkdir.
func (r *ReadOnlyFS) Mkdir(name string, perm os.FileMode) error {
	return permError("Mkdir", name)
}

// Open implements os.Open.
func (r *ReadOnlyFS) Open(name string) (*os.File, error) {
	return r.fs.Open(name)
}

// OpenFile implements os.OpenFile.
func (r *ReadOnlyFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if flag&(os.O_RDONLY|os.O_WRONLY|os.O_RDWR) != os.O_RDONLY {
		return nil, permError("OpenFile", name)
	}
	return r.fs.OpenFile(name, flag, perm)
}

// PathSeparator implements PathSeparator.
func (r *ReadOnlyFS) PathSeparator() rune {
	return r.fs.PathSeparator()
}

// ReadDir implements ioutil.ReadDir.
func (r *ReadOnlyFS) ReadDir(dirname string) ([]os.FileInfo, error) {
	return r.fs.ReadDir(dirname)
}

// ReadFile implements ioutil.ReadFile.
func (r *ReadOnlyFS) ReadFile(filename string) ([]byte, error) {
	return r.fs.ReadFile(filename)
}

// Readlink implments os.Readlink.
func (r *ReadOnlyFS) Readlink(name string) (string, error) {
	return r.fs.Readlink(name)
}

// Remove implements os.Remove.
func (r *ReadOnlyFS) Remove(name string) error {
	return permError("Remove", name)
}

// RemoveAll implements os.RemoveAll.
func (r *ReadOnlyFS) RemoveAll(name string) error {
	return permError("RemoveAll", name)
}

// Rename implements os.Rename.
func (r *ReadOnlyFS) Rename(oldpath, newpath string) error {
	return permError("Rename", oldpath)
}

// RawPath implements RawPath.
func (r *ReadOnlyFS) RawPath(path string) (string, error) {
	return r.fs.RawPath(path)
}

// Stat implements os.Stat.
func (r *ReadOnlyFS) Stat(name string) (os.FileInfo, error) {
	return r.fs.Stat(name)
}

// Symlink implements os.Symlink.
func (r *ReadOnlyFS) Symlink(oldname, newname string) error {
	return permError("Symlink", newname)
}

// Truncate implements os.Truncate.
func (r *ReadOnlyFS) Truncate(name string, size int64) error {
	return permError("Truncate", name)
}

// WriteFile implements ioutil.WriteFile.
func (r *ReadOnlyFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return permError("WriteFile", filename)
}

// permError returns an *os.PathError with Err syscall.EPERM.
func permError(op, path string) error {
	return &os.PathError{
		Op:   op,
		Path: path,
		Err:  syscall.EPERM,
	}
}
