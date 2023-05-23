package vfs

import (
	"os"
	"path/filepath"
	"syscall"
)

// A Stater implements Stat. It is assumed that the os.FileInfos returned by
// Stat are compatible with os.SameFile.
type Stater interface {
	Stat(string) (os.FileInfo, error)
}

// Contains returns true if p is reachable by traversing through prefix. prefix
// must exist, but p may not. It is an expensive but accurate alternative to the
// deprecated filepath.HasPrefix.
func Contains(fs Stater, p, prefix string) (bool, error) {
	prefixFI, err := fs.Stat(prefix)
	if err != nil {
		return false, err
	}
	for {
		fi, err := fs.Stat(p)
		switch {
		case err == nil:
			if os.SameFile(fi, prefixFI) {
				return true, nil
			}
			goto TryParent
		case os.IsNotExist(err):
			goto TryParent
		case os.IsPermission(err):
			goto TryParent
		default:
			// Remove any os.PathError or os.SyscallError wrapping, if present.
			for {
				if pathError, ok := err.(*os.PathError); ok {
					err = pathError.Err
				} else if syscallError, ok := err.(*os.SyscallError); ok {
					err = syscallError.Err
				} else {
					break
				}
			}
			// Ignore some syscall.Errnos.
			if errno, ok := err.(syscall.Errno); ok {
				if _, ignore := ignoreErrnoInContains[errno]; ignore {
					goto TryParent
				}
			}
			return false, err
		}
	TryParent:
		parentDir := filepath.Dir(p)
		if parentDir == p {
			// Return when we stop making progress.
			return false, nil
		}
		p = parentDir
	}
}
