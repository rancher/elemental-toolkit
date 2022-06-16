/*
Copyright © 2022 SUSE LLC

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

	luetTypes "github.com/mudler/luet/pkg/api/core/types"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
)

type FakeLuet struct {
	OnUnpackError               bool
	OnUnpackFromChannelError    bool
	UnpackSideEffect            func(string, string, bool) error
	UnpackFromChannelSideEffect func(string, string, ...v1.Repository) error
	unpackCalled                bool
	unpackFromChannelCalled     bool
	plugins                     []string
	arch                        string
}

func NewFakeLuet() *FakeLuet {
	return &FakeLuet{}
}

func (l *FakeLuet) Unpack(target string, image string, local bool) error {
	l.unpackCalled = true
	if l.OnUnpackError {
		return errors.New("Luet install error")
	}
	if l.UnpackSideEffect != nil {
		return l.UnpackSideEffect(target, image, local)
	}
	return nil
}

func (l *FakeLuet) UnpackFromChannel(target string, pkg string, repos ...v1.Repository) error {
	l.unpackFromChannelCalled = true
	if l.OnUnpackFromChannelError {
		return errors.New("Luet install error")
	}
	if l.UnpackFromChannelSideEffect != nil {
		return l.UnpackFromChannelSideEffect(target, pkg, repos...)
	}
	return nil
}

func (l FakeLuet) UnpackCalled() bool {
	return l.unpackCalled
}

func (l FakeLuet) UnpackChannelCalled() bool {
	return l.unpackFromChannelCalled
}

func (l FakeLuet) OverrideConfig(config *luetTypes.LuetConfig) {}

func (l *FakeLuet) SetPlugins(plugins ...string) {
	l.plugins = plugins
}

func (l *FakeLuet) GetPlugins() []string {
	return l.plugins
}

func (l *FakeLuet) SetArch(arch string) {
	l.arch = arch
}
