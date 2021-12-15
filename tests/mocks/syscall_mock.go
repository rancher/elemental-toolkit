/*
Copyright © 2021 SUSE LLC

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
