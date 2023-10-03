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

package action_test

import (
	"bytes"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/pkg/action"
	"github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var _ = Describe("Mount Action", func() {
	var cfg *v1.RunConfig
	var mounter *mount.FakeMounter
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var cleanup func()
	var memLog *bytes.Buffer

	BeforeEach(func() {
		mounter = &mount.FakeMounter{}
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		runner = v1mock.NewFakeRunner()
		logger.SetLevel(logrus.DebugLevel)
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		cfg = config.NewRunConfig(
			config.WithFs(fs),
			config.WithMounter(mounter),
			config.WithLogger(logger),
			config.WithRunner(runner),
		)
	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Write fstab", Label("mount", "fstab"), func() {
		It("Writes a simple fstab", func() {
			spec := &v1.MountSpec{
				WriteFstab: true,
				Image: &v1.Image{
					LoopDevice: "/dev/loop0",
				},
				Overlay: v1.OverlayMounts{
					Size: "30%",
				},
			}
			utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)
			err := action.WriteFstab(cfg, spec)
			Expect(err).To(BeNil())

			fstab, err := cfg.Config.Fs.ReadFile(filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(err).To(BeNil())
			Expect(string(fstab)).To(Equal("/dev/loop0\t/\tauto\tro\t0\t0\ntmpfs\t/run/elemental/overlay\ttmpfs\tdefaults,size=30%\t0\t0\n"))
		})
	})
	Describe("Mounts image", Label("mount", "image"), func() {
		var mountedImage string
		var fsckedDevices []string

		BeforeEach(func() {
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				switch cmd {
				case "systemd-fsck":
					fsckedDevices = append(fsckedDevices, args[0])
					return []byte{}, nil
				case "losetup":
					mountedImage = args[2]
					return []byte{}, nil
				default:
					return []byte{}, nil
				}
			}
		})
		It("Mounts the specified image to it's mountpoint", func() {
			spec := &v1.MountSpec{
				Image: &v1.Image{
					MountPoint: "/recovery",
					File:       constants.RecoveryImgPath,
				},
				Overlay: v1.OverlayMounts{Type: constants.Tmpfs},
			}

			err := action.RunMount(cfg, spec)
			Expect(err).To(BeNil())
			Expect(mountedImage).To(Equal(constants.RecoveryImgPath))

			Expect(len(fsckedDevices)).To(Equal(0))
		})
		It("Runs fsck on partitions", func() {
			spec := &v1.MountSpec{
				RunFsck: true,
				Image: &v1.Image{
					MountPoint: "/sysroot",
					File:       constants.ActiveImgPath,
				},
				Overlay: v1.OverlayMounts{Type: constants.Tmpfs},
			}

			err := action.RunMount(cfg, spec)
			Expect(err).To(BeNil())
			Expect(mountedImage).To(Equal(constants.ActiveImgPath))
			Expect(len(fsckedDevices)).ToNot(Equal(0))
		})
	})
})
