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
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	"github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ = Describe("Mount Action", func() {
	var cfg *types.RunConfig
	var mounter *mocks.FakeMounter
	var runner *mocks.FakeRunner
	var syscall *mocks.FakeSyscall
	var fs vfs.FS
	var logger types.Logger
	var cleanup func()
	var memLog *bytes.Buffer
	var spec *types.MountSpec

	BeforeEach(func() {
		mounter = mocks.NewFakeMounter()
		memLog = &bytes.Buffer{}
		logger = types.NewBufferLogger(memLog)
		runner = mocks.NewFakeRunner()
		syscall = &mocks.FakeSyscall{}
		logger.SetLevel(logrus.DebugLevel)
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		cfg = config.NewRunConfig(
			config.WithFs(fs),
			config.WithMounter(mounter),
			config.WithLogger(logger),
			config.WithRunner(runner),
			config.WithSyscall(syscall),
		)

		runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
			switch cmd {
			case "findmnt":
				mountPoints := "/dev/loop0\t/sysroot\text2\tro,relatime\n"
				mountPoints += "/dev/loop1\t/sysroot/volume\text2\tro,relatime\n"
				mountPoints += "/dev/loop2\t/run/elemental/extra\txfs\trw,relatime\n"
				mountPoints += "/dev/sda4\t/run/initramfs/elemental-state\text2\tro,relatime\n"
				return []byte(mountPoints), nil
			default:
				return []byte{}, nil
			}
		}

		spec = &types.MountSpec{
			Sysroot:    "/sysroot",
			WriteFstab: true,
			Ephemeral: types.EphemeralMounts{
				Type: "tmpfs",
				Size: "30%",
			},
			Persistent: types.PersistentMounts{
				Mode:  constants.BindMode,
				Paths: []string{"/some/path"},
				Volume: types.VolumeMount{
					Mountpoint: constants.PersistentDir,
					Device:     "/dev/persistentdev",
				},
			},
			Volumes: []*types.VolumeMount{
				{
					Mountpoint: "/run/elemental",
					Device:     "/dev/somedevice",
					Options:    []string{"rw", "defaults"},
					FSType:     "vfat",
				},
			},
		}

	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Write fstab", Label("mount", "fstab"), func() {
		It("Writes a simple fstab", func() {
			Expect(utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)).To(Succeed())
			fstabData, err := action.InitialFstabData(runner, spec.Sysroot)
			Expect(err).To(BeNil())
			err = action.WriteFstab(cfg, spec, fstabData)
			Expect(err).To(BeNil())

			fstab, err := cfg.Config.Fs.ReadFile(filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(err).To(BeNil())
			expectedFstab := "/dev/loop0\t/\text2\tro,relatime\t0\t0\n"
			expectedFstab += "/dev/loop1\t/volume\text2\tro,relatime\t0\t0\n"
			expectedFstab += "/dev/loop2\t/run/elemental/extra\txfs\trw,relatime\t0\t0\n"
			expectedFstab += "/dev/sda4\t/run/initramfs/elemental-state\text2\tro,relatime\t0\t0\n"
			expectedFstab += "/dev/somedevice\t/run/elemental\tvfat\trw,defaults\t0\t0\n"
			expectedFstab += "/dev/persistentdev\t/run/elemental/persistent\tauto\tdefaults\t0\t0\n"
			expectedFstab += "/run/elemental/persistent/.state/some-path.bind\t/some/path\tnone\tdefaults,bind\t0\t0\n"
			expectedFstab += "tmpfs\t/run/elemental/overlay\ttmpfs\tdefaults,size=30%\t0\t0\n"

			Expect(string(fstab)).To(Equal(expectedFstab))
		})

		It("Writes a simple fstab with overlay mode", func() {
			spec.Persistent.Mode = constants.OverlayMode
			Expect(utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)).To(Succeed())
			fstabData, err := action.InitialFstabData(runner, spec.Sysroot)
			Expect(err).To(BeNil())
			err = action.WriteFstab(cfg, spec, fstabData)
			Expect(err).To(BeNil())

			fstab, err := cfg.Config.Fs.ReadFile(filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(err).To(BeNil())
			expectedFstab := "/dev/loop0\t/\text2\tro,relatime\t0\t0\n"
			expectedFstab += "/dev/loop1\t/volume\text2\tro,relatime\t0\t0\n"
			expectedFstab += "/dev/loop2\t/run/elemental/extra\txfs\trw,relatime\t0\t0\n"
			expectedFstab += "/dev/sda4\t/run/initramfs/elemental-state\text2\tro,relatime\t0\t0\n"
			expectedFstab += "/dev/somedevice\t/run/elemental\tvfat\trw,defaults\t0\t0\n"
			expectedFstab += "/dev/persistentdev\t/run/elemental/persistent\tauto\tdefaults\t0\t0\n"
			expectedFstab += "overlay\t/some/path\toverlay\t"
			expectedFstab += "defaults,lowerdir=/some/path,upperdir=/run/elemental/persistent/.state/some-path.overlay/upper,workdir=/run/elemental/persistent/.state/some-path.overlay/work,x-systemd.requires-mounts-for=/run/elemental/persistent\t0\t0\n"
			expectedFstab += "tmpfs\t/run/elemental/overlay\ttmpfs\tdefaults,size=30%\t0\t0\n"

			Expect(string(fstab)).To(Equal(expectedFstab))
		})

		It("Does not write fstab if not requested", func() {
			spec := &types.MountSpec{
				WriteFstab: false,
				Ephemeral: types.EphemeralMounts{
					Size: "30%",
				},
			}
			utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)
			err := action.WriteFstab(cfg, spec, "")
			Expect(err).To(BeNil())

			ok, _ := utils.Exists(fs, filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(ok).To(BeFalse())
		})
	})
	Describe("Mount Volumes", func() {
		It("mounts expected volumes without errors", func() {
			spec.Volumes = append(spec.Volumes,
				&types.VolumeMount{
					Device:     "LABEL=TEST",
					Mountpoint: "/a/path",
				}, &types.VolumeMount{
					Device:     "PARTLABEL=partitionlabel",
					Mountpoint: "/a/different/path",
				}, &types.VolumeMount{
					Device:     "UUID=someuuidgoeshere",
					Mountpoint: "/a/path",
				},
			)
			Expect(action.MountVolumes(cfg, spec)).To(Succeed())
			list, _ := mounter.List()
			Expect(len(list)).To(Equal(5))
			// Note they were sorted according to the mountpoint
			Expect(list[0].Device).To(Equal("/dev/disk/by-partlabel/partitionlabel"))
			Expect(list[1].Path).To(Equal("/sysroot/a/path"))
			Expect(list[2].Device).To(Equal("/dev/disk/by-uuid/someuuidgoeshere"))
			Expect(list[3].Device).To(Equal("/dev/somedevice"))
			Expect(list[4].Device).To(Equal("/dev/persistentdev"))
		})
		It("fails to mount a volume", func() {
			mounter.ErrorOnMount = true
			Expect(action.MountVolumes(cfg, spec)).NotTo(Succeed())
		})
		It("fails to understand a non supported device reference", func() {
			spec.Volumes = append(spec.Volumes,
				&types.VolumeMount{
					Device:     "ThisIsNotADevice",
					Mountpoint: "/a/path",
				},
			)
			Expect(action.MountVolumes(cfg, spec)).NotTo(Succeed())
		})
	})
	Describe("Mounts ephemeral paths", func() {
		It("mounts tmpfs overlays paths without errors", func() {
			spec.Ephemeral.Paths = []string{"/etc"}
			Expect(action.MountEphemeral(cfg, spec.Sysroot, spec.Ephemeral)).To(Succeed())
			list, _ := mounter.List()
			Expect(list[0].Device).To(Equal("tmpfs"))
			Expect(list[1].Path).To(Equal("/sysroot/etc"))
			Expect(list[1].Device).To(Equal("overlay"))
		})
		It("mounts overlays paths on a block device without errors", func() {
			spec.Ephemeral.Paths = []string{"/etc"}
			spec.Ephemeral.Type = "block"
			spec.Ephemeral.Device = "/dev/some/device"
			Expect(action.MountEphemeral(cfg, spec.Sysroot, spec.Ephemeral)).To(Succeed())
			list, _ := mounter.List()
			Expect(list[0].Device).To(Equal("/dev/some/device"))
			Expect(list[1].Path).To(Equal("/sysroot/etc"))
			Expect(list[1].Device).To(Equal("overlay"))
		})
		It("fails to mount a volume", func() {
			mounter.ErrorOnMount = true
			Expect(action.MountEphemeral(cfg, spec.Sysroot, spec.Ephemeral)).NotTo(Succeed())
		})
		It("fails with invalid overlay type", func() {
			spec.Ephemeral.Type = "invalid"
			Expect(action.MountEphemeral(cfg, spec.Sysroot, spec.Ephemeral)).NotTo(Succeed())
		})
	})
	Describe("Mounts persistent paths", func() {
		It("mounts persistent binded paths without errors", func() {
			Expect(action.MountPersistent(cfg, spec)).To(Succeed())
			list, _ := mounter.List()
			Expect(list[0].Device).To(ContainSubstring("some-path.bind"))
			Expect(list[0].Path).To(ContainSubstring("/sysroot/some/path"))
		})
		It("mounts persistent overlay paths without errors", func() {
			spec.Persistent.Mode = constants.OverlayMode
			Expect(action.MountPersistent(cfg, spec)).To(Succeed())
			list, _ := mounter.List()
			Expect(list[0].Device).To(ContainSubstring("overlay"))
			Expect(list[0].Path).To(ContainSubstring("/sysroot/some/path"))
		})
		It("does nothing recovery mode", func() {
			spec.Mode = constants.RecoveryImgName
			Expect(action.MountPersistent(cfg, spec)).To(Succeed())
			list, _ := mounter.List()
			Expect(len(list)).To(Equal(0))
		})
		It("fails to mount a path", func() {
			mounter.ErrorOnMount = true
			Expect(action.MountPersistent(cfg, spec)).NotTo(Succeed())
		})
		It("fails to sync bind mounts", func() {
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				switch cmd {
				case "rsync":
					return []byte{}, fmt.Errorf("rsync error")
				default:
					return []byte{}, nil
				}
			}
			err := action.MountPersistent(cfg, spec)
			Expect(err.Error()).To(ContainSubstring("rsync error"))
		})
	})
	Describe("Runs selinux relabeling", func() {
		It("does not run if disabled in the spec", func() {
			spec.SelinuxRelabel = false

			err := action.SelinuxRelabel(cfg, spec)
			Expect(err).To(Succeed())

			exists, _ := utils.Exists(fs, constants.SELinuxRelabelDir)
			Expect(exists).To(BeFalse())
		})
		It("writes persistent and ephemeral dirs to /run/systemd/extra-relabel.d/elemental.layout", func() {
			spec.SelinuxRelabel = true

			err := action.SelinuxRelabel(cfg, spec)
			Expect(err).To(Succeed())

			exists, _ := utils.Exists(fs, constants.SELinuxRelabelDir)
			Expect(exists).To(BeTrue())

			data, err := fs.ReadFile(filepath.Join(constants.SELinuxRelabelDir, constants.SELinuxRelabelFile))
			Expect(err).To(Succeed())
			Expect(string(data)).To(Equal("/some/path"))
		})
		It("runs find with -exec setfiles in the new sysroot", func() {
			spec.SelinuxRelabel = true

			Expect(utils.MkdirAll(fs, spec.Sysroot, constants.DirPerm)).To(Succeed())
			Expect(utils.MkdirAll(fs, "/sbin", constants.DirPerm)).To(Succeed())
			Expect(utils.MkdirAll(fs, filepath.Dir(constants.SELinuxTargetedContextFile), constants.DirPerm)).To(Succeed())
			Expect(fs.WriteFile(constants.SELinuxTargetedContextFile, []byte("/.*"), constants.FilePerm)).To(Succeed())
			Expect(fs.WriteFile("/sbin/setfiles", []byte("#!/bin/bash"), 0755)).To(Succeed())

			findCnt := 0
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				switch cmd {
				case "find":
					findCnt += 1
					Expect(args).To(ContainElement("/some/path"))
					Expect(args).To(ContainElement("-depth"))
					Expect(args).To(ContainElement("-exec"))
					Expect(args).To(ContainElement("setfiles"))
					return []byte{}, nil
				default:
					return []byte{}, nil
				}
			}

			err := action.SelinuxRelabel(cfg, spec)
			Expect(err).To(Succeed())

			Expect(findCnt).To(Equal(1))
			Expect(syscall.WasChrootCalledWith(spec.Sysroot)).To(BeTrue())
		})
	})
})
