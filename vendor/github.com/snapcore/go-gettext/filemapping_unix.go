// +build !windows

package gettext

import (
	"os"
	"syscall"
)

func (m *fileMapping) tryMap(f *os.File, size int64) error {
	var err error
	m.data, err = syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return err
	}
	m.isMapped = true
	return nil
}

func (m *fileMapping) closeMapping() error {
	return syscall.Munmap(m.data)
}
