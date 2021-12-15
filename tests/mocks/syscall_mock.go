package mocks

import "errors"

// FakeSyscall is a test helper method to track calls to syscall
// It can also fail on Chroot command
type FakeSyscall struct {
	chrootHistory []string // Track calls to chroot
	ErrorOnChroot bool
}

// Chroot will store the chroot call
// It can return a failure if ErrorOnChroot is true
func (f *FakeSyscall) Chroot(path string) error {
	f.chrootHistory = append(f.chrootHistory, path)
	if f.ErrorOnChroot {
		return errors.New("chroot error")
	}
	return nil
}

// WasChrootCalledWith is a helper method to check if Chroot was called with the given path
func (f *FakeSyscall) WasChrootCalledWith(path string) bool {
	for _, c := range f.chrootHistory {
		if c == path {
			return true
		}
	}
	return false
}
