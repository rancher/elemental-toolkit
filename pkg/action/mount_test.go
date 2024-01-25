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

package action_test

import (
	"bytes"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/pkg/action"
	"github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
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

		runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
			switch cmd {
			case "findmnt":
				return []byte("/dev/loop0"), nil
			default:
				return []byte{}, nil
			}
		}

	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Write fstab", Label("mount", "fstab"), func() {
		It("Writes a simple fstab", func() {
			spec := &v1.MountSpec{
				WriteFstab: true,
				Ephemeral: v1.EphemeralMounts{
					Size: "30%",
				},
				Persistent: v1.PersistentMounts{
					Mode:  constants.BindMode,
					Paths: []string{"/some/path"},
				},
				Partitions: v1.ElementalPartitions{
					Persistent: &v1.Partition{
						Path:       "/some/device",
						MountPoint: "/mnt",
					},
				},
			}
			Expect(elemental.MountPartition(cfg.Config, spec.Partitions.Persistent)).To(Succeed())
			utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)
			err := action.WriteFstab(cfg, spec)
			Expect(err).To(BeNil())

			fstab, err := cfg.Config.Fs.ReadFile(filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(err).To(BeNil())
			expectedFstab := "/dev/loop0\t/\text2\tro,relatime\t0\t0\n"
			expectedFstab += "tmpfs\t/run/elemental/overlay\ttmpfs\tdefaults,size=30%\t0\t0\n"
			expectedFstab += "/some/device\t/mnt\tauto\tdefaults\t0\t0\n"
			expectedFstab += "/run/elemental/persistent/.state/some-path.bind\t/some/path\tnone\tdefaults,bind\t0\t0\n"
			Expect(string(fstab)).To(Equal(expectedFstab))
		})

		It("Writes a simple fstab with overlay mode", func() {
			spec := &v1.MountSpec{
				WriteFstab: true,
				Ephemeral: v1.EphemeralMounts{
					Size: "30%",
				},
				Persistent: v1.PersistentMounts{
					Mode:  constants.OverlayMode,
					Paths: []string{"/some/path"},
				},
				Partitions: v1.ElementalPartitions{
					Persistent: &v1.Partition{
						Path:       "/some/device",
						MountPoint: "/mnt",
					},
				},
			}
			Expect(elemental.MountPartition(cfg.Config, spec.Partitions.Persistent)).To(Succeed())
			utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)
			err := action.WriteFstab(cfg, spec)
			Expect(err).To(BeNil())

			fstab, err := cfg.Config.Fs.ReadFile(filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(err).To(BeNil())
			expectedFstab := "/dev/loop0\t/\text2\tro,relatime\t0\t0\n"
			expectedFstab += "tmpfs\t/run/elemental/overlay\ttmpfs\tdefaults,size=30%\t0\t0\n"
			expectedFstab += "/some/device\t/mnt\tauto\tdefaults\t0\t0\n"
			expectedFstab += "overlay\t/some/path\toverlay\t"
			expectedFstab += "defaults,lowerdir=/some/path,upperdir=/run/elemental/persistent/.state/some-path.overlay/upper,workdir=/run/elemental/persistent/.state/some-path.overlay/work,x-systemd.requires-mounts-for=/run/elemental/persistent\t0\t0\n"
			Expect(string(fstab)).To(Equal(expectedFstab))
		})

		It("Does not write fstab if not requested", func() {
			spec := &v1.MountSpec{
				WriteFstab: false,
				Ephemeral: v1.EphemeralMounts{
					Size: "30%",
				},
			}
			utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)
			err := action.WriteFstab(cfg, spec)
			Expect(err).To(BeNil())

			ok, _ := utils.Exists(fs, filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(ok).To(BeFalse())
		})
	})
})
