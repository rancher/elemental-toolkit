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

package utils

import (
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"testing"
)

func TestChroot(t *testing.T) {
	RegisterTestingT(t)
	syscallInterface := &v1mock.FakeSyscall{}
	mounter := &mount.FakeMounter{}
	runner := &v1mock.FakeRunner{}
	c := v1.NewRunConfig(
		v1.WithSyscall(syscallInterface),
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(mounter),
		v1.WithRunner(runner),
	)
	chroot := NewChroot(
		"/whatever",
		c,
	)
	defer chroot.Close()
	_, err := chroot.Run("chroot-command")
	Expect(err).To(BeNil())
	Expect(syscallInterface.WasChrootCalledWith("/whatever")).To(BeTrue())
}

func TestChrootFailure(t *testing.T) {
	RegisterTestingT(t)
	syscallInterface := &v1mock.FakeSyscall{ErrorOnChroot: true}
	mounter := &mount.FakeMounter{}
	runner := &v1mock.FakeRunner{}
	c := v1.NewRunConfig(
		v1.WithSyscall(syscallInterface),
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(mounter),
		v1.WithRunner(runner),
	)
	chroot := NewChroot(
		"/whatever",
		c,
	)
	defer chroot.Close()
	_, err := chroot.Run("chroot-command")
	Expect(err).ToNot(BeNil())
	Expect(syscallInterface.WasChrootCalledWith("/whatever")).To(BeTrue())
	Expect(err.Error()).To(ContainSubstring("chroot error"))
}

func TestChrootPrepareFailure(t *testing.T) {
	RegisterTestingT(t)
	syscallInterface := &v1mock.FakeSyscall{}
	mounter := &v1mock.ErrorMounter{ErrorOnMount: true}
	runner := &v1mock.FakeRunner{}
	c := v1.NewRunConfig(
		v1.WithSyscall(syscallInterface),
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(mounter),
		v1.WithRunner(runner),
	)
	chroot := NewChroot(
		"/whatever",
		c,
	)
	defer chroot.Close()
	_, err := chroot.Run("chroot-command")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("mount error"))
}

func TestChrootCloseFailure(t *testing.T) {
	RegisterTestingT(t)
	syscallInterface := &v1mock.FakeSyscall{}
	mounter := &v1mock.ErrorMounter{ErrorOnUnmount: true}
	runner := &v1mock.FakeRunner{}
	c := v1.NewRunConfig(
		v1.WithSyscall(syscallInterface),
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(mounter),
		v1.WithRunner(runner),
	)
	chroot := NewChroot(
		"/whatever",
		c,
	)
	err := chroot.Close()
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("unmount error"))
}
