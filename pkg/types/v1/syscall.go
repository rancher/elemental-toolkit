package v1

import "syscall"

type SyscallInterface interface {
	Chroot(string) error
}

type RealSyscall struct{}

func (r *RealSyscall) Chroot(path string) error {
	return syscall.Chroot(path)
}

