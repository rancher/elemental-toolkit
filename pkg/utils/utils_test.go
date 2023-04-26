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

package utils_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	eleefi "github.com/rancher/elemental-cli/pkg/efi"

	efi "github.com/canonical/go-efilib"
	"github.com/jaypipes/ghw/pkg/block"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	conf "github.com/rancher/elemental-cli/pkg/config"
	"github.com/rancher/elemental-cli/pkg/constants"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
	v1mock "github.com/rancher/elemental-cli/tests/mocks"
)

func getNamesFromListFiles(list []os.FileInfo) []string {
	var names []string
	for _, f := range list {
		names = append(names, f.Name())
	}
	return names
}

var _ = Describe("Utils", Label("utils"), func() {
	var config *v1.Config
	var runner *v1mock.FakeRunner
	var logger v1.Logger
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var mounter *v1mock.ErrorMounter
	var extractor *v1mock.FakeImageExtractor
	var fs vfs.FS
	var cleanup func()

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHTTPClient{}
		logger = v1.NewNullLogger()
		extractor = v1mock.NewFakeImageExtractor(logger)
		// Ensure /tmp exists in the VFS
		fs, cleanup, _ = vfst.NewTestFS(nil)
		fs.Mkdir("/tmp", constants.DirPerm)
		fs.Mkdir("/run", constants.DirPerm)
		fs.Mkdir("/etc", constants.DirPerm)

		config = conf.NewConfig(
			conf.WithFs(fs),
			conf.WithRunner(runner),
			conf.WithLogger(logger),
			conf.WithMounter(mounter),
			conf.WithSyscall(syscall),
			conf.WithClient(client),
			conf.WithImageExtractor(extractor),
		)
	})
	AfterEach(func() { cleanup() })

	Describe("Chroot", Label("chroot"), func() {
		var chroot *utils.Chroot
		BeforeEach(func() {
			chroot = utils.NewChroot(
				"/whatever",
				config,
			)
		})
		Describe("ChrootedCallback method", func() {
			It("runs a callback in a chroot", func() {
				err := utils.ChrootedCallback(config, "/somepath", map[string]string{}, func() error {
					return nil
				})
				Expect(err).ShouldNot(HaveOccurred())
				err = utils.ChrootedCallback(config, "/somepath", map[string]string{}, func() error {
					return fmt.Errorf("callback error")
				})
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("callback error"))
			})
		})
		Describe("on success", func() {
			It("command should be called in the chroot", func() {
				_, err := chroot.Run("chroot-command")
				Expect(err).To(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
			})
			It("commands should be called with a customized chroot", func() {
				chroot.SetExtraMounts(map[string]string{"/real/path": "/in/chroot/path"})
				Expect(chroot.Prepare()).To(BeNil())
				defer chroot.Close()
				_, err := chroot.Run("chroot-command")
				Expect(err).To(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
				_, err = chroot.Run("chroot-another-command")
				Expect(err).To(BeNil())
			})
			It("runs a callback in a custom chroot", func() {
				called := false
				callback := func() error {
					called = true
					return nil
				}
				err := chroot.RunCallback(callback)
				Expect(err).To(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
				Expect(called).To(BeTrue())
			})
		})
		Describe("on failure", func() {
			It("should return error if chroot-command fails", func() {
				runner.ReturnError = errors.New("run error")
				_, err := chroot.Run("chroot-command")
				Expect(err).NotTo(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
			})
			It("should return error if callback fails", func() {
				called := false
				callback := func() error {
					called = true
					return errors.New("Callback error")
				}
				err := chroot.RunCallback(callback)
				Expect(err).NotTo(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
				Expect(called).To(BeTrue())
			})
			It("should return error if preparing twice before closing", func() {
				Expect(chroot.Prepare()).To(BeNil())
				defer chroot.Close()
				Expect(chroot.Prepare()).NotTo(BeNil())
				Expect(chroot.Close()).To(BeNil())
				Expect(chroot.Prepare()).To(BeNil())
			})
			It("should return error if failed to chroot", func() {
				syscall.ErrorOnChroot = true
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("chroot error"))
			})
			It("should return error if failed to mount on prepare", Label("mount"), func() {
				mounter.ErrorOnMount = true
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mount error"))
			})
			It("should return error if failed to unmount on close", Label("unmount"), func() {
				mounter.ErrorOnUnmount = true
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed closing chroot"))
			})
		})
	})
	Describe("TestBootedFrom", Label("BootedFrom"), func() {
		It("returns true if we are booting from label FAKELABEL", func() {
			runner.ReturnValue = []byte("")
			Expect(utils.BootedFrom(runner, "FAKELABEL")).To(BeFalse())
		})
		It("returns false if we are not booting from label FAKELABEL", func() {
			runner.ReturnValue = []byte("FAKELABEL")
			Expect(utils.BootedFrom(runner, "FAKELABEL")).To(BeTrue())
		})
	})
	Describe("GetDeviceByLabel", Label("lsblk", "partitions"), func() {
		var cmds [][]string
		BeforeEach(func() {
			cmds = [][]string{
				{"udevadm", "settle"},
			}
		})
		It("returns found device", func() {
			ghwTest := v1mock.GhwMock{}
			disk := block.Disk{Name: "device", Partitions: []*block.Partition{
				{
					Name:            "device1",
					FilesystemLabel: "FAKE",
				},
			}}
			ghwTest.AddDisk(disk)
			ghwTest.CreateDevices()
			defer ghwTest.Clean()
			out, err := utils.GetDeviceByLabel(runner, "FAKE", 1)
			Expect(err).To(BeNil())
			Expect(out).To(Equal("/dev/device1"))
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("fails if no device is found in two attempts", func() {
			_, err := utils.GetDeviceByLabel(runner, "FAKE", 2)
			Expect(err).NotTo(BeNil())
			Expect(runner.CmdsMatch(append(cmds, cmds...))).To(BeNil())
		})
	})
	Describe("GetAllPartitions", Label("lsblk", "partitions"), func() {
		var ghwTest v1mock.GhwMock
		BeforeEach(func() {
			ghwTest = v1mock.GhwMock{}
			disk1 := block.Disk{
				Name: "sda",
				Partitions: []*block.Partition{
					{
						Name: "sda1Test",
					},
					{
						Name: "sda2Test",
					},
				},
			}
			disk2 := block.Disk{
				Name: "sdb",
				Partitions: []*block.Partition{
					{
						Name: "sdb1Test",
					},
				},
			}
			ghwTest.AddDisk(disk1)
			ghwTest.AddDisk(disk2)
			ghwTest.CreateDevices()
		})
		AfterEach(func() {
			ghwTest.Clean()
		})
		It("returns all found partitions", func() {
			parts, err := utils.GetAllPartitions()
			Expect(err).To(BeNil())
			var devices []string
			for _, p := range parts {
				devices = append(devices, p.Path)
			}
			Expect(devices).To(ContainElement(ContainSubstring("sda1Test")))
			Expect(devices).To(ContainElement(ContainSubstring("sda2Test")))
			Expect(devices).To(ContainElement(ContainSubstring("sdb1Test")))
		})
	})
	Describe("GetPartitionFS", Label("lsblk", "partitions"), func() {
		var ghwTest v1mock.GhwMock
		BeforeEach(func() {
			ghwTest = v1mock.GhwMock{}
			disk := block.Disk{Name: "device", Partitions: []*block.Partition{
				{
					Name: "device1",
					Type: "xfs",
				},
				{
					Name: "device2",
				},
			}}
			ghwTest.AddDisk(disk)
			ghwTest.CreateDevices()
		})
		AfterEach(func() {
			ghwTest.Clean()
		})
		It("returns found device with plain partition device", func() {
			pFS, err := utils.GetPartitionFS("device1")
			Expect(err).To(BeNil())
			Expect(pFS).To(Equal("xfs"))
		})
		It("returns found device with full partition device", func() {
			pFS, err := utils.GetPartitionFS("/dev/device1")
			Expect(err).To(BeNil())
			Expect(pFS).To(Equal("xfs"))
		})
		It("fails if no partition is found", func() {
			_, err := utils.GetPartitionFS("device2")
			Expect(err).NotTo(BeNil())
		})
	})
	Describe("CosignVerify", Label("cosign"), func() {
		It("runs a keyless verification", func() {
			_, err := utils.CosignVerify(fs, runner, "some/image:latest", "", true)
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch([][]string{{"cosign", "-d=true", "some/image:latest"}})).To(BeNil())
		})
		It("runs a verification using a public key", func() {
			_, err := utils.CosignVerify(fs, runner, "some/image:latest", "https://mykey.pub", false)
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(
				[][]string{{"cosign", "-key", "https://mykey.pub", "some/image:latest"}},
			)).To(BeNil())
		})
		It("Fails to to create temporary directories", func() {
			_, err := utils.CosignVerify(vfs.NewReadOnlyFS(fs), runner, "some/image:latest", "", true)
			Expect(err).NotTo(BeNil())
		})
	})
	Describe("Reboot and shutdown", Label("reboot", "shutdown"), func() {
		It("reboots", func() {
			start := time.Now()
			utils.Reboot(runner, 2)
			duration := time.Since(start)
			Expect(runner.CmdsMatch([][]string{{"reboot", "-f"}})).To(BeNil())
			Expect(duration.Seconds() >= 2).To(BeTrue())
		})
		It("shuts down", func() {
			start := time.Now()
			utils.Shutdown(runner, 3)
			duration := time.Since(start)
			Expect(runner.CmdsMatch([][]string{{"poweroff", "-f"}})).To(BeNil())
			Expect(duration.Seconds() >= 3).To(BeTrue())
		})
	})
	Describe("GetFullDeviceByLabel", Label("lsblk", "partitions"), func() {
		var cmds [][]string
		BeforeEach(func() {
			cmds = [][]string{
				{"udevadm", "settle"},
			}
		})
		It("returns found v1.Partition", func() {
			var flags []string
			ghwTest := v1mock.GhwMock{}
			disk := block.Disk{Name: "device", Partitions: []*block.Partition{
				{
					Name:            "device1",
					FilesystemLabel: "FAKE",
					Type:            "fakefs",
					MountPoint:      "/mnt/fake",
					SizeBytes:       0,
				},
			}}
			ghwTest.AddDisk(disk)
			ghwTest.CreateDevices()
			defer ghwTest.Clean()
			out, err := utils.GetFullDeviceByLabel(runner, "FAKE", 1)
			Expect(err).To(BeNil())
			Expect(out.FilesystemLabel).To(Equal("FAKE"))
			Expect(out.Size).To(Equal(uint(0)))
			Expect(out.FS).To(Equal("fakefs"))
			Expect(out.MountPoint).To(Equal("/mnt/fake"))
			Expect(out.Flags).To(Equal(flags))
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("fails to run lsblk", func() {
			runner.ReturnError = errors.New("failed running lsblk")
			_, err := utils.GetFullDeviceByLabel(runner, "FAKE", 1)
			Expect(err).To(HaveOccurred())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("fails to parse json output", func() {
			runner.ReturnValue = []byte(`{"invalidobject": []}`)
			_, err := utils.GetFullDeviceByLabel(runner, "FAKE", 1)
			Expect(err).To(HaveOccurred())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("fails if no device is found in two attempts", func() {
			runner.ReturnValue = []byte(`{"blockdevices":[{"label":"something","type": "part"}]}`)
			_, err := utils.GetFullDeviceByLabel(runner, "FAKE", 2)
			Expect(err).To(HaveOccurred())
			Expect(runner.CmdsMatch(append(cmds, cmds...))).To(BeNil())
		})
	})
	Describe("CopyFile", Label("CopyFile"), func() {
		It("Copies source file to target file", func() {
			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create("/some/file")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Stat("/some/otherfile")
			Expect(err).Should(HaveOccurred())
			Expect(utils.CopyFile(fs, "/some/file", "/some/otherfile")).ShouldNot(HaveOccurred())
			e, err := utils.Exists(fs, "/some/otherfile")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(e).To(BeTrue())
		})
		It("Copies source file to target folder", func() {
			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			err = utils.MkdirAll(fs, "/someotherfolder", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create("/some/file")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Stat("/someotherfolder/file")
			Expect(err).Should(HaveOccurred())
			Expect(utils.CopyFile(fs, "/some/file", "/someotherfolder")).ShouldNot(HaveOccurred())
			e, err := utils.Exists(fs, "/someotherfolder/file")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(e).To(BeTrue())
		})
		It("Fails to open non existing file", func() {
			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(utils.CopyFile(fs, "/some/file", "/some/otherfile")).NotTo(BeNil())
			_, err = fs.Stat("/some/otherfile")
			Expect(err).NotTo(BeNil())
		})
		It("Fails to copy on non writable target", func() {
			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			fs.Create("/some/file")
			_, err = fs.Stat("/some/otherfile")
			Expect(err).NotTo(BeNil())
			fs = vfs.NewReadOnlyFS(fs)
			Expect(utils.CopyFile(fs, "/some/file", "/some/otherfile")).NotTo(BeNil())
			_, err = fs.Stat("/some/otherfile")
			Expect(err).NotTo(BeNil())
		})
	})
	Describe("CreateDirStructure", Label("CreateDirStructure"), func() {
		It("Creates essential directories", func() {
			dirList := []string{"sys", "proc", "dev", "tmp", "boot", "usr/local", "oem"}
			for _, dir := range dirList {
				_, err := fs.Stat(fmt.Sprintf("/my/root/%s", dir))
				Expect(err).NotTo(BeNil())
			}
			Expect(utils.CreateDirStructure(fs, "/my/root")).To(BeNil())
			for _, dir := range dirList {
				fi, err := fs.Stat(fmt.Sprintf("/my/root/%s", dir))
				Expect(err).To(BeNil())
				if fi.Name() == "tmp" {
					Expect(fmt.Sprintf("%04o", fi.Mode().Perm())).To(Equal("0777"))
					Expect(fi.Mode() & os.ModeSticky).NotTo(Equal(0))
				}
				if fi.Name() == "sys" {
					Expect(fmt.Sprintf("%04o", fi.Mode().Perm())).To(Equal("0555"))
				}
			}
		})
		It("Fails on non writable target", func() {
			fs = vfs.NewReadOnlyFS(fs)
			Expect(utils.CreateDirStructure(fs, "/my/root")).NotTo(BeNil())
		})
	})
	Describe("SyncData", Label("SyncData"), func() {
		It("Copies all files from source to target", func() {
			sourceDir, err := utils.TempDir(fs, "", "elementalsource")
			Expect(err).ShouldNot(HaveOccurred())
			destDir, err := utils.TempDir(fs, "", "elementaltarget")
			Expect(err).ShouldNot(HaveOccurred())

			for i := 0; i < 5; i++ {
				_, _ = utils.TempFile(fs, sourceDir, "file*")
			}

			Expect(utils.SyncData(logger, fs, sourceDir, destDir)).To(BeNil())

			filesDest, err := fs.ReadDir(destDir)
			Expect(err).To(BeNil())

			destNames := getNamesFromListFiles(filesDest)
			filesSource, err := fs.ReadDir(sourceDir)
			Expect(err).To(BeNil())

			SourceNames := getNamesFromListFiles(filesSource)

			// Should be the same files in both dirs now
			Expect(destNames).To(Equal(SourceNames))
		})

		It("Copies all files from source to target respecting excludes", func() {
			sourceDir, err := utils.TempDir(fs, "", "elementalsource")
			Expect(err).ShouldNot(HaveOccurred())
			destDir, err := utils.TempDir(fs, "", "elementaltarget")
			Expect(err).ShouldNot(HaveOccurred())

			utils.MkdirAll(fs, filepath.Join(sourceDir, "host"), constants.DirPerm)
			utils.MkdirAll(fs, filepath.Join(sourceDir, "run"), constants.DirPerm)

			// /tmp/run would be excluded as well, as we define an exclude without the "/" prefix
			utils.MkdirAll(fs, filepath.Join(sourceDir, "tmp", "run"), constants.DirPerm)

			for i := 0; i < 5; i++ {
				_, _ = utils.TempFile(fs, sourceDir, "file*")
			}

			Expect(utils.SyncData(logger, fs, sourceDir, destDir, "host", "run")).To(BeNil())

			filesDest, err := fs.ReadDir(destDir)
			Expect(err).To(BeNil())

			destNames := getNamesFromListFiles(filesDest)

			filesSource, err := fs.ReadDir(sourceDir)
			Expect(err).To(BeNil())

			SourceNames := getNamesFromListFiles(filesSource)

			// Shouldn't be the same
			Expect(destNames).ToNot(Equal(SourceNames))
			expected := []string{}

			for _, s := range SourceNames {
				if s != "host" && s != "run" {
					expected = append(expected, s)
				}
			}
			Expect(destNames).To(Equal(expected))

			// /tmp/run is not copied over
			Expect(utils.Exists(fs, filepath.Join(destDir, "tmp", "run"))).To(BeFalse())
		})

		It("Copies all files from source to target respecting excludes with '/' prefix", func() {
			sourceDir, err := utils.TempDir(fs, "", "elementalsource")
			Expect(err).ShouldNot(HaveOccurred())
			destDir, err := utils.TempDir(fs, "", "elementaltarget")
			Expect(err).ShouldNot(HaveOccurred())

			utils.MkdirAll(fs, filepath.Join(sourceDir, "host"), constants.DirPerm)
			utils.MkdirAll(fs, filepath.Join(sourceDir, "run"), constants.DirPerm)
			utils.MkdirAll(fs, filepath.Join(sourceDir, "var", "run"), constants.DirPerm)
			utils.MkdirAll(fs, filepath.Join(sourceDir, "tmp", "host"), constants.DirPerm)

			Expect(utils.SyncData(logger, fs, sourceDir, destDir, "/host", "/run")).To(BeNil())

			filesDest, err := fs.ReadDir(destDir)
			Expect(err).To(BeNil())

			destNames := getNamesFromListFiles(filesDest)

			filesSource, err := fs.ReadDir(sourceDir)
			Expect(err).To(BeNil())

			SourceNames := getNamesFromListFiles(filesSource)

			// Shouldn't be the same
			Expect(destNames).ToNot(Equal(SourceNames))

			Expect(utils.Exists(fs, filepath.Join(destDir, "var", "run"))).To(BeTrue())
			Expect(utils.Exists(fs, filepath.Join(destDir, "tmp", "host"))).To(BeTrue())
			Expect(utils.Exists(fs, filepath.Join(destDir, "host"))).To(BeFalse())
			Expect(utils.Exists(fs, filepath.Join(destDir, "run"))).To(BeFalse())
		})

		It("should not fail if dirs are empty", func() {
			sourceDir, err := utils.TempDir(fs, "", "elementalsource")
			Expect(err).ShouldNot(HaveOccurred())
			destDir, err := utils.TempDir(fs, "", "elementaltarget")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(utils.SyncData(logger, fs, sourceDir, destDir)).To(BeNil())
		})
		It("should fail if destination does not exist", func() {
			sourceDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(sourceDir)
			Expect(utils.SyncData(logger, nil, sourceDir, "/welp")).NotTo(BeNil())
		})
		It("should fail if source does not exist", func() {
			destDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(destDir)
			Expect(utils.SyncData(logger, nil, "/welp", destDir)).NotTo(BeNil())
		})
	})
	Describe("IsLocalURI", Label("uri"), func() {
		It("Detects a local url", func() {
			local, err := utils.IsLocalURI("file://some/path")
			Expect(err).To(BeNil())
			Expect(local).To(BeTrue())
		})
		It("Detects a local path", func() {
			local, err := utils.IsLocalURI("/some/path")
			Expect(err).To(BeNil())
			Expect(local).To(BeTrue())
		})
		It("Detects a remote uri", func() {
			local, err := utils.IsLocalURI("http://something.org")
			Expect(err).To(BeNil())
			Expect(local).To(BeFalse())
		})
		It("Detects a remote uri", func() {
			local, err := utils.IsLocalURI("some.domain.org:33/some/path")
			Expect(err).To(BeNil())
			Expect(local).To(BeFalse())
			local, err = utils.IsLocalURI("some.domain.org/some/path:latest")
			Expect(err).To(BeNil())
			Expect(local).To(BeFalse())
		})
		It("Fails on invalid URL", func() {
			local, err := utils.IsLocalURI("$htt:|//insane.stuff")
			Expect(err).NotTo(BeNil())
			Expect(local).To(BeFalse())
		})
	})
	Describe("IsHTTPURI", Label("uri"), func() {
		It("Detects a http url", func() {
			local, err := utils.IsHTTPURI("http://domain.org/path")
			Expect(err).To(BeNil())
			Expect(local).To(BeTrue())
		})
		It("Detects a https url", func() {
			local, err := utils.IsHTTPURI("https://domain.org/path")
			Expect(err).To(BeNil())
			Expect(local).To(BeTrue())
		})
		It("Detects it is a non http URL", func() {
			local, err := utils.IsHTTPURI("file://path")
			Expect(err).To(BeNil())
			Expect(local).To(BeFalse())
			local, err = utils.IsHTTPURI("container.reg.org:1024/some/repository")
			Expect(err).To(BeNil())
			Expect(local).To(BeFalse())
		})
		It("Fails on invalid URL", func() {
			local, err := utils.IsLocalURI("$htt:|//insane.stuff")
			Expect(err).NotTo(BeNil())
			Expect(local).To(BeFalse())
		})
	})
	Describe("GetSource", Label("GetSource"), func() {
		It("Fails on invalid url", func() {
			Expect(utils.GetSource(config, "$htt:|//insane.stuff", "/tmp/dest")).NotTo(BeNil())
		})
		It("Fails on readonly destination", func() {
			config.Fs = vfs.NewReadOnlyFS(fs)
			Expect(utils.GetSource(config, "http://something.org", "/tmp/dest")).NotTo(BeNil())
		})
		It("Fails on non existing local source", func() {
			Expect(utils.GetSource(config, "/some/missing/file", "/tmp/dest")).NotTo(BeNil())
		})
		It("Fails on http client error", func() {
			client.Error = true
			url := "https://missing.io"
			Expect(utils.GetSource(config, url, "/tmp/dest")).NotTo(BeNil())
			client.WasGetCalledWith(url)
		})
		It("Copies local file to destination", func() {
			fs.Create("/tmp/file")
			Expect(utils.GetSource(config, "file:///tmp/file", "/tmp/dest")).To(BeNil())
			_, err := fs.Stat("/tmp/dest")
			Expect(err).To(BeNil())
		})
	})
	Describe("ValidContainerReference", Label("reference"), func() {
		It("Returns true on valid references", func() {
			Expect(utils.ValidContainerReference("opensuse/leap:15.3")).To(BeTrue())
			Expect(utils.ValidContainerReference("opensuse")).To(BeTrue())
			Expect(utils.ValidContainerReference("registry.suse.com/opensuse/something")).To(BeTrue())
			Expect(utils.ValidContainerReference("registry.suse.com:8080/something:253")).To(BeTrue())
		})
		It("Returns false on invalid references", func() {
			Expect(utils.ValidContainerReference("opensuse/leap:15+3")).To(BeFalse())
			Expect(utils.ValidContainerReference("opensusE")).To(BeFalse())
			Expect(utils.ValidContainerReference("registry.suse.com:8080/Something:253")).To(BeFalse())
			Expect(utils.ValidContainerReference("http://registry.suse.com:8080/something:253")).To(BeFalse())
		})
	})
	Describe("ValidTaggedContainerReference", Label("reference"), func() {
		It("Returns true on valid references including explicit tag", func() {
			Expect(utils.ValidTaggedContainerReference("opensuse/leap:15.3")).To(BeTrue())
			Expect(utils.ValidTaggedContainerReference("registry.suse.com/opensuse/something:latest")).To(BeTrue())
			Expect(utils.ValidTaggedContainerReference("registry.suse.com:8080/something:253")).To(BeTrue())
		})
		It("Returns false on valid references without explicit tag", func() {
			Expect(utils.ValidTaggedContainerReference("opensuse")).To(BeFalse())
			Expect(utils.ValidTaggedContainerReference("registry.suse.com/opensuse/something")).To(BeFalse())
			Expect(utils.ValidTaggedContainerReference("registry.suse.com:8080/something")).To(BeFalse())
		})
	})
	Describe("DirSize", Label("fs"), func() {
		BeforeEach(func() {
			err := utils.MkdirAll(fs, "/folder/subfolder", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			f, err := fs.Create("/folder/file")
			Expect(err).ShouldNot(HaveOccurred())
			err = f.Truncate(1024)
			Expect(err).ShouldNot(HaveOccurred())
			f, err = fs.Create("/folder/subfolder/file")
			Expect(err).ShouldNot(HaveOccurred())
			err = f.Truncate(2048)
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Returns the expected size of a test folder", func() {
			size, err := utils.DirSize(fs, "/folder")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To(Equal(int64(3072)))
		})
		It("Returns the expected size of a test folder", func() {
			err := fs.Chmod("/folder/subfolder", 0600)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = utils.DirSize(fs, "/folder")
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("FindFileWithPrefix", Label("find"), func() {
		BeforeEach(func() {
			err := utils.MkdirAll(fs, "/path/inner", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = fs.Create("/path/onefile")
			Expect(err).ShouldNot(HaveOccurred())

			_, err = fs.Create("/path/somefile")
			Expect(err).ShouldNot(HaveOccurred())

			err = fs.Symlink("onefile", "/path/linkedfile")
			Expect(err).ShouldNot(HaveOccurred())

			err = fs.Symlink("/path/onefile", "/path/abslinkedfile")
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("finds a matching file", func() {
			f, err := utils.FindFileWithPrefix(fs, "/path", "prefix", "some")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(f).To(Equal("/path/somefile"))
		})
		It("finds a matching file, but returns the target of a relative symlink", func() {
			// apparently fs.Readlink returns the raw path so we need to
			// use raw paths here. This is an arguable behavior
			rawPath, err := fs.RawPath("/path")
			Expect(err).ShouldNot(HaveOccurred())

			f, err := utils.FindFileWithPrefix(vfs.OSFS, rawPath, "linked")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(err).ShouldNot(HaveOccurred())
			Expect(f).To(Equal(filepath.Join(rawPath, "onefile")))
		})
		It("finds a matching file, but returns the target of an absolute symlink", func() {
			// apparently fs.Readlink returns the raw path so we need to
			// use raw paths here. This is an arguable behavior
			rawPath, err := fs.RawPath("/path")
			Expect(err).ShouldNot(HaveOccurred())

			f, err := utils.FindFileWithPrefix(vfs.OSFS, rawPath, "abslinked")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(err).ShouldNot(HaveOccurred())
			Expect(f).To(Equal(filepath.Join(rawPath, "onefile")))
		})
		It("fails to read given path", func() {
			_, err := utils.FindFileWithPrefix(fs, "nonexisting", "some")
			Expect(err).Should(HaveOccurred())
		})
		It("doesn't find any matching file in path", func() {
			utils.MkdirAll(fs, "/path", constants.DirPerm)
			_, err := utils.FindFileWithPrefix(fs, "/path", "prefix", "anotherprefix")
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("CalcFileChecksum", Label("checksum"), func() {
		It("compute correct sha256 checksum", func() {
			testData := strings.Repeat("abcdefghilmnopqrstuvz\n", 20)
			testDataSHA256 := "7f182529f6362ae9cfa952ab87342a7180db45d2c57b52b50a68b6130b15a422"

			err := fs.Mkdir("/iso", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			err = fs.WriteFile("/iso/test.iso", []byte(testData), 0644)
			Expect(err).ShouldNot(HaveOccurred())

			checksum, err := utils.CalcFileChecksum(fs, "/iso/test.iso")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(checksum).To(Equal(testDataSHA256))
		})
	})
	Describe("Grub", Label("grub"), func() {
		Describe("Install", func() {
			var target, rootDir, bootDir string
			var buf *bytes.Buffer
			BeforeEach(func() {
				target = "/dev/test"
				rootDir = constants.ActiveDir
				bootDir = constants.StateDir
				buf = &bytes.Buffer{}
				logger = v1.NewBufferLogger(buf)
				logger.SetLevel(v1.DebugLevel())
				config.Logger = logger

				err := utils.MkdirAll(fs, filepath.Join(bootDir, "grub2"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())

				err = utils.MkdirAll(fs, filepath.Dir(filepath.Join(rootDir, constants.GrubConf)), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())

				err = fs.WriteFile(filepath.Join(rootDir, constants.GrubConf), []byte("console=tty1"), 0644)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("installs with default values", func() {
				grub := utils.NewGrub(config)
				err := grub.Install(target, rootDir, bootDir, constants.GrubConf, false, "", true, false)
				Expect(err).To(BeNil())

				Expect(buf).To(ContainSubstring("Installing GRUB.."))
				Expect(buf).To(ContainSubstring("Grub install to device /dev/test complete"))
				Expect(buf).ToNot(ContainSubstring("efi"))
				Expect(buf.String()).ToNot(ContainSubstring("Adding extra tty (serial) to grub.cfg"))
				targetGrub, err := fs.ReadFile(fmt.Sprintf("%s/grub2/grub.cfg", bootDir))
				Expect(err).To(BeNil())
				// Should not be modified at all
				Expect(targetGrub).To(ContainSubstring("console=tty1"))

			})
			It("installs with efi firmware", Label("efi"), func() {
				err := utils.MkdirAll(fs, filepath.Join(rootDir, "/usr/share/efi/x86_64/"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/usr/share/efi/x86_64/shim.efi"), []byte(""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/usr/share/efi/x86_64/MokManager.efi"), []byte(""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/usr/share/efi/x86_64/grub.efi"), []byte(""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = utils.MkdirAll(fs, filepath.Join(rootDir, "/x86_64/"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/x86_64/loopback.mod"), []byte(""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = utils.MkdirAll(fs, filepath.Join(rootDir, "/etc/"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("ID=\"suse\""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				grub := utils.NewGrub(config)
				err = grub.Install(target, rootDir, bootDir, constants.GrubConf, true, "", true, false)
				Expect(err).ShouldNot(HaveOccurred())

				// Check everything was copied
				_, err = fs.ReadFile(fmt.Sprintf("%s/grub2/grub.cfg", bootDir))
				Expect(err).To(BeNil())
				_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI"))
				Expect(err).To(BeNil())
				_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/boot"))
				Expect(err).To(BeNil())
				_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/boot/shim.efi"))
				Expect(err).To(BeNil())
				_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/boot/MokManager.efi"))
				Expect(err).To(BeNil())
				_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/boot/grub.efi"))
				Expect(err).To(BeNil())
				_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/boot/bootx64.efi"))
				Expect(err).To(BeNil())

			})
			It("fails with efi if no modules files exist", Label("efi"), func() {
				grub := utils.NewGrub(config)
				err := grub.Install(target, rootDir, bootDir, constants.GrubConf, true, "", true, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("grub"))
				Expect(err.Error()).To(ContainSubstring("modules"))
			})
			It("fails with efi if no os-release file exist", Label("efi"), func() {
				err := utils.MkdirAll(fs, filepath.Join(rootDir, "/x86_64/"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/x86_64/loopback.mod"), []byte(""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				grub := utils.NewGrub(config)
				err = grub.Install(target, rootDir, bootDir, constants.GrubConf, true, "", true, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("os-release"))
			})
			It("fails with efi if no grub files exist", Label("efi"), func() {
				err := utils.MkdirAll(fs, filepath.Join(rootDir, "/x86_64/"), constants.DirPerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/x86_64/loopback.mod"), []byte(""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				err = fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("ID=\"suse\""), constants.FilePerm)
				Expect(err).ShouldNot(HaveOccurred())
				grub := utils.NewGrub(config)
				err = grub.Install(target, rootDir, bootDir, constants.GrubConf, true, "", true, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("efi"))
				Expect(err.Error()).To(ContainSubstring("artifacts"))
			})
			It("Fails if it can't read grub config file", func() {
				err := fs.RemoveAll(filepath.Join(rootDir, constants.GrubConf))
				Expect(err).ShouldNot(HaveOccurred())
				grub := utils.NewGrub(config)
				Expect(grub.Install(target, rootDir, bootDir, constants.GrubConf, false, "", true, false)).NotTo(BeNil())

				Expect(buf).To(ContainSubstring("Failed copying grub config file"))
			})
		})
		Describe("SetPersistentVariables", func() {
			It("Sets the grub environment file", func() {
				grub := utils.NewGrub(config)
				Expect(grub.SetPersistentVariables(
					"somefile", map[string]string{"key1": "value1", "key2": "value2"},
				)).To(BeNil())
				Expect(runner.IncludesCmds([][]string{
					{"grub2-editenv", "somefile", "set", "key1=value1"},
					{"grub2-editenv", "somefile", "set", "key2=value2"},
				})).To(BeNil())
			})
			It("Fails running grub2-editenv", func() {
				runner.ReturnError = errors.New("grub error")
				grub := utils.NewGrub(config)
				Expect(grub.SetPersistentVariables(
					"somefile", map[string]string{"key1": "value1"},
				)).NotTo(BeNil())
				Expect(runner.CmdsMatch([][]string{
					{"grub2-editenv", "somefile", "set", "key1=value1"},
				})).To(BeNil())
			})
		})
		Describe("CreateBootEntry", Label("bootentry"), func() {
			var efivars eleefi.Variables
			var relativeTo string

			BeforeEach(func() {
				efivars = &eleefi.MockEFIVariables{}
				err := fs.Mkdir("/EFI", constants.DirPerm)
				Expect(err).ToNot(HaveOccurred())
				err = fs.WriteFile("/EFI/test.efi", []byte(""), constants.FilePerm)
				Expect(err).ToNot(HaveOccurred())
				relativeTo, _ = fs.RawPath("/EFI")

			})
			It("Sets the proper entry", func() {
				// We need to pass the relative path because bootmanager works on real paths
				grub := utils.NewGrub(config)
				err := grub.CreateBootEntry("test.efi", relativeTo, efivars)
				Expect(err).ToNot(HaveOccurred())
				vars, _ := efivars.ListVariables()
				// Only one entry should have been created
				// Second one is the BootOrder!
				Expect(len(vars)).To(Equal(2))
				// Load the options and check that its correct
				variable, _, err := efivars.GetVariable(vars[0].GUID, "Boot0000")
				option, err := efi.ReadLoadOption(bytes.NewReader(variable))
				Expect(err).ToNot(HaveOccurred())
				Expect(option.Description).To(Equal("elemental-shim"))
				Expect(option.FilePath).To(ContainSubstring("test.efi"))
				Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test.efi`))
			})
			It("Does not duplicate if an entry exists", func() {
				// We need to pass the relative path because bootmanager works on real paths
				grub := utils.NewGrub(config)
				err := grub.CreateBootEntry("test.efi", relativeTo, efivars)
				Expect(err).ToNot(HaveOccurred())
				vars, _ := efivars.ListVariables()
				// Only one entry should have been created
				// Second one is the BootOrder!
				Expect(len(vars)).To(Equal(2))
				// Load the options and check that its correct
				variable, _, err := efivars.GetVariable(vars[0].GUID, "Boot0000")
				option, err := efi.ReadLoadOption(bytes.NewReader(variable))
				Expect(err).ToNot(HaveOccurred())
				Expect(option.Description).To(Equal("elemental-shim"))
				Expect(option.FilePath).To(ContainSubstring("test.efi"))
				Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test.efi`))
				// And here we go again
				err = grub.CreateBootEntry("test.efi", relativeTo, efivars)
				// Reload vars!
				vars, _ = efivars.ListVariables()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(vars)).To(Equal(2))
			})
			It("Creates a new one if the path changes", func() {
				err := fs.WriteFile("/EFI/test1.efi", []byte(""), constants.FilePerm)
				Expect(err).ToNot(HaveOccurred())
				// We need to pass the relative path because bootmanager works on real paths
				grub := utils.NewGrub(config)
				err = grub.CreateBootEntry("test.efi", relativeTo, efivars)
				Expect(err).ToNot(HaveOccurred())
				vars, _ := efivars.ListVariables()
				// Only one entry should have been created
				// Second one is the BootOrder!
				Expect(len(vars)).To(Equal(2))
				// Load the options and check that its correct
				variable, _, err := efivars.GetVariable(vars[0].GUID, "Boot0000")
				option, err := efi.ReadLoadOption(bytes.NewReader(variable))
				Expect(err).ToNot(HaveOccurred())
				Expect(option.Description).To(Equal("elemental-shim"))
				Expect(option.FilePath).To(ContainSubstring("test.efi"))
				Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test.efi`))

				// And here we go again
				err = grub.CreateBootEntry("test1.efi", relativeTo, efivars)
				Expect(err).ToNot(HaveOccurred())
				// Reload vars!
				vars, _ = efivars.ListVariables()
				Expect(len(vars)).To(Equal(3))
				// As this is the second entry generated its name is Boot0001
				variable, _, err = efivars.GetVariable(vars[0].GUID, "Boot0001")
				option, err = efi.ReadLoadOption(bytes.NewReader(variable))
				Expect(err).ToNot(HaveOccurred())
				Expect(option.Description).To(Equal("elemental-shim"))
				Expect(option.FilePath).To(ContainSubstring("test1.efi"))
				Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test1.efi`))
			})
		})
	})
	Describe("CreateSquashFS", Label("CreateSquashFS"), func() {
		It("runs with no options if none given", func() {
			err := utils.CreateSquashFS(runner, logger, "source", "dest", []string{})
			Expect(runner.IncludesCmds([][]string{
				{"mksquashfs", "source", "dest"},
			})).To(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
		It("runs with options if given", func() {
			err := utils.CreateSquashFS(runner, logger, "source", "dest", constants.GetDefaultSquashfsOptions())
			cmd := []string{"mksquashfs", "source", "dest"}
			cmd = append(cmd, constants.GetDefaultSquashfsOptions()...)
			Expect(runner.IncludesCmds([][]string{
				cmd,
			})).To(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
		It("returns an error if it fails", func() {
			runner.ReturnError = errors.New("error")
			err := utils.CreateSquashFS(runner, logger, "source", "dest", []string{})
			Expect(runner.IncludesCmds([][]string{
				{"mksquashfs", "source", "dest"},
			})).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("LoadEnvFile", Label("LoadEnvFile"), func() {
		BeforeEach(func() {
			fs.Mkdir("/etc", constants.DirPerm)
		})
		It("returns proper map if file exists", func() {
			err := fs.WriteFile("/etc/envfile", []byte("TESTKEY=TESTVALUE"), constants.FilePerm)
			Expect(err).ToNot(HaveOccurred())
			envData, err := utils.LoadEnvFile(fs, "/etc/envfile")
			Expect(err).ToNot(HaveOccurred())
			Expect(envData).To(HaveKeyWithValue("TESTKEY", "TESTVALUE"))
		})
		It("returns error if file doesnt exist", func() {
			_, err := utils.LoadEnvFile(fs, "/etc/envfile")
			Expect(err).To(HaveOccurred())
		})

		It("returns error if it cant unmarshall the env file", func() {
			err := fs.WriteFile("/etc/envfile", []byte("WHAT\"WHAT"), constants.FilePerm)
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.LoadEnvFile(fs, "/etc/envfile")
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("IsMounted", Label("ismounted"), func() {
		It("checks a mounted partition", func() {
			part := &v1.Partition{
				MountPoint: "/some/mountpoint",
			}
			err := mounter.Mount("/some/device", "/some/mountpoint", "auto", []string{})
			Expect(err).ShouldNot(HaveOccurred())
			mnt, err := utils.IsMounted(config, part)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(mnt).To(BeTrue())
		})
		It("checks a not mounted partition", func() {
			part := &v1.Partition{
				MountPoint: "/some/mountpoint",
			}
			mnt, err := utils.IsMounted(config, part)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(mnt).To(BeFalse())
		})
		It("checks a partition without mountpoint", func() {
			part := &v1.Partition{}
			mnt, err := utils.IsMounted(config, part)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(mnt).To(BeFalse())
		})
		It("checks a nil partitiont", func() {
			mnt, err := utils.IsMounted(config, nil)
			Expect(err).Should(HaveOccurred())
			Expect(mnt).To(BeFalse())
		})
	})
	Describe("CleanStack", Label("CleanStack"), func() {
		var cleaner *utils.CleanStack
		BeforeEach(func() {
			cleaner = utils.NewCleanStack()
		})
		It("Adds a callback to the stack and pops it", func() {
			var flag bool
			callback := func() error {
				flag = true
				return nil
			}
			Expect(cleaner.Pop()).To(BeNil())
			cleaner.Push(callback)
			poppedJob := cleaner.Pop()
			Expect(poppedJob).NotTo(BeNil())
			poppedJob()
			Expect(flag).To(BeTrue())
		})
		It("On Cleanup runs callback stack in reverse order", func() {
			result := ""
			callback1 := func() error {
				result = result + "one "
				return nil
			}
			callback2 := func() error {
				result = result + "two "
				return nil
			}
			callback3 := func() error {
				result = result + "three "
				return nil
			}
			cleaner.Push(callback1)
			cleaner.Push(callback2)
			cleaner.Push(callback3)
			cleaner.Cleanup(nil)
			Expect(result).To(Equal("three two one "))
		})
		It("On Cleanup keeps former error and all callbacks are executed", func() {
			err := errors.New("Former error")
			count := 0
			callback := func() error {
				count++
				if count == 2 {
					return errors.New("Cleanup Error")
				}
				return nil
			}
			cleaner.Push(callback)
			cleaner.Push(callback)
			cleaner.Push(callback)
			err = cleaner.Cleanup(err)
			Expect(count).To(Equal(3))
			Expect(err.Error()).To(ContainSubstring("Former error"))
		})
		It("On Cleanup error reports first error and all callbacks are executed", func() {
			var err error
			count := 0
			callback := func() error {
				count++
				if count >= 2 {
					return errors.New(fmt.Sprintf("Cleanup error %d", count))
				}
				return nil
			}
			cleaner.Push(callback)
			cleaner.Push(callback)
			cleaner.Push(callback)
			err = cleaner.Cleanup(err)
			Expect(count).To(Equal(3))
			Expect(err.Error()).To(ContainSubstring("Cleanup error 2"))
			Expect(err.Error()).To(ContainSubstring("Cleanup error 3"))
		})
	})
	Describe("VHD utils", Label("vhd"), func() {
		It("creates a valid header", func() {
			tmpDir, _ := utils.TempDir(fs, "", "")
			f, _ := fs.OpenFile(filepath.Join(tmpDir, "test.vhd"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			utils.RawDiskToFixedVhd(f)
			_ = f.Close()
			f, _ = fs.Open(filepath.Join(tmpDir, "test.vhd"))
			info, _ := f.Stat()
			// Should only have the footer in teh file, hence 512 bytes
			Expect(info.Size()).To(BeNumerically("==", 512))
			// Dump the header from the file into our VHDHeader
			buff := make([]byte, 512)
			_, _ = f.ReadAt(buff, info.Size()-512)
			_ = f.Close()

			header := utils.VHDHeader{}
			err := binary.Read(bytes.NewBuffer(buff[:]), binary.BigEndian, &header)

			Expect(err).ToNot(HaveOccurred())
			// Just check the fields that we know the value of, that should indicate that the header is valid
			Expect(hex.EncodeToString(header.DiskType[:])).To(Equal("00000002"))
			Expect(hex.EncodeToString(header.Features[:])).To(Equal("00000002"))
			Expect(hex.EncodeToString(header.DataOffset[:])).To(Equal("ffffffffffffffff"))
			Expect(hex.EncodeToString(header.CreatorApplication[:])).To(Equal("656c656d"))
		})
		Describe("CHS calculation", func() {
			It("limits the number of sectors", func() {
				tmpDir, _ := utils.TempDir(fs, "", "")
				f, _ := fs.Create(filepath.Join(tmpDir, "test.vhd"))
				// This size would make the chs calculation break, but we have a guard for it
				f.Truncate(500 * 1024 * 1024 * 1024)
				f.Close()
				f, _ = fs.OpenFile(filepath.Join(tmpDir, "test.vhd"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				utils.RawDiskToFixedVhd(f)
				_ = f.Close()
				f, _ = fs.Open(filepath.Join(tmpDir, "test.vhd"))
				info, _ := f.Stat()
				// Dump the header from the file into our VHDHeader
				buff := make([]byte, 512)
				_, _ = f.ReadAt(buff, info.Size()-512)
				_ = f.Close()

				header := utils.VHDHeader{}
				err := binary.Read(bytes.NewBuffer(buff[:]), binary.BigEndian, &header)

				Expect(err).ToNot(HaveOccurred())
				// Just check the fields that we know the value of, that should indicate that the header is valid
				Expect(hex.EncodeToString(header.DiskType[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.Features[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.DataOffset[:])).To(Equal("ffffffffffffffff"))
				// cylinders which is (totalSectors / sectorsPerTrack) / heads
				// and totalsectors is 65535 * 16 * 255 due to hitting the max sector
				// This turns out to be 65535 or ffff in hex or [2]byte{255,255}
				Expect(hex.EncodeToString(header.DiskGeometry[:2])).To(Equal("ffff"))
				Expect(header.DiskGeometry[2]).To(Equal(uint8(16)))  // heads
				Expect(header.DiskGeometry[3]).To(Equal(uint8(255))) // sectors per track
			})
			// The tests below test the different routes that the chs calculation can take to get the disk geometry
			// it's all based on number of sectors, so we have to try with different known sizes to see if the
			// geometry changes are properly reflected on the final VHD header
			It("sets the disk geometry correctly based on sector number", func() {
				tmpDir, _ := utils.TempDir(fs, "", "")
				f, _ := fs.Create(filepath.Join(tmpDir, "test.vhd"))
				// one route of the chs calculation
				f.Truncate(1 * 1024 * 1024)
				f.Close()
				f, _ = fs.OpenFile(filepath.Join(tmpDir, "test.vhd"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				utils.RawDiskToFixedVhd(f)
				_ = f.Close()
				f, _ = fs.Open(filepath.Join(tmpDir, "test.vhd"))
				info, _ := f.Stat()
				// Dump the header from the file into our VHDHeader
				buff := make([]byte, 512)
				_, _ = f.ReadAt(buff, info.Size()-512)
				_ = f.Close()

				header := utils.VHDHeader{}
				err := binary.Read(bytes.NewBuffer(buff[:]), binary.BigEndian, &header)

				Expect(err).ToNot(HaveOccurred())
				// Just check the fields that we know the value of, that should indicate that the header is valid
				Expect(hex.EncodeToString(header.DiskType[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.Features[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.DataOffset[:])).To(Equal("ffffffffffffffff"))
				// should not be the max value
				Expect(hex.EncodeToString(header.DiskGeometry[:2])).ToNot(Equal("ffff"))
				Expect(header.DiskGeometry[2]).To(Equal(uint8(4)))  // heads
				Expect(header.DiskGeometry[3]).To(Equal(uint8(17))) // sectors per track
			})
			It("sets the disk geometry correctly based on sector number", func() {
				tmpDir, _ := utils.TempDir(fs, "", "")
				f, _ := fs.Create(filepath.Join(tmpDir, "test.vhd"))
				// one route of the chs calculation
				f.Truncate(1 * 1024 * 1024 * 1024)
				f.Close()
				f, _ = fs.OpenFile(filepath.Join(tmpDir, "test.vhd"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				utils.RawDiskToFixedVhd(f)
				_ = f.Close()
				f, _ = fs.Open(filepath.Join(tmpDir, "test.vhd"))
				info, _ := f.Stat()
				// Dump the header from the file into our VHDHeader
				buff := make([]byte, 512)
				_, _ = f.ReadAt(buff, info.Size()-512)
				_ = f.Close()

				header := utils.VHDHeader{}
				err := binary.Read(bytes.NewBuffer(buff[:]), binary.BigEndian, &header)

				Expect(err).ToNot(HaveOccurred())
				// Just check the fields that we know the value of, that should indicate that the header is valid
				Expect(hex.EncodeToString(header.DiskType[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.Features[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.DataOffset[:])).To(Equal("ffffffffffffffff"))
				// should not be the max value
				Expect(hex.EncodeToString(header.DiskGeometry[:2])).ToNot(Equal("ffff"))
				Expect(header.DiskGeometry[2]).To(Equal(uint8(16))) // heads
				Expect(header.DiskGeometry[3]).To(Equal(uint8(63))) // sectors per track
			})
			It("sets the disk geometry correctly based on sector number", func() {
				tmpDir, _ := utils.TempDir(fs, "", "")
				f, _ := fs.Create(filepath.Join(tmpDir, "test.vhd"))
				// another route of the chs calculation
				f.Truncate(220 * 1024 * 1024)
				f.Close()
				f, _ = fs.OpenFile(filepath.Join(tmpDir, "test.vhd"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				utils.RawDiskToFixedVhd(f)
				_ = f.Close()
				f, _ = fs.Open(filepath.Join(tmpDir, "test.vhd"))
				info, _ := f.Stat()
				// Dump the header from the file into our VHDHeader
				buff := make([]byte, 512)
				_, _ = f.ReadAt(buff, info.Size()-512)
				_ = f.Close()

				header := utils.VHDHeader{}
				err := binary.Read(bytes.NewBuffer(buff[:]), binary.BigEndian, &header)

				Expect(err).ToNot(HaveOccurred())
				// Just check the fields that we know the value of, that should indicate that the header is valid
				Expect(hex.EncodeToString(header.DiskType[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.Features[:])).To(Equal("00000002"))
				Expect(hex.EncodeToString(header.DataOffset[:])).To(Equal("ffffffffffffffff"))
				// should not be the max value
				Expect(hex.EncodeToString(header.DiskGeometry[:2])).ToNot(Equal("ffff"))
				Expect(header.DiskGeometry[2]).To(Equal(uint8(16))) // heads
				Expect(header.DiskGeometry[3]).To(Equal(uint8(31))) // sectors per track
			})
		})

	})
	Describe("IdentifySourceSystem", Label("fs", "IdentifySourceSystem"), func() {
		var rootDir string
		var buf *bytes.Buffer
		BeforeEach(func() {
			rootDir = constants.ActiveDir
			buf = &bytes.Buffer{}
			logger = v1.NewBufferLogger(buf)
			logger.SetLevel(v1.DebugLevel())
			config.Logger = logger
			err := utils.MkdirAll(fs, filepath.Join(rootDir, "/etc/"), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("fails if os-release doesnt exist", func() {
			_, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("os-release"))
		})
		It("identifies fedora system", func() {
			err := fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("ID=\"fedora\""), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			system, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(system).To(Equal(constants.Fedora))
		})
		It("identifies ubuntu system", func() {
			err := fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("ID=\"ubuntu\""), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			system, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(system).To(Equal(constants.Ubuntu))
		})
		It("identifies suse system", func() {
			err := fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("ID=\"suse\""), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			system, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(system).To(Equal(constants.Suse))
		})
		It("fallback into suse if its an unknown system", func() {
			err := fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("ID=\"sle-micro-for-rancher\""), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			system, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(system).To(Equal(constants.Suse))
		})
		It("fallback into suse if os-release is empty", func() {
			err := fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte(""), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			system, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(system).To(Equal(constants.Suse))
		})
		It("identifies suse system with spaces in the file", func() {
			err := fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("\n\n\nID=\"suse\""), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			system, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(system).To(Equal(constants.Suse))
		})
		It("identifies suse system with comments in the file", func() {
			err := fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("# this is a comment\nID=\"suse\""), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			system, err := utils.IdentifySourceSystem(fs, rootDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(system).To(Equal(constants.Suse))
		})

	})
})
