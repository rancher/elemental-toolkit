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

type FakeLuet struct {
	OnUnpackError bool
	unpackCalled  bool
}

func NewFakeLuet() *FakeLuet {
	return &FakeLuet{
		OnUnpackError: false,
		unpackCalled:  false,
	}
}

func (l *FakeLuet) Unpack(target string, image string) error {
	l.unpackCalled = true
	if l.OnUnpackError == true {
		return errors.New("Luet install error")
	}
	return nil
}

func (l FakeLuet) UnpackCalled() bool {
	return l.unpackCalled
}
