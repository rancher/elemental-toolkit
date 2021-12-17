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

package v1_test

import (
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"testing"
)

func TestSetupStyleDefault(t *testing.T) {
	RegisterTestingT(t)
	c := v1.NewRunConfig(
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(&mount.FakeMounter{}),
		v1.WithRunner(&v1mock.FakeRunner{}),
		v1.WithSyscall(&v1mock.FakeSyscall{}))
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.MSDOS))
	Expect(c.BootFlag).To(Equal(v1.BOOT))
	c = v1.NewRunConfig(
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(&mount.FakeMounter{}),
		v1.WithRunner(&v1mock.FakeRunner{}),
		v1.WithSyscall(&v1mock.FakeSyscall{}))
	c.ForceEfi = true
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.GPT))
	Expect(c.BootFlag).To(Equal(v1.ESP))
	c = v1.NewRunConfig(
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(&mount.FakeMounter{}),
		v1.WithRunner(&v1mock.FakeRunner{}),
		v1.WithSyscall(&v1mock.FakeSyscall{}))
	c.ForceGpt = true
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.GPT))
	Expect(c.BootFlag).To(Equal(v1.BIOS))
}

func TestRunConfigOptions(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	mounter := mount.NewFakeMounter([]mount.MountPoint{})
	runner := &v1mock.FakeRunner{}
	sysc := &v1mock.FakeSyscall{}
	logger := v1.NewNullLogger()
	ci := &v1mock.FakeCloudInitRunner{}
	c := v1.NewRunConfig(
		v1.WithFs(fs),
		v1.WithMounter(mounter),
		v1.WithRunner(runner),
		v1.WithSyscall(sysc),
		v1.WithLogger(logger),
		v1.WithCloudInitRunner(ci),
	)
	Expect(c.Fs).To(Equal(fs))
	Expect(c.Mounter).To(Equal(mounter))
	Expect(c.Runner).To(Equal(runner))
	Expect(c.Syscall).To(Equal(sysc))
	Expect(c.Logger).To(Equal(logger))
	Expect(c.CloudInitRunner).To(Equal(ci))
}

func TestRunConfigNoMounter(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	runner := &v1mock.FakeRunner{}
	sysc := &v1mock.FakeSyscall{}
	logger := v1.NewNullLogger()
	_ = v1.NewRunConfig(
		v1.WithFs(fs),
		v1.WithRunner(runner),
		v1.WithSyscall(sysc),
		v1.WithLogger(logger),
	)
}
