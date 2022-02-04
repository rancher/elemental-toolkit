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
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental/tests/mocks"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("Config", func() {
		Describe("DigestSetup", Label("parttable", "bootflag"), func() {
			Describe("On efi system", Label("efi"), func() {
				It(fmt.Sprintf("sets part to %s and boot to %s", v1.GPT, v1.ESP), func() {
					fs := afero.NewMemMapFs()
					_, _ = fs.Create(constants.EfiDevice)

					c := v1.NewRunConfig(
						v1.WithFs(fs),
						v1.WithMounter(&mount.FakeMounter{}),
						v1.WithRunner(&v1mock.FakeRunner{}),
						v1.WithSyscall(&v1mock.FakeSyscall{}))
					c.DigestSetup()
					Expect(c.PartTable).To(Equal(v1.GPT))
					Expect(c.BootFlag).To(Equal(v1.ESP))
				})
			})
			Describe("On --force-efi flag", func() {
				It(fmt.Sprintf("sets part to %s and boot to %s", v1.GPT, v1.ESP), func() {
					c := v1.NewRunConfig(
						v1.WithFs(afero.NewMemMapFs()),
						v1.WithMounter(&mount.FakeMounter{}),
						v1.WithRunner(&v1mock.FakeRunner{}),
						v1.WithSyscall(&v1mock.FakeSyscall{}))
					c.ForceEfi = true
					c.DigestSetup()
					Expect(c.PartTable).To(Equal(v1.GPT))
					Expect(c.BootFlag).To(Equal(v1.ESP))
				})
			})
			Describe("On --force-gpt flag", func() {
				It(fmt.Sprintf("sets part to %s and boot to %s", v1.GPT, v1.BIOS), func() {
					c := v1.NewRunConfig(
						v1.WithFs(afero.NewMemMapFs()),
						v1.WithMounter(&mount.FakeMounter{}),
						v1.WithRunner(&v1mock.FakeRunner{}),
						v1.WithSyscall(&v1mock.FakeSyscall{}))
					c.ForceGpt = true
					c.DigestSetup()
					Expect(c.PartTable).To(Equal(v1.GPT))
					Expect(c.BootFlag).To(Equal(v1.BIOS))
				})
			})
			Describe("On default values", func() {
				It(fmt.Sprintf("sets part to %s and boot to %s", v1.MSDOS, v1.BOOT), func() {
					c := v1.NewRunConfig(
						v1.WithFs(afero.NewMemMapFs()),
						v1.WithMounter(&mount.FakeMounter{}),
						v1.WithRunner(&v1mock.FakeRunner{}),
						v1.WithSyscall(&v1mock.FakeSyscall{}))
					c.DigestSetup()
					Expect(c.PartTable).To(Equal(v1.MSDOS))
					Expect(c.BootFlag).To(Equal(v1.BOOT))
				})
			})
		})
		Describe("ConfigOptions", func() {
			It("Sets the proper interfaces in the config struct", func() {
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
			})
		})
		Describe("ConfigOptions no mounter specified", Label("mount", "mounter"), func() {
			It("should use the default mounter", Label("systemctl"), func() {
				fs := afero.NewMemMapFs()
				runner := &v1mock.FakeRunner{}
				sysc := &v1mock.FakeSyscall{}
				logger := v1.NewNullLogger()
				c := v1.NewRunConfig(
					v1.WithFs(fs),
					v1.WithRunner(runner),
					v1.WithSyscall(sysc),
					v1.WithLogger(logger),
				)
				Expect(c.Mounter).To(Equal(mount.New(constants.MountBinary)))
			})
		})
		Describe("PartitionList.GetByName", Label("partition"), func() {
			var c *v1.RunConfig

			BeforeEach(func() {
				fs := afero.NewMemMapFs()
				_, _ = fs.Create(constants.EfiDevice)

				c = v1.NewRunConfig(
					v1.WithFs(fs),
					v1.WithMounter(&mount.FakeMounter{}),
					v1.WithRunner(&v1mock.FakeRunner{}),
					v1.WithSyscall(&v1mock.FakeSyscall{}))
				c.DigestSetup()
			})
			It("Finds a partition given a partition label", func() {
				c.DigestSetup()
				part := c.Partitions.GetByName(constants.StatePartName)
				Expect(part.Name).To(Equal(constants.StatePartName))
			})
			It("Returns nil if requested partition label is not found", func() {
				c.DigestSetup()
				Expect(c.Partitions.GetByName("whatever")).To(BeNil())
			})
		})
	})

})
