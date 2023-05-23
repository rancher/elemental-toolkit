// Package vfst provides helper functions for testing code that uses
// github.com/twpayne/go-vfs.
package vfst

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	vfs "github.com/twpayne/go-vfs"
)

//nolint:gochecknoglobals
var umask os.FileMode

// A Dir is a directory with a specified permissions and zero or more Entries.
type Dir struct {
	Perm    os.FileMode
	Entries map[string]interface{}
}

// A File is a file with a specified permissions and contents.
type File struct {
	Perm     os.FileMode
	Contents []byte
}

// A Symlink is a symbolic link with a specified target.
type Symlink struct {
	Target string
}

// A Test is a test on an vfs.FS.
type Test func(*testing.T, vfs.FS)

// A PathTest is a test on a specified path in an vfs.FS.
type PathTest func(*testing.T, vfs.FS, string)

// A BuilderOption sets an option on a Builder.
type BuilderOption func(*Builder)

// A Builder populates an vfs.FS.
type Builder struct {
	umask   os.FileMode
	verbose bool
}

// BuilderUmask sets a builder's umask.
func BuilderUmask(umask os.FileMode) BuilderOption {
	return func(b *Builder) {
		b.umask = umask
	}
}

// BuilderVerbose sets a builder's verbose flag. When true, the builder will
// log all operations with the standard log package.
func BuilderVerbose(verbose bool) BuilderOption {
	return func(b *Builder) {
		b.verbose = verbose
	}
}

// NewBuilder returns a new Builder with the given options set.
func NewBuilder(options ...BuilderOption) *Builder {
	b := &Builder{
		umask:   umask,
		verbose: false,
	}
	for _, option := range options {
		option(b)
	}
	return b
}

// build is a recursive helper for Build.
func (b *Builder) build(fs vfs.FS, path string, i interface{}) error {
	switch i := i.(type) {
	case []interface{}:
		for _, element := range i {
			if err := b.build(fs, path, element); err != nil {
				return err
			}
		}
		return nil
	case *Dir:
		if parentDir := filepath.Dir(path); parentDir != "." {
			if err := b.MkdirAll(fs, parentDir, 0o777); err != nil {
				return err
			}
		}
		if err := b.Mkdir(fs, path, i.Perm); err != nil {
			return err
		}
		entryNames := make([]string, 0, len(i.Entries))
		for entryName := range i.Entries {
			entryNames = append(entryNames, entryName)
		}
		sort.Strings(entryNames)
		for _, entryName := range entryNames {
			if err := b.build(fs, filepath.Join(path, entryName), i.Entries[entryName]); err != nil {
				return err
			}
		}
		return nil
	case map[string]interface{}:
		if err := b.MkdirAll(fs, path, 0o777); err != nil {
			return err
		}
		entryNames := make([]string, 0, len(i))
		for entryName := range i {
			entryNames = append(entryNames, entryName)
		}
		sort.Strings(entryNames)
		for _, entryName := range entryNames {
			if err := b.build(fs, filepath.Join(path, entryName), i[entryName]); err != nil {
				return err
			}
		}
		return nil
	case map[string]string:
		if err := b.MkdirAll(fs, path, 0o777); err != nil {
			return err
		}
		entryNames := make([]string, 0, len(i))
		for entryName := range i {
			entryNames = append(entryNames, entryName)
		}
		sort.Strings(entryNames)
		for _, entryName := range entryNames {
			if err := b.WriteFile(fs, filepath.Join(path, entryName), []byte(i[entryName]), 0o666); err != nil {
				return err
			}
		}
		return nil
	case *File:
		return b.WriteFile(fs, path, i.Contents, i.Perm)
	case string:
		return b.WriteFile(fs, path, []byte(i), 0o666)
	case []byte:
		return b.WriteFile(fs, path, i, 0o666)
	case *Symlink:
		return b.Symlink(fs, i.Target, path)
	case nil:
		return nil
	default:
		return fmt.Errorf("%s: unsupported type %T", path, i)
	}
}

// Build populates fs from root.
func (b *Builder) Build(fs vfs.FS, root interface{}) error {
	return b.build(fs, "/", root)
}

// Mkdir creates directory path with permissions perm. It is idempotent and
// will not fail if path already exists, is a directory, and has permissions
// perm.
func (b *Builder) Mkdir(fs vfs.FS, path string, perm os.FileMode) error {
	if info, err := fs.Lstat(path); os.IsNotExist(err) {
		if b.verbose {
			log.Printf("mkdir -m 0%o %s", perm&^b.umask, path)
		}
		return fs.Mkdir(path, perm&^b.umask)
	} else if err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("%s: not a directory", path)
	} else if gotPerm, wantPerm := info.Mode()&os.ModePerm, perm&^b.umask; !permEqual(gotPerm, wantPerm) {
		return fmt.Errorf("%s has permissions 0%o, want 0%o", path, gotPerm, wantPerm)
	}
	return nil
}

// MkdirAll creates directory path and any missing parent directories with
// permissions perm. It is idempotent and will not file if path already exists
// and is a directory.
func (b *Builder) MkdirAll(fs vfs.FS, path string, perm os.FileMode) error {
	// Check path.
	info, err := fs.Lstat(path)
	switch {
	case err != nil && os.IsNotExist(err):
		// path does not exist, fallthrough to create.
	case err == nil && info.IsDir():
		// path already exists and is a directory.
		return nil
	case err == nil && !info.IsDir():
		// path already exists, but is not a directory.
		return err
	default:
		// Some other error.
		return err
	}

	// Create path.
	if b.verbose {
		log.Printf("mkdir -p -m 0%o %s", perm&^b.umask, path)
	}
	return vfs.MkdirAll(fs, path, perm&^b.umask)
}

// Symlink creates a symbolic link from newname to oldname. It will create any
// missing parent directories with default permissions. It is idempotent and
// will not fail if the symbolic link already exists and points to oldname.
func (b *Builder) Symlink(fs vfs.FS, oldname, newname string) error {
	// Check newname.
	info, err := fs.Lstat(newname)
	switch {
	case err == nil && info.Mode()&os.ModeType != os.ModeSymlink:
		// newname exists, but it's not a symlink.
		return fmt.Errorf("%s: not a symbolic link", newname)
	case err == nil:
		// newname exists, and it's a symlink. Check that it is a symlink to
		// oldname.
		gotTarget, err := fs.Readlink(newname)
		if err != nil {
			return err
		}
		if gotTarget != oldname {
			return fmt.Errorf("%s: has target %s, want %s", newname, gotTarget, oldname)
		}
		return nil
	case os.IsNotExist(err):
		// newname does not exist, fallthrough to create.
	default:
		// Some other error, return it.
		return err
	}

	// Create newname.
	if err := b.MkdirAll(fs, filepath.Dir(newname), 0o777); err != nil {
		return err
	}
	if b.verbose {
		log.Printf("ln -s %s %s", oldname, newname)
	}
	return fs.Symlink(oldname, newname)
}

// WriteFile writes file path withe contents contents and permissions perm. It
// will create any missing parent directories with default permissions. It is
// idempotent and will not fail if the file already exists, has contents
// contents, and permissions perm.
func (b *Builder) WriteFile(fs vfs.FS, path string, contents []byte, perm os.FileMode) error {
	if info, err := fs.Lstat(path); os.IsNotExist(err) {
		// fallthrough to fs.WriteFile
	} else if err != nil {
		return err
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file", path)
	} else if gotPerm, wantPerm := info.Mode()&os.ModePerm, perm&^b.umask; !permEqual(gotPerm, wantPerm) {
		return fmt.Errorf("%s has permissions 0%o, want 0%o", path, gotPerm, wantPerm)
	} else {
		gotContents, err := fs.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(gotContents, contents) {
			return fmt.Errorf("%s: has contents %v, want %v", path, gotContents, contents)
		}
		return nil
	}
	if err := b.MkdirAll(fs, filepath.Dir(path), 0o777); err != nil {
		return err
	}
	if b.verbose {
		log.Printf("install -m 0%o /dev/null %s", perm&^b.umask, path)
	}
	return fs.WriteFile(path, contents, perm&^b.umask)
}

// runTests recursively runs tests on fs.
func runTests(t *testing.T, fs vfs.FS, name string, test interface{}) {
	t.Helper()
	prefix := ""
	if name != "" {
		prefix = name + "_"
	}
	switch test := test.(type) {
	case Test:
		test(t, fs)
	case []Test:
		for i, test := range test {
			t.Run(prefix+strconv.Itoa(i), func(t *testing.T) {
				//nolint:scopelint
				test(t, fs)
			})
		}
	case map[string]Test:
		testNames := make([]string, 0, len(test))
		for testName := range test {
			testNames = append(testNames, testName)
		}
		sort.Strings(testNames)
		for _, testName := range testNames {
			t.Run(prefix+testName, func(t *testing.T) {
				//nolint:scopelint
				test[testName](t, fs)
			})
		}
	case []interface{}:
		for _, u := range test {
			runTests(t, fs, name, u)
		}
	case map[string]interface{}:
		testNames := make([]string, 0, len(test))
		for testName := range test {
			testNames = append(testNames, testName)
		}
		sort.Strings(testNames)
		for _, testName := range testNames {
			runTests(t, fs, prefix+testName, test[testName])
		}
	case nil:
	default:
		t.Fatalf("%s: unsupported type %T", name, test)
	}
}

// RunTests recursively runs tests on fs.
func RunTests(t *testing.T, fs vfs.FS, name string, tests ...interface{}) {
	t.Helper()
	runTests(t, fs, name, tests)
}

// TestContents returns a PathTest that verifies the contents of the file are
// equal to wantContents.
func TestContents(wantContents []byte) PathTest {
	return func(t *testing.T, fs vfs.FS, path string) {
		t.Helper()
		if gotContents, err := fs.ReadFile(path); err != nil || !bytes.Equal(gotContents, wantContents) {
			t.Errorf("fs.ReadFile(%q) == %v, %v, want %v, <nil>", path, gotContents, err, wantContents)
		}
	}
}

// TestContentsString returns a PathTest that verifies the contetnts of the
// file are equal to wantContentsStr.
func TestContentsString(wantContentsStr string) PathTest {
	return func(t *testing.T, fs vfs.FS, path string) {
		t.Helper()
		if gotContents, err := fs.ReadFile(path); err != nil || string(gotContents) != wantContentsStr {
			t.Errorf("fs.ReadFile(%q) == %q, %v, want %q, <nil>", path, gotContents, err, wantContentsStr)
		}
	}
}

// testDoesNotExist is a PathTest that verifies that a file or directory does
// not exist.
//nolint:gochecknoglobals
var testDoesNotExist = func(t *testing.T, fs vfs.FS, path string) {
	t.Helper()
	_, err := fs.Lstat(path)
	if got, want := os.IsNotExist(err), true; got != want {
		t.Errorf("_, err := fs.Lstat(%q); os.IsNotExist(err) == %v, want %v", path, got, want)
	}
}

// TestDoesNotExist is a PathTest that verifies that a file or directory does
// not exist.
//nolint:gochecknoglobals
var TestDoesNotExist PathTest = testDoesNotExist

// TestIsDir is a PathTest that verifies that the path is a directory.
//nolint:gochecknoglobals
var TestIsDir = TestModeType(os.ModeDir)

// TestModePerm returns a PathTest that verifies that the path's permissions
// are equal to wantPerm.
func TestModePerm(wantPerm os.FileMode) PathTest {
	return func(t *testing.T, fs vfs.FS, path string) {
		t.Helper()
		info, err := fs.Lstat(path)
		if err != nil {
			t.Errorf("fs.Lstat(%q) == %+v, %v, want !<nil>, <nil>", path, info, err)
			return
		}
		if gotPerm := info.Mode() & os.ModePerm; !permEqual(gotPerm, wantPerm) {
			t.Errorf("fs.Lstat(%q).Mode()&os.ModePerm == 0%o, want 0%o", path, gotPerm, wantPerm)
		}
	}
}

// TestModeIsRegular is a PathTest that tests that the path is a regular file.
//nolint:gochecknoglobals
var TestModeIsRegular = TestModeType(0)

// TestModeType returns a PathTest that verifies that the path's mode type is
// equal to wantModeType.
func TestModeType(wantModeType os.FileMode) PathTest {
	return func(t *testing.T, fs vfs.FS, path string) {
		t.Helper()
		info, err := fs.Lstat(path)
		if err != nil {
			t.Errorf("fs.Lstat(%q) == %+v, %v, want !<nil>, <nil>", path, info, err)
			return
		}
		if gotModeType := info.Mode() & os.ModeType; gotModeType != wantModeType {
			t.Errorf("fs.Lstat(%q).Mode()&os.ModeType == %v, want %v", path, gotModeType, wantModeType)
		}
	}
}

// TestPath returns a Test that runs pathTests on path.
func TestPath(path string, pathTests ...PathTest) Test {
	return func(t *testing.T, fs vfs.FS) {
		t.Helper()
		for i, pathTest := range pathTests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				//nolint:scopelint
				pathTest(t, fs, path)
			})
		}
	}
}

// TestSize returns a PathTest that tests that path's Size() is equal to
// wantSize.
func TestSize(wantSize int64) PathTest {
	return func(t *testing.T, fs vfs.FS, path string) {
		t.Helper()
		info, err := fs.Lstat(path)
		if err != nil {
			t.Errorf("fs.Lstat(%q) == %+v, %v, want !<nil>, <nil>", path, info, err)
			return
		}
		if gotSize := info.Size(); gotSize != wantSize {
			t.Errorf("fs.Lstat(%q).Size() == %d, want %d", path, gotSize, wantSize)
		}
	}
}

// TestSymlinkTarget returns a PathTest that tests that path's target is wantTarget.
func TestSymlinkTarget(wantTarget string) PathTest {
	return func(t *testing.T, fs vfs.FS, path string) {
		t.Helper()
		if gotTarget, err := fs.Readlink(path); err != nil || gotTarget != wantTarget {
			t.Errorf("fs.Readlink(%q) == %q, %v, want %q, <nil>", path, gotTarget, err, wantTarget)
			return
		}
	}
}

// TestMinSize returns a PathTest that tests that path's Size() is at least
// wantMinSize.
func TestMinSize(wantMinSize int64) PathTest {
	return func(t *testing.T, fs vfs.FS, path string) {
		t.Helper()
		info, err := fs.Lstat(path)
		if err != nil {
			t.Errorf("fs.Lstat(%q) == %+v, %v, want !<nil>, <nil>", path, info, err)
			return
		}
		if gotSize := info.Size(); gotSize < wantMinSize {
			t.Errorf("fs.Lstat(%q).Size() == %d, want >=%d", path, gotSize, wantMinSize)
		}
	}
}
