package mocks

import (
	"errors"
	"k8s.io/mount-utils"
)

// ErrorMounter is a fake mounter for tests that can error out.
type ErrorMounter struct {
	ErrorOnMount   bool
	ErrorOnUnmount bool
}

// Mount will return an error if ErrorOnMount is true
func (e ErrorMounter) Mount(source string, target string, fstype string, options []string) error {
	if e.ErrorOnMount {
		return errors.New("mount error")
	}
	return nil
}

// Unmount will return an error if ErrorOnUnmount is true
func (e ErrorMounter) Unmount(target string) error {
	if e.ErrorOnUnmount {
		return errors.New("unmount error")
	}
	return nil
}

// We need to have this below to fulfill the interface for mount.Interface

func (e ErrorMounter) MountSensitive(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	return nil
}
func (e ErrorMounter) MountSensitiveWithoutSystemd(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	return nil
}
func (e ErrorMounter) MountSensitiveWithoutSystemdWithMountFlags(source string, target string, fstype string, options []string, sensitiveOptions []string, mountFlags []string) error {
	return nil
}
func (e ErrorMounter) List() ([]mount.MountPoint, error)               { return []mount.MountPoint{}, nil }
func (e ErrorMounter) IsLikelyNotMountPoint(file string) (bool, error) { return true, nil }
func (e ErrorMounter) GetMountRefs(pathname string) ([]string, error)  { return []string{}, nil }
