// Copyright 2020-2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"

	internal_unix "github.com/canonical/go-efilib/internal/unix"
)

func efivarfsPath() string {
	return "/sys/firmware/efi/efivars"
}

type varFile interface {
	io.ReadWriteCloser
	Readdir(n int) ([]os.FileInfo, error)
	Stat() (os.FileInfo, error)
	GetInodeFlags() (uint, error)
	SetInodeFlags(flags uint) error
}

func makeVarFileMutableAndTakeFile(f varFile) (restore func() error, err error) {
	const immutableFlag = 0x00000010

	flags, err := f.GetInodeFlags()
	if err != nil {
		return nil, err
	}

	if flags&immutableFlag == 0 {
		// Nothing to do
		f.Close()
		return func() error { return nil }, nil
	}

	if err := f.SetInodeFlags(flags &^ immutableFlag); err != nil {
		return nil, err
	}

	return func() error {
		defer func() {
			f.Close()
		}()
		return f.SetInodeFlags(flags)
	}, nil
}

type realVarFile struct {
	*os.File
}

func (f *realVarFile) GetInodeFlags() (uint, error) {
	flags, err := internal_unix.IoctlGetUint(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	if err != nil {
		return 0, &os.PathError{Op: "ioctl", Path: f.Name(), Err: err}
	}
	return flags, nil
}

func (f *realVarFile) SetInodeFlags(flags uint) error {
	if err := internal_unix.IoctlSetPointerUint(int(f.Fd()), unix.FS_IOC_SETFLAGS, flags); err != nil {
		return &os.PathError{Op: "ioctl", Path: f.Name(), Err: err}
	}
	return nil
}

var openVarFile = func(path string, flags int, perm os.FileMode) (varFile, error) {
	f, err := os.OpenFile(path, flags, perm)
	if err != nil {
		return nil, err
	}
	return &realVarFile{f}, nil
}

var guidLength = len("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx")

func probeEfivarfs() bool {
	var st unix.Statfs_t
	if err := unixStatfs(efivarfsPath(), &st); err != nil {
		return false
	}
	if uint(st.Type) != uint(unix.EFIVARFS_MAGIC) {
		return false
	}
	return true
}

func maybeRetry(n int, fn func() (bool, error)) error {
	for i := 1; ; i++ {
		retry, err := fn()
		switch {
		case i > n:
			return err
		case !retry:
			return err
		case err == nil:
			return nil
		}
	}
}

// inodeMayBeImmutable returns whether the supplied error returned from open (for
// writing) or unlink indicates that the inode is immutable. This is indicated
// when the error is EPERM.
//
// We retry for EPERM errors that occur when opening an inode for writing,
// or when unlinking its directory entry. Although we temporarily mark inodes as
// mutable before opening them to write or when unlinking, we can get this error
// as a result of race with another process that might have been writing to the
// variable (and subsequently marked the inode as immutable again when it
// finished) or may have deleted and recreated it, making the new inode immutable.
func inodeMayBeImmutable(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}

	return errno == syscall.EPERM
}

func transformEfivarfsError(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist) || err == io.EOF:
		// ENOENT can come from the VFS layer during opening and unlinking, and
		// converted from EFI_NOT_FOUND errors returned from the variable
		// service. When reading a variable, if the variable doesn't exist
		// when trying to determine the size of it, the kernel converts ENOENT
		// into success with 0 bytes read which means we need to handle io.EOF
		// as well.
		return ErrVarNotExist
	case errors.Is(err, syscall.EINVAL):
		// EINVAL can come from the VFS layer during opening or unlinking due
		// to invalid or incompatible flag combinations, although we don't expect
		// that. It's also converted from EFI_INVALID_PARAMETER errors returned
		// from the variable service.
		return ErrVarInvalidParam
	case errors.Is(err, syscall.EIO):
		// EIO can come from the VFS layer during unlinking, although we don't
		// expect that. It's also converted from EFI_DEVICE_ERROR errors returned
		// from the variable service
		return ErrVarDeviceError
	case errors.Is(err, os.ErrPermission):
		// EACCESS can come from the VFS layer for the following reasons:
		// - opening a file for writing when the caller does not have write
		//   access to it.
		// - opening a file for writing when the caller does not have write
		//   access to the parent directory and a new file needs to be created.
		// - unlinking a file when the caller does not have write access to
		//   the parent directory.
		// EPERM can come from the VFS layer when opening a file for writing
		// or unlinking if the inode is immutable (see inodeMayBeImmutable).
		//
		// EACCES is also converted from EFI_SECURITY_VIOLATION errors returned
		// from the variable service.
		return ErrVarPermission
	case errors.Is(err, syscall.ENOSPC):
		// ENOSPC is converted from EFI_OUT_OF_RESOURCES errors returned from
		// the variable service.
		return ErrVarInsufficientSpace
	case errors.Is(err, syscall.EROFS):
		// EROFS is converted from EFI_WRITE_PROTECTED errors returned from the
		// variable service.
		return ErrVarWriteProtected
	default:
		return err
	}
}

func writeEfivarfsFile(path string, attrs VariableAttributes, data []byte) (retry bool, err error) {
	// Open for reading to make the inode mutable
	r, err := openVarFile(path, os.O_RDONLY, 0)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// It's not an error if the variable doesn't exist.
	case err != nil:
		return false, transformEfivarfsError(err)
	default:
		restoreImmutable, err := makeVarFileMutableAndTakeFile(r)
		if err != nil {
			r.Close()
			return false, transformEfivarfsError(err)
		}

		defer restoreImmutable()
	}

	if len(data) == 0 {
		// short-cut for unauthenticated variable delete - efivarfs will perform a
		// zero-byte write to delete the variable if we unlink the entry here.
		if attrs&(AttributeAuthenticatedWriteAccess|AttributeTimeBasedAuthenticatedWriteAccess|AttributeEnhancedAuthenticatedAccess) > 0 {
			// If the supplied attributes are incompatible with the variable,
			// the variable service will return EFI_INVALID_PARAMETER and
			// we'll get EINVAL back. If the supplied attributes are correct
			// but we perform a zero-byte write to an authenticated vaiable,
			// the variable service will return EFI_SECURITY_VIOLATION, but
			// the kernel also turns this into EINVAL. Instead, we generate
			// an appropriate error if the supplied attributes indicate that
			// the variable is authenticated.
			return false, ErrVarPermission
		}
		if err := removeVarFile(path); err != nil {
			switch {
			case errors.Is(err, os.ErrNotExist):
				// It's not an error if the variable doesn't exist.
				return false, nil
			case inodeMayBeImmutable(err):
				// Try again
				return true, transformEfivarfsError(err)
			default:
				// Don't try again
				return false, transformEfivarfsError(err)
			}
		}
		return false, nil
	}

	flags := os.O_WRONLY | os.O_CREATE
	if attrs&AttributeAppendWrite != 0 {
		flags |= os.O_APPEND
	}

	w, err := openVarFile(path, flags, 0644)
	switch {
	case inodeMayBeImmutable(err):
		// Try again
		return true, transformEfivarfsError(err)
	case err != nil:
		// Don't try again
		return false, transformEfivarfsError(err)
	}
	defer w.Close()

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, attrs)
	buf.Write(data)

	_, err = buf.WriteTo(w)
	return false, transformEfivarfsError(err)
}

type efivarfsVarsBackend struct{}

func (v efivarfsVarsBackend) Get(name string, guid GUID) (VariableAttributes, []byte, error) {
	path := filepath.Join(efivarfsPath(), fmt.Sprintf("%s-%s", name, guid))
	f, err := openVarFile(path, os.O_RDONLY, 0)
	if err != nil {
		return 0, nil, transformEfivarfsError(err)
	}
	defer f.Close()

	// Read the entire payload in a single read, as that's how
	// GetVariable works and is the only way the kernel can obtain
	// the variable contents. If we perform multiple reads, the
	// kernel still has to obtain the entire variable contents
	// each time. To do this, we need to know the size of the variable
	// contents, which we can obtain from the inode.
	fi, err := f.Stat()
	if err != nil {
		return 0, nil, err
	}
	if fi.Size() < 4 {
		return 0, nil, ErrVarNotExist
	}

	buf := make([]byte, fi.Size())
	if _, err := f.Read(buf); err != nil {
		return 0, nil, transformEfivarfsError(err)
	}

	return VariableAttributes(binary.LittleEndian.Uint32(buf)), buf[4:], nil
}

func (v efivarfsVarsBackend) Set(name string, guid GUID, attrs VariableAttributes, data []byte) error {
	path := filepath.Join(efivarfsPath(), fmt.Sprintf("%s-%s", name, guid))
	return maybeRetry(4, func() (bool, error) { return writeEfivarfsFile(path, attrs, data) })
}

func (v efivarfsVarsBackend) List() ([]VariableDescriptor, error) {
	f, err := openVarFile(efivarfsPath(), os.O_RDONLY, 0)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, ErrVarsUnavailable
	case err != nil:
		return nil, transformEfivarfsError(err)
	}
	defer f.Close()

	dirents, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}

	var entries []VariableDescriptor

	for _, dirent := range dirents {
		if !dirent.Mode().IsRegular() {
			// Skip non-regular files
			continue
		}
		if len(dirent.Name()) < guidLength+1 {
			// Skip files with a basename that isn't long enough
			// to contain a GUID and a hyphen
			continue
		}
		if dirent.Name()[len(dirent.Name())-guidLength-1] != '-' {
			// Skip files where the basename doesn't contain a
			// hyphen between the name and GUID
			continue
		}
		if dirent.Size() == 0 {
			// Skip files with zero size. These are variables that
			// have been deleted by writing an empty payload
			continue
		}

		name := dirent.Name()[:len(dirent.Name())-guidLength-1]
		guid, err := DecodeGUIDString(dirent.Name()[len(name)+1:])
		if err != nil {
			continue
		}

		entries = append(entries, VariableDescriptor{Name: name, GUID: guid})
	}

	return entries, nil
}

func addDefaultVarsBackend(ctx context.Context) context.Context {
	if !probeEfivarfs() {
		return withVarsBackend(ctx, nullVarsBackend{})
	}
	return withVarsBackend(ctx, efivarfsVarsBackend{})
}
