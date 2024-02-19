/*
Copyright © 2022 - 2024 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mocks

import (
	"errors"

	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
	"k8s.io/mount-utils"
)

var _ v2.Mounter = (*FakeMounter)(nil)

// FakeMounter is a fake mounter for tests that can error out.
type FakeMounter struct {
	ErrorOnMount   bool
	ErrorOnUnmount bool
	FakeMounter    mount.Interface
}

// NewFakeMounter returns an FakeMounter with an instance of FakeMounter inside so we can use its functions
func NewFakeMounter() *FakeMounter {
	return &FakeMounter{
		FakeMounter: &mount.FakeMounter{},
	}
}

// Mount will return an error if ErrorOnMount is true
func (e FakeMounter) Mount(source string, target string, fstype string, options []string) error {
	if e.ErrorOnMount {
		return errors.New("mount error")
	}
	return e.FakeMounter.Mount(source, target, fstype, options)
}

// Unmount will return an error if ErrorOnUnmount is true
func (e FakeMounter) Unmount(target string) error {
	if e.ErrorOnUnmount {
		return errors.New("unmount error")
	}
	return e.FakeMounter.Unmount(target)
}

func (e FakeMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	mnts, _ := e.List()

	for _, mnt := range mnts {
		if file == mnt.Path {
			return false, nil
		}
	}
	return true, nil
}

// This is not part of the interface, just a helper method for tests
func (e FakeMounter) List() ([]mount.MountPoint, error) {
	return e.FakeMounter.List()
}
