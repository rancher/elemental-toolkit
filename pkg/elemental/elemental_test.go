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
	conf "github.com/rancher/elemental-cli/pkg/config"
	"github.com/rancher/elemental-cli/pkg/constants"
	cnst "github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/elemental"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
	v1mock "github.com/rancher/elemental-cli/tests/mocks"
	"github.com/twpayne/go-vfs/vfst"
	"k8s.io/mount-utils"
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
	var mounter *v1mock.ErrorMounter
	var fs *vfst.TestFS
	var cleanup func()
	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHTTPClient{}
		logger = v1.NewNullLogger()
		fs, cleanup, _ = vfst.NewTestFS(nil)
		config = conf.NewConfig(
			conf.WithFs(fs),
			conf.WithRunner(runner),
			conf.WithLogger(logger),
			conf.WithMounter(mounter),
			conf.WithSyscall(syscall),
			conf.WithClient(client),
		)
	})
	AfterEach(func() { cleanup() })
	Describe("MountPartitions", Label("MountPartitions", "disk", "partition", "mount"), func() {
		var el *elemental.Elemental
		var parts v1.ElementalPartitions
		BeforeEach(func() {
			parts = conf.NewInstallElementalParitions()

			err := utils.MkdirAll(fs, "/some", cnst.DirPerm)
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
			parts = conf.NewInstallElementalParitions()

			err := utils.MkdirAll(fs, "/some", cnst.DirPerm)
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
				Label:      cnst.ActiveLabel,
				Size:       32,
				File:       filepath.Join(cnst.StateDir, "cOS", cnst.ActiveImgFile),
				FS:         cnst.LinuxImgFs,
				MountPoint: cnst.ActiveDir,
				Source:     v1.NewDirSrc(cnst.IsoBaseTree),
			}
			_ = utils.MkdirAll(fs, cnst.IsoBaseTree, cnst.DirPerm)
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

			err := utils.MkdirAll(fs, "/some", cnst.DirPerm)
			Expect(err).ToNot(HaveOccurred())
			_, err = fs.Create("/some/device")
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("Successful run", func() {
			var runFunc func(cmd string, args ...string) ([]byte, error)
			var efiPartCmds, partCmds, biosPartCmds [][]string
			BeforeEach(func() {
				partNum, printOut = 0, printOutput
				err := utils.MkdirAll(fs, "/some", cnst.DirPerm)
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
						"mkpart", "recovery", "ext4", "264192", "17041407",
					}, {"mkfs.ext4", "-L", "COS_RECOVERY", "/some/device3"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "state", "ext4", "17041408", "48498687",
					}, {"mkfs.ext4", "-L", "COS_STATE", "/some/device4"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "persistent", "ext4", "48498688", "100%",
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
				err := utils.MkdirAll(fs, "/some", cnst.DirPerm)
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
	Describe("DeployImage", Label("DeployImage"), func() {
		var el *elemental.Elemental
		var img *v1.Image
		var cmdFail string
		BeforeEach(func() {
			sourceDir, err := utils.TempDir(fs, "", "elemental")
			Expect(err).ShouldNot(HaveOccurred())
			destDir, err := utils.TempDir(fs, "", "elemental")
			Expect(err).ShouldNot(HaveOccurred())
			cmdFail = ""
			el = elemental.NewElemental(config)
			img = &v1.Image{
				FS:         constants.LinuxImgFs,
				Size:       16,
				Source:     v1.NewDirSrc(sourceDir),
				MountPoint: destDir,
				File:       filepath.Join(destDir, "image.img"),
				Label:      "some_label",
			}
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmdFail == cmd {
					return []byte{}, errors.New("Command failed")
				}
				switch cmd {
				case "losetup":
					return []byte("/dev/loop"), nil
				default:
					return []byte{}, nil
				}
			}
		})
		It("Deploys an image from a directory and leaves it mounted", func() {
			Expect(el.DeployImage(img, true)).To(BeNil())
		})
		It("Deploys an image from a directory and leaves it unmounted", func() {
			Expect(el.DeployImage(img, false)).To(BeNil())
		})
		It("Deploys an squashfs image from a directory", func() {
			img.FS = constants.SquashFs
			Expect(el.DeployImage(img, true)).To(BeNil())
			Expect(runner.MatchMilestones([][]string{
				{
					"mksquashfs", "/tmp/elemental-tmp", "/tmp/elemental/image.img",
					"-b", "1024k", "-comp", "xz", "-Xbcj", "x86",
				},
			}))
		})
		It("Deploys a file image and mounts it", func() {
			sourceImg := "/source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destDir, err := utils.TempDir(fs, "", "elemental")
			Expect(err).To(BeNil())
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			Expect(el.DeployImage(img, true)).To(BeNil())
		})
		It("Deploys a file image and fails to mount it", func() {
			sourceImg := "/source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destDir, err := utils.TempDir(fs, "", "elemental")
			Expect(err).To(BeNil())
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			mounter.ErrorOnMount = true
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Deploys a file image and fails to label it", func() {
			sourceImg := "/source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destDir, err := utils.TempDir(fs, "", "elemental")
			Expect(err).To(BeNil())
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			cmdFail = "tune2fs"
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails creating the squashfs filesystem", func() {
			cmdFail = "mksquashfs"
			img.FS = constants.SquashFs
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
			Expect(runner.MatchMilestones([][]string{
				{
					"mksquashfs", "/tmp/elemental-tmp", "/tmp/elemental/image.img",
					"-b", "1024k", "-comp", "xz", "-Xbcj", "x86",
				},
			}))
		})
		It("Fails formatting the image", func() {
			cmdFail = "mkfs.ext2"
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails mounting the image", func() {
			mounter.ErrorOnMount = true
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails copying the image if source does not exist", func() {
			img.Source = v1.NewDirSrc("/welp")
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails unmounting the image after copying", func() {
			mounter.ErrorOnUnmount = true
			Expect(el.DeployImage(img, false)).NotTo(BeNil())
		})
	})
	Describe("DumpSource", Label("dump"), func() {
		var e *elemental.Elemental
		var destDir string
		var luet *v1mock.FakeLuet
		BeforeEach(func() {
			var err error
			luet = v1mock.NewFakeLuet()
			config.Luet = luet
			e = elemental.NewElemental(config)
			destDir, err = utils.TempDir(fs, "", "elemental")
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Copies files from a directory source", func() {
			sourceDir, err := utils.TempDir(fs, "", "elemental")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(e.DumpSource(destDir, v1.NewDirSrc(sourceDir))).To(BeNil())
		})
		It("Fails if source directory does not exist", func() {
			Expect(e.DumpSource(destDir, v1.NewDirSrc("/welp"))).ToNot(BeNil())
		})
		It("Unpacks a docker image to target", Label("docker"), func() {
			Expect(e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))).To(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})
		It("Unpacks a docker image to target with cosign validation", Label("docker", "cosign"), func() {
			config.Cosign = true
			Expect(e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))).To(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
			Expect(runner.CmdsMatch([][]string{{"cosign", "verify", "docker/image:latest"}}))
		})
		It("Fails cosign validation", Label("cosign"), func() {
			runner.ReturnError = errors.New("cosign error")
			config.Cosign = true
			Expect(e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))).NotTo(BeNil())
			Expect(runner.CmdsMatch([][]string{{"cosign", "verify", "docker/image:latest"}}))
		})
		It("Fails to unpack a docker image to target", Label("docker"), func() {
			luet.OnUnpackError = true
			Expect(e.DumpSource(destDir, v1.NewDockerSrc("docker/image:latest"))).NotTo(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})
		It("Copies image file to target", func() {
			sourceImg := "/source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destFile := filepath.Join(destDir, "active.img")
			_, err = fs.Stat(destFile)
			Expect(err).NotTo(BeNil())
			Expect(e.DumpSource(destFile, v1.NewFileSrc(sourceImg))).To(BeNil())
			_, err = fs.Stat(destFile)
			Expect(err).To(BeNil())
		})
		It("Fails to copy, source file is not present", func() {
			Expect(e.DumpSource("whatever", v1.NewFileSrc("/source.img"))).NotTo(BeNil())
		})
		It("Unpacks from channel to target", func() {
			Expect(e.DumpSource(destDir, v1.NewChannelSrc("some/package"))).To(BeNil())
			Expect(luet.UnpackChannelCalled()).To(BeTrue())
		})
		It("Fails to unpack from channel to target", func() {
			luet.OnUnpackFromChannelError = true
			Expect(e.DumpSource(destDir, v1.NewChannelSrc("some/package"))).NotTo(BeNil())
			Expect(luet.UnpackChannelCalled()).To(BeTrue())
		})
	})
	Describe("CheckActiveDeployment", Label("check"), func() {
		It("deployment found", func() {
			ghwTest := v1mock.GhwMock{}
			disk := block.Disk{Name: "device", Partitions: []*block.Partition{
				{
					Name:            "device1",
					FilesystemLabel: cnst.ActiveLabel,
				},
			}}
			ghwTest.AddDisk(disk)
			ghwTest.CreateDevices()
			defer ghwTest.Clean()
			runner.ReturnValue = []byte(
				fmt.Sprintf(
					`{"blockdevices": [{"label": "%s", "type": "loop", "path": "/some/device"}]}`,
					cnst.ActiveLabel,
				),
			)
			e := elemental.NewElemental(config)
			Expect(e.CheckActiveDeployment([]string{cnst.ActiveLabel, cnst.PassiveLabel})).To(BeTrue())
		})

		It("Should not error out", func() {
			runner.ReturnValue = []byte("")
			e := elemental.NewElemental(config)
			Expect(e.CheckActiveDeployment([]string{cnst.ActiveLabel, cnst.PassiveLabel})).To(BeFalse())
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
		BeforeEach(func() {
			e = elemental.NewElemental(config)
		})
		It("Gets the iso and returns the temporary where it is stored", func() {
			tmpDir, err := utils.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = fs.WriteFile(fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), cnst.FilePerm)
			Expect(err).To(BeNil())
			iso := fmt.Sprintf("%s/fake.iso", tmpDir)
			isoDir, err := e.GetIso(iso)
			Expect(err).To(BeNil())
			// Confirm that the iso is stored in isoDir
			utils.Exists(fs, filepath.Join(isoDir, "cOs.iso"))
		})
		It("Fails if it cant find the iso", func() {
			iso := "whatever"
			e := elemental.NewElemental(config)
			_, err := e.GetIso(iso)
			Expect(err).ToNot(BeNil())
		})
		It("Fails if it cannot mount the iso", func() {
			mounter.ErrorOnMount = true
			tmpDir, err := utils.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = fs.WriteFile(fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), cnst.FilePerm)
			Expect(err).To(BeNil())
			iso := fmt.Sprintf("%s/fake.iso", tmpDir)
			_, err = e.GetIso(iso)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("mount error"))
		})
	})
	Describe("UpdateSourcesFormDownloadedISO", Label("iso"), func() {
		var e *elemental.Elemental
		var activeImg, recoveryImg *v1.Image
		BeforeEach(func() {
			activeImg, recoveryImg = nil, nil
			e = elemental.NewElemental(config)
		})
		It("updates active image", func() {
			activeImg = &v1.Image{}
			err := e.UpdateSourcesFormDownloadedISO("/some/dir", activeImg, recoveryImg)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(activeImg.Source.IsDir()).To(BeTrue())
			Expect(activeImg.Source.Value()).To(Equal("/some/dir/rootfs"))
			Expect(recoveryImg).To(BeNil())
		})
		It("updates active and recovery image", func() {
			activeImg = &v1.Image{File: "activeFile"}
			recoveryImg = &v1.Image{}
			err := e.UpdateSourcesFormDownloadedISO("/some/dir", activeImg, recoveryImg)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(recoveryImg.Source.IsFile()).To(BeTrue())
			Expect(recoveryImg.Source.Value()).To(Equal("activeFile"))
			Expect(recoveryImg.Label).To(Equal(cnst.SystemLabel))
			Expect(activeImg.Source.IsDir()).To(BeTrue())
			Expect(activeImg.Source.Value()).To(Equal("/some/dir/rootfs"))
		})
		It("updates recovery only image", func() {
			recoveryImg = &v1.Image{}
			isoMnt := "/some/dir/iso"
			err := utils.MkdirAll(fs, isoMnt, cnst.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			recoverySquash := filepath.Join(isoMnt, cnst.RecoverySquashFile)
			_, err = fs.Create(recoverySquash)
			Expect(err).ShouldNot(HaveOccurred())
			err = e.UpdateSourcesFormDownloadedISO("/some/dir", activeImg, recoveryImg)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(recoveryImg.Source.IsFile()).To(BeTrue())
			Expect(recoveryImg.Source.Value()).To(Equal(recoverySquash))
			Expect(activeImg).To(BeNil())
		})
		It("fails to update recovery from active file", func() {
			recoveryImg = &v1.Image{}
			err := e.UpdateSourcesFormDownloadedISO("/some/dir", activeImg, recoveryImg)
			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("CloudConfig", Label("CloudConfig", "cloud-config"), func() {
		var e *elemental.Elemental
		BeforeEach(func() {
			e = elemental.NewElemental(config)
		})
		It("Copies the cloud config file", func() {
			testString := "In a galaxy far far away..."
			cloudInit := "/config.yaml"
			err := fs.WriteFile(cloudInit, []byte(testString), cnst.FilePerm)
			Expect(err).To(BeNil())
			Expect(err).To(BeNil())

			err = e.CopyCloudConfig(cloudInit)
			Expect(err).To(BeNil())
			copiedFile, err := fs.ReadFile(fmt.Sprintf("%s/99_custom.yaml", cnst.OEMDir))
			Expect(err).To(BeNil())
			Expect(copiedFile).To(ContainSubstring(testString))
		})
		It("Doesnt do anything if the config file is not set", func() {
			err := e.CopyCloudConfig("")
			Expect(err).To(BeNil())
		})
	})
	Describe("SetDefaultGrubEntry", Label("SetDefaultGrubEntry", "grub"), func() {
		It("Sets the default grub entry without issues", func() {
			el := elemental.NewElemental(config)
			Expect(el.SetDefaultGrubEntry("/mountpoint", "/imgMountpoint", "default_entry")).To(BeNil())
		})
		It("does nothing on empty default entry and no /etc/os-release", func() {
			el := elemental.NewElemental(config)
			Expect(el.SetDefaultGrubEntry("/mountpoint", "/imgMountPoint", "")).To(BeNil())
			// No grub2-editenv command called
			Expect(runner.CmdsMatch([][]string{{"grub2-editenv"}})).NotTo(BeNil())
		})
		It("loads /etc/os-release on empty default entry", func() {
			err := utils.MkdirAll(config.Fs, "/imgMountPoint/etc", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			err = config.Fs.WriteFile("/imgMountPoint/etc/os-release", []byte("GRUB_ENTRY_NAME=test"), constants.FilePerm)
			Expect(err).ShouldNot(HaveOccurred())

			el := elemental.NewElemental(config)
			Expect(el.SetDefaultGrubEntry("/mountpoint", "/imgMountPoint", "")).To(BeNil())
			// Calls grub2-editenv with the loaded content from /etc/os-release
			Expect(runner.CmdsMatch([][]string{
				{"grub2-editenv", "/mountpoint/grub_oem_env", "set", "default_menu_entry=test"},
			})).To(BeNil())
		})
		It("Fails setting grubenv", func() {
			runner.ReturnError = errors.New("failure")
			el := elemental.NewElemental(config)
			Expect(el.SetDefaultGrubEntry("/mountpoint", "/imgMountPoint", "default_entry")).NotTo(BeNil())
		})
	})
	Describe("FindKernelInitrd", Label("find"), func() {
		BeforeEach(func() {
			err := utils.MkdirAll(fs, "/path/boot", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("finds kernel and initrd files", func() {
			_, err := fs.Create("/path/boot/initrd")
			Expect(err).ShouldNot(HaveOccurred())

			_, err = fs.Create("/path/boot/vmlinuz")
			Expect(err).ShouldNot(HaveOccurred())

			el := elemental.NewElemental(config)
			k, i, err := el.FindKernelInitrd("/path")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(k).To(Equal("/path/boot/vmlinuz"))
			Expect(i).To(Equal("/path/boot/initrd"))
		})
		It("fails if no initrd is found", func() {
			_, err := fs.Create("/path/boot/vmlinuz")
			Expect(err).ShouldNot(HaveOccurred())

			el := elemental.NewElemental(config)
			_, _, err = el.FindKernelInitrd("/path")
			Expect(err).Should(HaveOccurred())
		})
		It("fails if no kernel is found", func() {
			_, err := fs.Create("/path/boot/initrd")
			Expect(err).ShouldNot(HaveOccurred())

			el := elemental.NewElemental(config)
			_, _, err = el.FindKernelInitrd("/path")
			Expect(err).Should(HaveOccurred())
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
func pathInMountPoints(mounter mount.Interface, path string) bool {
	mountPoints, _ := mounter.List()
	for _, m := range mountPoints {
		if path == m.Path {
			return true
		}
	}
	return false
}
