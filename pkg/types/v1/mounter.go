/*
Copyright © 2022 - 2023 SUSE LLC

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

package v1

import (
	"k8s.io/mount-utils"
)

// This is is just a redefinition of mount.Interface to v1.Mounter types
type Mounter interface {
	Mount(source string, target string, fstype string, options []string) error
	Unmount(target string) error
	IsLikelyNotMountPoint(file string) (bool, error)
}

func NewMounter(binary string) Mounter {
	return mount.New(binary)
}

func NewDummyMounter() Mounter {
	return mount.NewFakeMounter([]mount.MountPoint{})
}
