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

package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/config"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental/tests/mocks"
	"github.com/twpayne/go-vfs/vfst"
	"k8s.io/mount-utils"
)

var _ = Describe("Types", Label("types", "config"), func() {
	Describe("Config", func() {
		Describe("ConfigOptions", func() {
			It("Sets the proper interfaces in the config struct", func() {
				fs, cleanup, err := vfst.NewTestFS(nil)
				defer cleanup()
				Expect(err).ToNot(HaveOccurred())
				mounter := mount.NewFakeMounter([]mount.MountPoint{})
				runner := v1mock.NewFakeRunner()
				client := &v1mock.FakeHTTPClient{}
				sysc := &v1mock.FakeSyscall{}
				logger := v1.NewNullLogger()
				ci := &v1mock.FakeCloudInitRunner{}
				luet := &v1mock.FakeLuet{}
				c := config.NewRunConfig(
					config.WithFs(fs),
					config.WithMounter(mounter),
					config.WithRunner(runner),
					config.WithSyscall(sysc),
					config.WithLogger(logger),
					config.WithCloudInitRunner(ci),
					config.WithClient(client),
					config.WithLuet(luet),
				)
				Expect(c.Fs).To(Equal(fs))
				Expect(c.Mounter).To(Equal(mounter))
				Expect(c.Runner).To(Equal(runner))
				Expect(c.Syscall).To(Equal(sysc))
				Expect(c.Logger).To(Equal(logger))
				Expect(c.CloudInitRunner).To(Equal(ci))
				Expect(c.Client).To(Equal(client))
				Expect(c.Luet).To(Equal(luet))
			})
			It("Sets the runner if we dont pass one", func() {
				fs, cleanup, err := vfst.NewTestFS(nil)
				defer cleanup()
				Expect(err).ToNot(HaveOccurred())
				mounter := mount.NewFakeMounter([]mount.MountPoint{})
				c := config.NewRunConfig(
					config.WithFs(fs),
					config.WithMounter(mounter),
				)
				Expect(c.Fs).To(Equal(fs))
				Expect(c.Mounter).To(Equal(mounter))
				Expect(c.Runner).ToNot(BeNil())
			})
		})
		Describe("ConfigOptions no mounter specified", Label("mount", "mounter"), func() {
			It("should use the default mounter", Label("systemctl"), func() {
				runner := v1mock.NewFakeRunner()
				sysc := &v1mock.FakeSyscall{}
				logger := v1.NewNullLogger()
				c := config.NewRunConfig(
					config.WithRunner(runner),
					config.WithSyscall(sysc),
					config.WithLogger(logger),
				)
				Expect(c.Mounter).To(Equal(mount.New(constants.MountBinary)))
			})
		})
		Describe("BuildConfig", func() {
			build := config.NewBuildConfig()
			Expect(build.Name).To(Equal(constants.BuildImgName))
			Expect(build.ISO.BootCatalog).To(Equal(constants.IsoBootCatalog))
		})
	})

})
