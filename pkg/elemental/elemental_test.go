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

package elemental_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaypipes/ghw/pkg/block"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const printOutput = `BYT;
/dev/loop0:50593792s:loopback:512:512:gpt:Loopback device:;`
const partTmpl = `
%d:%ss:%ss:2048s:ext4::type=83;`

func TestElementalSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elemental test suite")
}

var _ = Describe("Elemental", Label("elemental"), func() {
	var config *v1.Config
	var runner *v1mock.FakeRunner
	var logger v1.Logger
	var syscall v1.SyscallInterface
	var client *v1mock.FakeHTTPClient
	var mounter *v1mock.FakeMounter
	var extractor *v1mock.FakeImageExtractor
	var fs *vfst.TestFS
	var cleanup func()
	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewFakeMounter()
		client = &v1mock.FakeHTTPClient{}
		logger = v1.NewNullLogger()
		extractor = v1mock.NewFakeImageExtractor(logger)
		fs, cleanup, _ = vfst.NewTestFS(nil)
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
	Describe("MountRWPartition", Label("mount"), func() {
		var el *elemental.Elemental
		var parts v1.ElementalPartitions
		BeforeEach(func() {
			parts = conf.NewInstallElementalPartitions()

			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ToNot(HaveOccurred())
			_, err = fs.Create("/some/device")
			Expect(err).ToNot(HaveOccurred())

			parts.OEM.Path = "/dev/device1"

			el = elemental.NewElemental(config)
		})

		It("Mounts and umounts a partition with RW", func() {
			umount, err := el.MountRWPartition(parts.OEM)
			Expect(err).To(BeNil())
			lst, _ := mounter.List()
			Expect(len(lst)).To(Equal(1))
			Expect(lst[0].Opts).To(Equal([]string{"rw"}))

			Expect(umount()).ShouldNot(HaveOccurred())
			lst, _ = mounter.List()
			Expect(len(lst)).To(Equal(0))
		})
		It("Remounts a partition with RW", func() {
			err := el.MountPartition(parts.OEM)
			Expect(err).To(BeNil())
			lst, _ := mounter.List()
			Expect(len(lst)).To(Equal(1))

			umount, err := el.MountRWPartition(parts.OEM)
			Expect(err).To(BeNil())
			lst, _ = mounter.List()
			// fake mounter is not merging remounts it just appends
			Expect(len(lst)).To(Equal(2))
			Expect(lst[1].Opts).To(Equal([]string{"remount", "rw"}))

			Expect(umount()).ShouldNot(HaveOccurred())
			lst, _ = mounter.List()
			// Increased once more to remount read-onply
			Expect(len(lst)).To(Equal(3))
			Expect(lst[2].Opts).To(Equal([]string{"remount", "ro"}))
		})
		It("Fails to mount a partition", func() {
			mounter.ErrorOnMount = true
			_, err := el.MountRWPartition(parts.OEM)
			Expect(err).Should(HaveOccurred())
		})
		It("Fails to remount a partition", func() {
			err := el.MountPartition(parts.OEM)
			Expect(err).To(BeNil())
			lst, _ := mounter.List()
			Expect(len(lst)).To(Equal(1))

			mounter.ErrorOnMount = true
			_, err = el.MountRWPartition(parts.OEM)
			Expect(err).Should(HaveOccurred())
			lst, _ = mounter.List()
			Expect(len(lst)).To(Equal(1))
		})
	})
	Describe("MountPartitions", Label("MountPartitions", "disk", "partition", "mount"), func() {
		var el *elemental.Elemental
		var parts v1.ElementalPartitions
		BeforeEach(func() {
			parts = conf.NewInstallElementalPartitions()

			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ToNot(HaveOccurred())
			_, err = fs.Create("/some/device")
			Expect(err).ToNot(HaveOccurred())

			parts.OEM.Path = "/dev/device2"
			parts.Recovery.Path = "/dev/device3"
			parts.State.Path = "/dev/device4"
			parts.Persistent.Path = "/dev/device5"

			el = elemental.NewElemental(config)
		})

		It("Mounts disk partitions", func() {
			err := el.MountPartitions(parts.PartitionsByMountPoint(false))
			Expect(err).To(BeNil())
			lst, _ := mounter.List()
			Expect(len(lst)).To(Equal(4))
		})

		It("Mounts disk partitions excluding recovery", func() {
			err := el.MountPartitions(parts.PartitionsByMountPoint(false, parts.Recovery))
			Expect(err).To(BeNil())
			lst, _ := mounter.List()
			for _, i := range lst {
				Expect(i.Path).NotTo(Equal("/dev/device3"))
			}
		})

		It("Fails if some partition resists to mount ", func() {
			mounter.ErrorOnMount = true
			err := el.MountPartitions(parts.PartitionsByMountPoint(false))
			Expect(err).NotTo(BeNil())
		})

		It("Fails if oem partition is not found ", func() {
			parts.OEM.Path = ""
			err := el.MountPartitions(parts.PartitionsByMountPoint(false))
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("UnmountPartitions", Label("UnmountPartitions", "disk", "partition", "unmount"), func() {
		var el *elemental.Elemental
		var parts v1.ElementalPartitions
		BeforeEach(func() {
			parts = conf.NewInstallElementalPartitions()

			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ToNot(HaveOccurred())
			_, err = fs.Create("/some/device")
			Expect(err).ToNot(HaveOccurred())

			parts.OEM.Path = "/dev/device2"
			parts.Recovery.Path = "/dev/device3"
			parts.State.Path = "/dev/device4"
			parts.Persistent.Path = "/dev/device5"

			el = elemental.NewElemental(config)
			err = el.MountPartitions(parts.PartitionsByMountPoint(false))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Unmounts disk partitions", func() {
			err := el.UnmountPartitions(parts.PartitionsByMountPoint(true))
			Expect(err).To(BeNil())
			lst, _ := mounter.List()
			Expect(len(lst)).To(Equal(0))
		})

		It("Fails to unmount disk partitions", func() {
			mounter.ErrorOnUnmount = true
			err := el.UnmountPartitions(parts.PartitionsByMountPoint(true))
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("MountImage", Label("MountImage", "mount", "image"), func() {
		var el *elemental.Elemental
		var img *v1.Image
		BeforeEach(func() {
			el = elemental.NewElemental(config)
			img = &v1.Image{MountPoint: "/some/mountpoint"}
		})

		It("Mounts file system image", func() {
			runner.ReturnValue = []byte("/dev/loop")
			Expect(el.MountImage(img)).To(BeNil())
			Expect(img.LoopDevice).To(Equal("/dev/loop"))
		})

		It("Fails to set a loop device", Label("loop"), func() {
			runner.ReturnError = errors.New("failed to set a loop device")
			Expect(el.MountImage(img)).NotTo(BeNil())
			Expect(img.LoopDevice).To(Equal(""))
		})

		It("Fails to mount a loop device", Label("loop"), func() {
			runner.ReturnValue = []byte("/dev/loop")
			mounter.ErrorOnMount = true
			Expect(el.MountImage(img)).NotTo(BeNil())
			Expect(img.LoopDevice).To(Equal(""))
		})
	})

	Describe("UnmountImage", Label("UnmountImage", "mount", "image"), func() {
		var el *elemental.Elemental
		var img *v1.Image
		BeforeEach(func() {
			runner.ReturnValue = []byte("/dev/loop")
			el = elemental.NewElemental(config)
			img = &v1.Image{MountPoint: "/some/mountpoint"}
			Expect(el.MountImage(img)).To(BeNil())
			Expect(img.LoopDevice).To(Equal("/dev/loop"))
		})

		It("Unmounts file system image", func() {
			Expect(el.UnmountImage(img)).To(BeNil())
			Expect(img.LoopDevice).To(Equal(""))
		})

		It("Fails to unmount a mountpoint", func() {
			mounter.ErrorOnUnmount = true
			Expect(el.UnmountImage(img)).NotTo(BeNil())
		})

		It("Fails to unset a loop device", Label("loop"), func() {
			runner.ReturnError = errors.New("failed to unset a loop device")
			Expect(el.UnmountImage(img)).NotTo(BeNil())
		})
	})

	Describe("CreateFileSystemImage", Label("CreateFileSystemImage", "image"), func() {
		var el *elemental.Elemental
		var img *v1.Image
		BeforeEach(func() {
			img = &v1.Image{
				Label:      constants.ActiveLabel,
				Size:       32,
				File:       filepath.Join(constants.StateDir, constants.ActiveImgPath),
				FS:         constants.LinuxImgFs,
				MountPoint: constants.ActiveDir,
				Source:     v1.NewDirSrc(constants.ISOBaseTree),
			}
			_ = utils.MkdirAll(fs, constants.ISOBaseTree, constants.DirPerm)
			el = elemental.NewElemental(config)
		})

		It("Creates a new file system image", func() {
			_, err := fs.Stat(img.File)
			Expect(err).NotTo(BeNil())
			err = el.CreateFileSystemImage(img)
			Expect(err).To(BeNil())
			stat, err := fs.Stat(img.File)
			Expect(err).To(BeNil())
			Expect(stat.Size()).To(Equal(int64(32 * 1024 * 1024)))
		})

		It("Fails formatting a file system image", Label("format"), func() {
			runner.ReturnError = errors.New("run error")
			_, err := fs.Stat(img.File)
			Expect(err).NotTo(BeNil())
			err = el.CreateFileSystemImage(img)
			Expect(err).NotTo(BeNil())
			_, err = fs.Stat(img.File)
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("FormatPartition", Label("FormatPartition", "partition", "format"), func() {
		It("Reformats an already existing partition", func() {
			el := elemental.NewElemental(config)
			part := &v1.Partition{
				Path:            "/dev/device1",
				FS:              "ext4",
				FilesystemLabel: "MY_LABEL",
			}
			Expect(el.FormatPartition(part)).To(BeNil())
		})

	})
	Describe("PartitionAndFormatDevice", Label("PartitionAndFormatDevice", "partition", "format"), func() {
		var el *elemental.Elemental
		var cInit *v1mock.FakeCloudInitRunner
		var partNum int
		var printOut string
		var failPart bool
		var install *v1.InstallSpec

		BeforeEach(func() {
			cInit = &v1mock.FakeCloudInitRunner{ExecStages: []string{}, Error: false}
			config.CloudInitRunner = cInit
			el = elemental.NewElemental(config)
			install = conf.NewInstallSpec(*config)
			install.Target = "/some/device"

			err := utils.MkdirAll(fs, "/some", constants.DirPerm)
			Expect(err).ToNot(HaveOccurred())
			_, err = fs.Create("/some/device")
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("Successful run", func() {
			var runFunc func(cmd string, args ...string) ([]byte, error)
			var efiPartCmds, partCmds, biosPartCmds [][]string
			BeforeEach(func() {
				partNum, printOut = 0, printOutput
				err := utils.MkdirAll(fs, "/some", constants.DirPerm)
				Expect(err).To(BeNil())
				efiPartCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mklabel", "gpt",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "efi", "fat32", "2048", "133119", "set", "1", "esp", "on",
					}, {"mkfs.vfat", "-n", "COS_GRUB", "/some/device1"},
				}
				biosPartCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mklabel", "gpt",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "bios", "", "2048", "4095", "set", "1", "bios_grub", "on",
					}, {"wipefs", "--all", "/some/device1"},
				}
				// These commands are only valid for EFI case
				partCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "oem", "ext4", "133120", "264191",
					}, {"mkfs.ext4", "-L", "COS_OEM", "/some/device2"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "recovery", "ext4", "264192", "8652799",
					}, {"mkfs.ext4", "-L", "COS_RECOVERY", "/some/device3"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "state", "ext4", "8652800", "25430015",
					}, {"mkfs.ext4", "-L", "COS_STATE", "/some/device4"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "persistent", "ext4", "25430016", "100%",
					}, {"mkfs.ext4", "-L", "COS_PERSISTENT", "/some/device5"},
				}

				runFunc = func(cmd string, args ...string) ([]byte, error) {
					switch cmd {
					case "parted":
						idx := 0
						for i, arg := range args {
							if arg == "mkpart" {
								idx = i
								break
							}
						}
						if idx > 0 {
							partNum++
							printOut += fmt.Sprintf(partTmpl, partNum, args[idx+3], args[idx+4])
							_, _ = fs.Create(fmt.Sprintf("/some/device%d", partNum))
						}
						return []byte(printOut), nil
					default:
						return []byte{}, nil
					}
				}
				runner.SideEffect = runFunc
			})

			It("Successfully creates partitions and formats them, EFI boot", func() {
				install.PartTable = v1.GPT
				install.Firmware = v1.EFI
				install.Partitions.SetFirmwarePartitions(v1.EFI, v1.GPT)
				Expect(el.PartitionAndFormatDevice(install)).To(BeNil())
				Expect(runner.MatchMilestones(append(efiPartCmds, partCmds...))).To(BeNil())
			})

			It("Successfully creates partitions and formats them, BIOS boot", func() {
				install.PartTable = v1.GPT
				install.Firmware = v1.BIOS
				install.Partitions.SetFirmwarePartitions(v1.BIOS, v1.GPT)
				Expect(el.PartitionAndFormatDevice(install)).To(BeNil())
				Expect(runner.MatchMilestones(biosPartCmds)).To(BeNil())
			})
		})

		Describe("Run with failures", func() {
			var runFunc func(cmd string, args ...string) ([]byte, error)
			BeforeEach(func() {
				err := utils.MkdirAll(fs, "/some", constants.DirPerm)
				Expect(err).To(BeNil())
				partNum, printOut = 0, printOutput
				runFunc = func(cmd string, args ...string) ([]byte, error) {
					switch cmd {
					case "parted":
						idx := 0
						for i, arg := range args {
							if arg == "mkpart" {
								idx = i
								break
							}
						}
						if idx > 0 {
							partNum++
							printOut += fmt.Sprintf(partTmpl, partNum, args[idx+3], args[idx+4])
							if failPart {
								return []byte{}, errors.New("Failure")
							}
							_, _ = fs.Create(fmt.Sprintf("/some/device%d", partNum))
						}
						return []byte(printOut), nil
					case "mkfs.ext4", "wipefs", "mkfs.vfat":
						return []byte{}, errors.New("Failure")
					default:
						return []byte{}, nil
					}
				}
				runner.SideEffect = runFunc
			})

			It("Fails creating efi partition", func() {
				failPart = true
				Expect(el.PartitionAndFormatDevice(install)).NotTo(BeNil())
				// Failed to create first partition
				Expect(partNum).To(Equal(1))
			})

			It("Fails formatting efi partition", func() {
				failPart = false
				Expect(el.PartitionAndFormatDevice(install)).NotTo(BeNil())
				// Failed to format first partition
				Expect(partNum).To(Equal(1))
			})
		})
	})
	Describe("DumpSource", Label("dump"), func() {
		var e *elemental.Elemental
		var destDir string
		BeforeEach(func() {
			var err error
			e = elemental.NewElemental(config)
			destDir, err = utils.TempDir(fs, "", "elemental")
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Copies files from a directory source", func() {
			rsyncCount := 0
			src := ""
			dest := ""

			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == "rsync" {
					rsyncCount += 1
					src = args[len(args)-2]
					dest = args[len(args)-1]
				}

				return []byte{}, nil
			}

			_, err := e.DumpSource("/dest", v1.NewDirSrc("/source"))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(rsyncCount).To(Equal(1))
			Expect(src).To(HaveSuffix("/source/"))
			Expect(dest).To(HaveSuffix("/dest/"))
		})
		It("Unpacks a docker image to target", Label("docker"), func() {
			_, err := e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))
			Expect(err).To(BeNil())
		})
		It("Unpacks a docker image to target with cosign validation", Label("docker", "cosign"), func() {
			config.Cosign = true
			_, err := e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch([][]string{{"cosign", "verify", "docker/image:latest"}}))
		})
		It("Fails cosign validation", Label("cosign"), func() {
			runner.ReturnError = errors.New("cosign error")
			config.Cosign = true
			_, err := e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))
			Expect(err).NotTo(BeNil())
			Expect(runner.CmdsMatch([][]string{{"cosign", "verify", "docker/image:latest"}}))
		})
		It("Fails to unpack a docker image to target", Label("docker"), func() {
			unpackErr := errors.New("failed to unpack")
			extractor.SideEffect = func(_, _, _ string, _ bool) error { return unpackErr }
			_, err := e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))
			Expect(err).To(Equal(unpackErr))
		})
		It("Copies image file to target", func() {
			sourceImg := "/source.img"
			destFile := filepath.Join(destDir, "active.img")

			_, err := e.DumpSource(destFile, v1.NewFileSrc(sourceImg))
			Expect(err).To(BeNil())
			Expect(runner.IncludesCmds([][]string{{"rsync"}}))
		})
		It("Fails to copy, source can't be mounted", func() {
			mounter.ErrorOnMount = true
			_, err := e.DumpSource("whatever", v1.NewFileSrc("/source.img"))
			Expect(err).To(HaveOccurred())
		})
		It("Fails to copy, no write permissions", func() {
			sourceImg := "/source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			config.Fs = vfs.NewReadOnlyFS(fs)
			_, err = e.DumpSource("whatever", v1.NewFileSrc("/source.img"))
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("DeployImgTree", Label("deployImgTree"), func() {
		var e *elemental.Elemental
		var imgFile, srcDir, root string
		var img *v1.Image

		BeforeEach(func() {
			e = elemental.NewElemental(config)

			imgFile = "/statePart/dst.img"
			root = "/workingDir"

			srcDir = "/srcDir"
			Expect(utils.MkdirAll(fs, srcDir, constants.DirPerm)).To(Succeed())

			img = &v1.Image{
				File:   imgFile,
				Source: v1.NewDirSrc(srcDir),
			}
		})
		It("Deploys an image including the root tree contents", func() {
			_, cleaner, err := e.DeployImgTree(img, root)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(runner.IncludesCmds([][]string{{"rsync"}}))
			Expect(cleaner()).To(Succeed())
		})
		It("Fails setting a bind mount to root", func() {
			mounter.ErrorOnMount = true
			_, _, err := e.DeployImgTree(img, root)
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("CreateImgFromTree", Label("createImg"), func() {
		var e *elemental.Elemental
		var imgFile, root string
		var img *v1.Image
		var cleaned bool

		BeforeEach(func() {
			cleaned = false
			e = elemental.NewElemental(config)
			destDir, err := utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())
			root, err = utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())

			imgFile = filepath.Join(destDir, "dst.img")
			sf, err := fs.Create(filepath.Join(root, "somefile"))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(sf.Truncate(32 * 1024 * 1024)).To(Succeed())
			Expect(sf.Close()).To(Succeed())

			Expect(err).ShouldNot(HaveOccurred())
			img = &v1.Image{
				FS:         constants.LinuxImgFs,
				File:       imgFile,
				MountPoint: "/some/mountpoint",
			}
		})
		It("Creates an image including including the root tree contents", func() {
			cleaner := func() error {
				cleaned = true
				return nil
			}
			err := e.CreateImgFromTree(root, img, false, cleaner)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(img.Size).To(Equal(32 + constants.ImgOverhead + 1))
			Expect(cleaned).To(BeTrue())
		})
		It("Creates an squashfs image", func() {
			img.FS = constants.SquashFs
			err := e.CreateImgFromTree(root, img, false, nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(img.Size).To(Equal(uint(0)))
			Expect(runner.IncludesCmds([][]string{{"mksquashfs"}}))
		})
		It("Creates an image of an specific size including including the root tree contents", func() {
			img.Size = 64
			err := e.CreateImgFromTree(root, img, false, nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(img.Size).To(Equal(uint(64)))
			Expect(runner.IncludesCmds([][]string{{"rsync"}}))
		})
		It("Fails to mount created filesystem image", func() {
			mounter.ErrorOnUnmount = true
			err := e.CreateImgFromTree(root, img, false, nil)
			Expect(err).Should(HaveOccurred())
			Expect(img.Size).To(Equal(32 + constants.ImgOverhead + 1))
			Expect(cleaned).To(BeFalse())
			Expect(runner.IncludesCmds([][]string{{"rsync"}}))
		})
	})
	Describe("DeployImage", Label("deployImg"), func() {
		var e *elemental.Elemental
		var imgFile, srcDir string
		var img *v1.Image

		BeforeEach(func() {
			e = elemental.NewElemental(config)
			destDir, err := utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())
			srcDir, err = utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())

			imgFile = filepath.Join(destDir, "dst.img")

			sf, err := fs.Create(filepath.Join(srcDir, "somefile"))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(sf.Truncate(32 * 1024 * 1024)).To(Succeed())
			Expect(sf.Close()).To(Succeed())

			Expect(err).ShouldNot(HaveOccurred())
			img = &v1.Image{
				FS:         constants.LinuxImgFs,
				File:       imgFile,
				MountPoint: "/some/mountpoint",
				Source:     v1.NewDirSrc(srcDir),
			}
		})
		It("Deploys image source into a filesystem image", func() {
			_, err := e.DeployImage(img)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(runner.IncludesCmds([][]string{{"mkfs.ext2"}, {"rsync"}})).To(Succeed())
		})
		It("Fails to dump source without write permissions", func() {
			config.Fs = vfs.NewReadOnlyFS(fs)
			_, err := e.DeployImage(img)
			Expect(err).Should(HaveOccurred())
			Expect(runner.IncludesCmds([][]string{{"mkfs.ext2"}})).NotTo(Succeed())
		})
		It("Fails to create filesystem", func() {
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == "mkfs.ext2" {
					return []byte{}, fmt.Errorf("Failed calling mkfs.ext2")
				}
				return []byte{}, nil
			}
			_, err := e.DeployImage(img)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("calling mkfs.ext2"))
			Expect(runner.IncludesCmds([][]string{{"mkfs.ext2"}})).To(Succeed())
		})
	})
	Describe("CopyImgFile", Label("copyimg"), func() {
		var e *elemental.Elemental
		var imgFile, srcFile string
		var img *v1.Image
		var fileContent []byte
		BeforeEach(func() {
			e = elemental.NewElemental(config)
			destDir, err := utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())
			imgFile = filepath.Join(destDir, "dst.img")
			srcFile = filepath.Join(destDir, "src.img")
			fileContent = []byte("imagefile")
			err = fs.WriteFile(srcFile, fileContent, constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())
			img = &v1.Image{
				Label:  "myLabel",
				FS:     constants.LinuxImgFs,
				File:   imgFile,
				Source: v1.NewFileSrc(srcFile),
			}
		})
		It("Copies image file and sets new label", func() {
			err := e.CopyFileImg(img)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(runner.IncludesCmds([][]string{{"tune2fs", "-L", img.Label, img.File}})).To(BeNil())
			data, err := fs.ReadFile(imgFile)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(data).To(Equal(fileContent))
		})
		It("Copies image file and without setting a new label", func() {
			img.FS = constants.SquashFs
			err := e.CopyFileImg(img)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(runner.IncludesCmds([][]string{{"tune2fs", "-L", img.Label, img.File}})).NotTo(BeNil())
			data, err := fs.ReadFile(imgFile)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(data).To(Equal(fileContent))
		})
		It("Fails to copy image if source is not of file type", func() {
			img.Source = v1.NewEmptySrc()
			err := e.CopyFileImg(img)
			Expect(err).Should(HaveOccurred())
		})
		It("Fails to copy image if source does not exist", func() {
			img.Source = v1.NewFileSrc("whatever")
			err := e.CopyFileImg(img)
			Expect(err).Should(HaveOccurred())
		})
		It("Fails to copy image if it can't create target dir", func() {
			img.File = "/new/path.img"
			config.Fs = vfs.NewReadOnlyFS(fs)
			err := e.CopyFileImg(img)
			Expect(err).Should(HaveOccurred())
		})
		It("Fails to copy image if it can't write a new file", func() {
			config.Fs = vfs.NewReadOnlyFS(fs)
			err := e.CopyFileImg(img)
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("CheckActiveDeployment", Label("check"), func() {
		It("deployment found", func() {
			ghwTest := v1mock.GhwMock{}
			disk := block.Disk{Name: "device", Partitions: []*block.Partition{
				{
					Name:            "device1",
					FilesystemLabel: constants.ActiveLabel,
				},
			}}
			ghwTest.AddDisk(disk)
			ghwTest.CreateDevices()
			defer ghwTest.Clean()
			runner.ReturnValue = []byte(
				fmt.Sprintf(
					`{"blockdevices": [{"label": "%s", "type": "loop", "path": "/some/device"}]}`,
					constants.ActiveLabel,
				),
			)
			e := elemental.NewElemental(config)
			Expect(e.CheckActiveDeployment([]string{constants.ActiveLabel, constants.PassiveLabel})).To(BeTrue())
		})

		It("Should not error out", func() {
			runner.ReturnValue = []byte("")
			e := elemental.NewElemental(config)
			Expect(e.CheckActiveDeployment([]string{constants.ActiveLabel, constants.PassiveLabel})).To(BeFalse())
		})
	})
	Describe("SelinuxRelabel", Label("SelinuxRelabel", "selinux"), func() {
		var policyFile string
		var relabelCmd []string
		BeforeEach(func() {
			// to mock the existance of setfiles command on non selinux hosts
			err := utils.MkdirAll(fs, "/usr/sbin", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			sbin, err := fs.RawPath("/usr/sbin")
			Expect(err).ShouldNot(HaveOccurred())

			path := os.Getenv("PATH")
			os.Setenv("PATH", fmt.Sprintf("%s:%s", sbin, path))
			_, err = fs.Create("/usr/sbin/setfiles")
			Expect(err).ShouldNot(HaveOccurred())
			err = fs.Chmod("/usr/sbin/setfiles", 0777)
			Expect(err).ShouldNot(HaveOccurred())

			// to mock SELinux policy files
			policyFile = filepath.Join(constants.SELinuxTargetedPolicyPath, "policy.31")
			err = utils.MkdirAll(fs, filepath.Dir(constants.SELinuxTargetedContextFile), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(constants.SELinuxTargetedContextFile)
			Expect(err).ShouldNot(HaveOccurred())
			err = utils.MkdirAll(fs, constants.SELinuxTargetedPolicyPath, constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(policyFile)
			Expect(err).ShouldNot(HaveOccurred())

			relabelCmd = []string{
				"setfiles", "-c", policyFile, "-e", "/dev", "-e", "/proc", "-e", "/sys",
				"-F", constants.SELinuxTargetedContextFile, "/",
			}
		})
		It("does nothing if the context file is not found", func() {
			err := fs.Remove(constants.SELinuxTargetedContextFile)
			Expect(err).ShouldNot(HaveOccurred())

			c := elemental.NewElemental(config)
			Expect(c.SelinuxRelabel("/", true)).To(BeNil())
			Expect(runner.CmdsMatch([][]string{{}}))
		})
		It("does nothing if the policy file is not found", func() {
			err := fs.Remove(policyFile)
			Expect(err).ShouldNot(HaveOccurred())

			c := elemental.NewElemental(config)
			Expect(c.SelinuxRelabel("/", true)).To(BeNil())
			Expect(runner.CmdsMatch([][]string{{}}))
		})
		It("relabels the current root", func() {
			c := elemental.NewElemental(config)
			Expect(c.SelinuxRelabel("", true)).To(BeNil())
			Expect(runner.CmdsMatch([][]string{relabelCmd})).To(BeNil())

			runner.ClearCmds()
			Expect(c.SelinuxRelabel("/", true)).To(BeNil())
			Expect(runner.CmdsMatch([][]string{relabelCmd})).To(BeNil())
		})
		It("fails to relabel the current root", func() {
			runner.ReturnError = errors.New("setfiles failure")
			c := elemental.NewElemental(config)
			Expect(c.SelinuxRelabel("", true)).NotTo(BeNil())
			Expect(runner.CmdsMatch([][]string{relabelCmd})).To(BeNil())
		})
		It("ignores relabel failures", func() {
			runner.ReturnError = errors.New("setfiles failure")
			c := elemental.NewElemental(config)
			Expect(c.SelinuxRelabel("", false)).To(BeNil())
			Expect(runner.CmdsMatch([][]string{relabelCmd})).To(BeNil())
		})
		It("relabels the given root-tree path", func() {
			contextFile := filepath.Join("/root", constants.SELinuxTargetedContextFile)
			err := utils.MkdirAll(fs, filepath.Dir(contextFile), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(contextFile)
			Expect(err).ShouldNot(HaveOccurred())
			policyFile = filepath.Join("/root", policyFile)
			err = utils.MkdirAll(fs, filepath.Join("/root", constants.SELinuxTargetedPolicyPath), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(policyFile)
			Expect(err).ShouldNot(HaveOccurred())

			relabelCmd = []string{
				"setfiles", "-c", policyFile, "-F", "-r", "/root", contextFile, "/root",
			}

			c := elemental.NewElemental(config)
			Expect(c.SelinuxRelabel("/root", true)).To(BeNil())
			Expect(runner.CmdsMatch([][]string{relabelCmd})).To(BeNil())
		})
	})
	Describe("GetIso", Label("GetIso", "iso"), func() {
		var e *elemental.Elemental
		var activeImg *v1.Image
		BeforeEach(func() {
			activeImg = &v1.Image{}
			e = elemental.NewElemental(config)
		})
		It("Gets the iso, mounts it and updates image source", func() {
			tmpDir, err := utils.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = fs.WriteFile(fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), constants.FilePerm)
			Expect(err).To(BeNil())
			iso := fmt.Sprintf("%s/fake.iso", tmpDir)

			// Create the internal ISO file structure
			rootfsImg := filepath.Join(os.TempDir(), "/elemental/iso", constants.ISORootFile)
			Expect(utils.MkdirAll(fs, filepath.Dir(rootfsImg), constants.DirPerm)).To(Succeed())
			Expect(fs.WriteFile(rootfsImg, []byte{}, constants.FilePerm)).To(Succeed())

			isoClean, err := e.UpdateSourceFormISO(iso, activeImg)
			Expect(err).To(BeNil())
			Expect(activeImg.Source.IsFile()).To(BeTrue())
			Expect(isoClean()).To(Succeed())
		})
		It("Fails if it cant find the iso", func() {
			iso := "whatever"
			e := elemental.NewElemental(config)
			isoClean, err := e.UpdateSourceFormISO(iso, activeImg)
			Expect(err).ToNot(BeNil())
			Expect(isoClean()).To(Succeed())
		})
		It("Fails creating the mountpoint", func() {
			tmpDir, err := utils.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = fs.WriteFile(fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), constants.FilePerm)
			Expect(err).To(BeNil())
			iso := fmt.Sprintf("%s/fake.iso", tmpDir)
			config.Fs = vfs.NewReadOnlyFS(fs)

			isoClean, err := e.UpdateSourceFormISO(iso, activeImg)
			Expect(err).ToNot(BeNil())
			Expect(isoClean()).To(Succeed())
		})
		It("Fails if it cannot mount the iso", func() {
			mounter.ErrorOnMount = true
			tmpDir, err := utils.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = fs.WriteFile(fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), constants.FilePerm)
			Expect(err).To(BeNil())
			iso := fmt.Sprintf("%s/fake.iso", tmpDir)
			isoClean, err := e.UpdateSourceFormISO(iso, activeImg)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("mount error"))
			Expect(isoClean()).To(Succeed())
		})
	})
	Describe("CloudConfig", Label("CloudConfig", "cloud-config"), func() {
		var e *elemental.Elemental
		var parts v1.ElementalPartitions
		BeforeEach(func() {
			parts = conf.NewInstallElementalPartitions()
			e = elemental.NewElemental(config)
		})
		It("Copies the cloud config file", func() {
			testString := "In a galaxy far far away..."
			cloudInit := []string{"/config.yaml"}
			err := fs.WriteFile(cloudInit[0], []byte(testString), constants.FilePerm)
			Expect(err).To(BeNil())
			Expect(err).To(BeNil())

			err = e.CopyCloudConfig(parts.GetConfigStorage(), cloudInit)
			Expect(err).To(BeNil())
			copiedFile, err := fs.ReadFile(fmt.Sprintf("%s/90_custom.yaml", constants.OEMDir))
			Expect(err).To(BeNil())
			Expect(copiedFile).To(ContainSubstring(testString))
		})
		It("Doesnt do anything if the config file is not set", func() {
			err := e.CopyCloudConfig(parts.GetConfigStorage(), []string{})
			Expect(err).To(BeNil())
		})
		It("Doesnt do anything if the OEM partition has no mount point", func() {
			parts.OEM.MountPoint = ""
			err := e.CopyCloudConfig(parts.GetConfigStorage(), []string{})
			Expect(err).To(BeNil())
		})
	})
	Describe("DeactivateDevices", Label("blkdeactivate"), func() {
		It("calls blkdeactivat", func() {
			el := elemental.NewElemental(config)
			err := el.DeactivateDevices()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(runner.CmdsMatch([][]string{{
				"blkdeactivate", "--lvmoptions", "retry,wholevg",
				"--dmoptions", "force,retry", "--errors",
			}})).To(BeNil())
		})
	})
})

// PathInMountPoints will check if the given path is in the mountPoints list
func pathInMountPoints(mounter *v1mock.FakeMounter, path string) bool {
	mountPoints, _ := mounter.List()
	for _, m := range mountPoints {
		if path == m.Path {
			return true
		}
	}
	return false
}
