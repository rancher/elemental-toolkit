package mocks

import "errors"

type FakeSyscall struct {
	chrootHistory []string // Track calls to chroot
	ErrorOnChroot bool
}

func (f *FakeSyscall) Chroot(path string) error {
	f.chrootHistory = append(f.chrootHistory, path)
	if f.ErrorOnChroot {
		return errors.New("chroot error")
	}
	return nil
}

func (f *FakeSyscall) WasChrootCalledWith(path string) bool {
	for _, c := range f.chrootHistory {
		if c == path {
			return true
		}
	}
	return false
}
