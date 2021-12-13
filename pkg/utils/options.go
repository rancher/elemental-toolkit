package utils

import (
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
)

type ChrootOptions func(a *Chroot) error
type GrubOptions func(a *Grub) error

func WithRunner(runner v1.Runner) func(r *Chroot) error {
	return func(a *Chroot) error {
		a.runner = runner
		return nil
	}
}

func WithRunnerGrub(runner v1.Runner) func(r *Grub) error {
	return func(a *Grub) error {
		a.runner = runner
		return nil
	}
}

func WithSyscall(syscall v1.SyscallInterface) func(r *Chroot) error {
	return func(a *Chroot) error {
		a.syscall = syscall
		return nil
	}
}

func WithFS(fs afero.Fs) func(r *Chroot) error {
	return func(a *Chroot) error {
		a.fs = fs
		return nil
	}
}

func WithMounter(mounter mount.Interface) func(r *Chroot) error {
	return func(a *Chroot) error {
		a.mounter = mounter
		return nil
	}
}

func WithLogger(logger v1.Logger) func(r *Chroot) error {
	return func(a *Chroot) error {
		a.logger = logger
		return nil
	}
}
