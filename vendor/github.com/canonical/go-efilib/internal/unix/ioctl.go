// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package unix

import (
	"syscall"
	"unsafe"
)

func IoctlGetUint(fd int, req uint) (uint, error) {
	var value uint
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&value)))
	if err != 0 {
		return 0, err
	}
	return value, nil
}

func IoctlGetUint64(fd int, req uint) (uint64, error) {
	var value uint64
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&value)))
	if err != 0 {
		return 0, err
	}
	return value, nil
}

func IoctlSetPointerUint(fd int, req, value uint) error {
	v := value
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v)))
	if err != 0 {
		return err
	}
	return nil
}
