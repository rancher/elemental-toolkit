package modprobe

import (
	"os"

	"golang.org/x/sys/unix"
)

// Init will use the provide .ko file's os.File (created with os.Open or
// similar), to load that kernel module into the running kernel. This may error
// out for a number of reasons, such as no permission (either setcap
// CAP_SYS_MODULE or run as root), the .ko being for the wrong kernel, or the
// file not being a module at all.
//
// Any arguments to the module may be passed through `params`, such as
// `file=/root/data/backing_file`.
func Init(file *os.File, params string) error {
	return unix.FinitModule(int(file.Fd()), params, 0)
}

// InitWithFlags will preform an Init, but allow the passing of flags to the
// syscall. The `flags` parameter is a bit mask value created by ORing together
// zero or more of the following flags:
//
//   MODULE_INIT_IGNORE_MODVERSIONS - Ignore symbol version hashes
//   MODULE_INIT_IGNORE_VERMAGIC - Ignore kernel version magic.
//
// Both flags are defined in the golang.org/x/sys/unix package.
func InitWithFlags(file *os.File, params string, flags int) error {
	return unix.FinitModule(int(file.Fd()), params, flags)
}

// Remove will unload a loaded kernel module. If no such module is loaded, or if
// the module can not be unloaded, this function will return an error.
func Remove(name string) error {
	return unix.DeleteModule(name, 0)
}
