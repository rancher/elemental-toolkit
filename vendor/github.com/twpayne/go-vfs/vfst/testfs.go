package vfst

import (
	"io/ioutil"
	"os"

	vfs "github.com/twpayne/go-vfs"
)

// A TestFS is a virtual filesystem based in a temporary directory.
type TestFS struct {
	vfs.PathFS
	tempDir string
	keep    bool
}

func newTestFS() (*TestFS, func(), error) {
	tempDir, err := ioutil.TempDir("", "go-vfs-vfst")
	if err != nil {
		return nil, nil, err
	}
	t := &TestFS{
		PathFS:  *vfs.NewPathFS(vfs.OSFS, tempDir),
		tempDir: tempDir,
		keep:    false,
	}
	return t, t.cleanup, nil
}

// NewTestFS returns a new *TestFS populated with root and a cleanup function.
func NewTestFS(root interface{}, builderOptions ...BuilderOption) (*TestFS, func(), error) {
	fs, cleanup, err := newTestFS()
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if err := NewBuilder(builderOptions...).Build(fs, root); err != nil {
		cleanup()
		return nil, nil, err
	}
	return fs, cleanup, nil
}

// Keep prevents t's cleanup function from removing the temporary directory. It
// has no effect if cleanup has already been called.
func (t *TestFS) Keep() {
	t.keep = true
}

// TempDir returns t's temporary directory.
func (t *TestFS) TempDir() string {
	return t.tempDir
}

func (t *TestFS) cleanup() {
	if !t.keep {
		os.RemoveAll(t.tempDir)
	}
}
