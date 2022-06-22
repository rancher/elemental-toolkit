package vfs

import (
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

// A PathFS operates on an existing FS, but prefixes all names with a path. All
// names must be absolute paths, with the exception of symlinks, which may be
// relative.
type PathFS struct {
	fs   FS
	path string
}

// NewPathFS returns a new *PathFS operating on fs and prefixing all names with
// path.
func NewPathFS(fs FS, path string) *PathFS {
	return &PathFS{
		path: filepath.ToSlash(path),
		fs:   fs,
	}
}

// Chmod implements os.Chmod.
func (p *PathFS) Chmod(name string, mode os.FileMode) error {
	realName, err := p.join("Chmod", name)
	if err != nil {
		return err
	}
	return p.fs.Chmod(realName, mode)
}

// Chown implements os.Chown.
func (p *PathFS) Chown(name string, uid, gid int) error {
	realName, err := p.join("Chown", name)
	if err != nil {
		return err
	}
	return p.fs.Chown(realName, uid, gid)
}

// Chtimes implements os.Chtimes.
func (p *PathFS) Chtimes(name string, atime, mtime time.Time) error {
	realName, err := p.join("Chtimes", name)
	if err != nil {
		return err
	}
	return p.fs.Chtimes(realName, atime, mtime)
}

// Create implements os.Create.
func (p *PathFS) Create(name string) (*os.File, error) {
	realName, err := p.join("Create", name)
	if err != nil {
		return nil, err
	}
	return p.fs.Create(realName)
}

// Glob implements filepath.Glob.
func (p *PathFS) Glob(pattern string) ([]string, error) {
	realPattern, err := p.join("Glob", pattern)
	if err != nil {
		return nil, err
	}
	matches, err := p.fs.Glob(realPattern)
	if err != nil {
		return nil, err
	}
	for i, match := range matches {
		matches[i], err = trimPrefix(match, p.path)
		if err != nil {
			return nil, err
		}
	}
	return matches, nil
}

// Join returns p's path joined with name.
func (p *PathFS) Join(op, name string) (string, error) {
	return p.join("Join", name)
}

// Lchown implements os.Lchown.
func (p *PathFS) Lchown(name string, uid, gid int) error {
	realName, err := p.join("Lchown", name)
	if err != nil {
		return err
	}
	return p.fs.Lchown(realName, uid, gid)
}

// Lstat implements os.Lstat.
func (p *PathFS) Lstat(name string) (os.FileInfo, error) {
	realName, err := p.join("Lstat", name)
	if err != nil {
		return nil, err
	}
	return p.fs.Lstat(realName)
}

// Mkdir implements os.Mkdir.
func (p *PathFS) Mkdir(name string, perm os.FileMode) error {
	realName, err := p.join("Mkdir", name)
	if err != nil {
		return err
	}
	return p.fs.Mkdir(realName, perm)
}

// Open implements os.Open.
func (p *PathFS) Open(name string) (*os.File, error) {
	realName, err := p.join("Open", name)
	if err != nil {
		return nil, err
	}
	return p.fs.Open(realName)
}

// OpenFile implements os.OpenFile.
func (p *PathFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	realName, err := p.join("OpenFile", name)
	if err != nil {
		return nil, err
	}
	return p.fs.OpenFile(realName, flag, perm)
}

// PathSeparator implements PathSeparator.
func (p *PathFS) PathSeparator() rune {
	return p.fs.PathSeparator()
}

// RawPath implements RawPath.
func (p *PathFS) RawPath(path string) (string, error) {
	return p.join("RawPath", path)
}

// ReadDir implements ioutil.ReadDir.
func (p *PathFS) ReadDir(dirname string) ([]os.FileInfo, error) {
	realDirname, err := p.join("ReadDir", dirname)
	if err != nil {
		return nil, err
	}
	return p.fs.ReadDir(realDirname)
}

// ReadFile implements ioutil.ReadFile.
func (p *PathFS) ReadFile(filename string) ([]byte, error) {
	realFilename, err := p.join("ReadFile", filename)
	if err != nil {
		return nil, err
	}
	return p.fs.ReadFile(realFilename)
}

// Readlink implements os.Readlink.
func (p *PathFS) Readlink(name string) (string, error) {
	realName, err := p.join("Readlink", name)
	if err != nil {
		return "", err
	}
	return p.fs.Readlink(realName)
}

// Remove implements os.Remove.
func (p *PathFS) Remove(name string) error {
	realName, err := p.join("Remove", name)
	if err != nil {
		return err
	}
	return p.fs.Remove(realName)
}

// RemoveAll implements os.RemoveAll.
func (p *PathFS) RemoveAll(name string) error {
	realName, err := p.join("RemoveAll", name)
	if err != nil {
		return err
	}
	return p.fs.RemoveAll(realName)
}

// Rename implements os.Rename.
func (p *PathFS) Rename(oldpath, newpath string) error {
	realOldpath, err := p.join("Rename", oldpath)
	if err != nil {
		return err
	}
	realNewpath, err := p.join("Rename", newpath)
	if err != nil {
		return err
	}
	return p.fs.Rename(realOldpath, realNewpath)
}

// Stat implements os.Stat.
func (p *PathFS) Stat(name string) (os.FileInfo, error) {
	realName, err := p.join("Stat", name)
	if err != nil {
		return nil, err
	}
	return p.fs.Stat(realName)
}

// Symlink implements os.Symlink.
func (p *PathFS) Symlink(oldname, newname string) error {
	var realOldname string
	if path.IsAbs(oldname) {
		var err error
		realOldname, err = p.join("Symlink", oldname)
		if err != nil {
			return err
		}
	} else {
		realOldname = oldname
	}
	realNewname, err := p.join("Symlink", newname)
	if err != nil {
		return err
	}
	return p.fs.Symlink(realOldname, realNewname)
}

// Truncate implements os.Truncate.
func (p *PathFS) Truncate(name string, size int64) error {
	realName, err := p.join("Truncate", name)
	if err != nil {
		return err
	}
	return p.fs.Truncate(realName, size)
}

// WriteFile implements ioutil.WriteFile.
func (p *PathFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	realFilename, err := p.join("WriteFile", filename)
	if err != nil {
		return err
	}
	return p.fs.WriteFile(realFilename, data, perm)
}

// join returns p's path joined with name.
func (p *PathFS) join(op, name string) (string, error) {
	name = relativizePath(name)
	if !path.IsAbs(name) {
		return "", &os.PathError{
			Op:   op,
			Path: name,
			Err:  syscall.EPERM,
		}
	}
	return filepath.Join(p.path, name), nil
}
