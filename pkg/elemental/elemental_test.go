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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/action"
	conf "github.com/rancher-sandbox/elemental/pkg/config"
	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/elemental"
	part "github.com/rancher-sandbox/elemental/pkg/partitioner"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental/tests/mocks"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"os"
	"path/filepath"
	"testing"
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
	var config *v1.RunConfig
	var runner *v1mock.FakeRunner
	var logger v1.Logger
	var syscall v1.SyscallInterface
	var client v1.HTTPClient
	var mounter mount.Interface
	var fs afero.Fs

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHttpClient{}
		logger = v1.NewNullLogger()
		fs = afero.NewMemMapFs()
		config = conf.NewRunConfig(
			v1.WithFs(fs),
			v1.WithRunner(runner),
			v1.WithLogger(logger),
			v1.WithMounter(mounter),
			v1.WithSyscall(syscall),
			v1.WithClient(client),
		)
	})

	Describe("MountPartitions", Label("MountPartitions", "disk", "partition", "mount"), func() {
		var el *elemental.Elemental
		BeforeEach(func() {
			fs.Create(cnst.EfiDevice)
			action.InstallSetup(config)
			Expect(config.PartTable).To(Equal(v1.GPT))
			Expect(config.BootFlag).To(Equal(v1.ESP))
			el = elemental.NewElemental(config)
		})

		It("Mounts disk partitions", func() {
			runner.ReturnValue = []byte("/some/device")
			err := el.MountPartitions()
			Expect(err).To(BeNil())
		})

		It("Fails if some partition resists to mount ", func() {
			runner.ReturnValue = []byte("/some/device")
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnMount = true
			err := el.MountPartitions()
			Expect(err).NotTo(BeNil())
		})

		It("Fails if oem partition is not found ", func() {
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if len(args) >= 2 && args[1] == cnst.OEMLabel {
					return []byte{}, nil
				}
				return []byte("/some/device"), nil
			}
			err := el.MountPartitions()
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("UnmountPartitions", Label("UnmountPartitions", "disk", "partition", "unmount"), func() {
		var el *elemental.Elemental
		BeforeEach(func() {
			runner.ReturnValue = []byte("/some/device")
			fs.Create(cnst.EfiDevice)
			action.InstallSetup(config)
			Expect(config.PartTable).To(Equal(v1.GPT))
			Expect(config.BootFlag).To(Equal(v1.ESP))
			el = elemental.NewElemental(config)
			Expect(el.MountPartitions()).To(BeNil())
		})

		It("Unmounts disk partitions", func() {
			err := el.UnmountPartitions()
			Expect(err).To(BeNil())
		})

		It("Fails to unmount disk partitions", func() {
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnUnmount = true
			err := el.UnmountPartitions()
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
			mounter := mounter.(*v1mock.ErrorMounter)
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
			mounter := mounter.(*v1mock.ErrorMounter)
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
		BeforeEach(func() {
			action.InstallSetup(config)
			el = elemental.NewElemental(config)
			config.Images.GetActive().Size = 32
		})

		It("Creates a new file system image", func() {
			_, err := fs.Stat(config.Images.GetActive().File)
			Expect(err).NotTo(BeNil())
			err = el.CreateFileSystemImage(config.Images.GetActive())
			Expect(err).To(BeNil())
			stat, err := fs.Stat(config.Images.GetActive().File)
			Expect(err).To(BeNil())
			Expect(stat.Size()).To(Equal(int64(32 * 1024 * 1024)))
		})

		It("Fails formatting a file system image", Label("format"), func() {
			runner.ReturnError = errors.New("run error")
			_, err := fs.Stat(config.Images.GetActive().File)
			Expect(err).NotTo(BeNil())
			err = el.CreateFileSystemImage(config.Images.GetActive())
			Expect(err).NotTo(BeNil())
			_, err = fs.Stat(config.Images.GetActive().File)
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("FormatPartition", Label("FormatPartition", "partition", "format"), func() {
		It("Reformats an already existing partition", func() {
			el := elemental.NewElemental(config)
			part := &v1.Partition{
				Path:  "/dev/device1",
				FS:    "ext4",
				Label: "MY_LABEL",
			}
			Expect(el.FormatPartition(part)).To(BeNil())
		})

	})
	Describe("PartitionAndFormatDevice", Label("PartitionAndFormatDevice", "partition", "format"), func() {
		var el *elemental.Elemental
		var dev *part.Disk
		var cInit *v1mock.FakeCloudInitRunner
		var partNum, errPart int
		var printOut string
		var failEfiFormat bool

		BeforeEach(func() {
			cInit = &v1mock.FakeCloudInitRunner{ExecStages: []string{}, Error: false}
			config.CloudInitRunner = cInit
			fs.Create(cnst.EfiDevice)
			el = elemental.NewElemental(config)
			dev = part.NewDisk(
				"/some/device",
				part.WithRunner(runner),
				part.WithFS(fs),
				part.WithLogger(logger),
			)
		})

		Describe("Successful run", func() {
			var runFunc func(cmd string, args ...string) ([]byte, error)
			var efiPartCmds, partCmds, biosPartCmds [][]string
			BeforeEach(func() {
				partNum, printOut = 0, printOutput
				efiPartCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mklabel", "gpt",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.grub", "fat32", "2048", "133119", "set", "1", "esp", "on",
					}, {"mkfs.vfat", "-n", "COS_GRUB", "/some/device1"},
				}
				biosPartCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mklabel", "gpt",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.bios", "", "2048", "4095", "set", "1", "bios_grub", "on",
					}, {"wipefs", "--all", "/some/device1"},
				}
				// These commands are only valid for EFI case
				partCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.oem", "ext4", "133120", "264191",
					}, {"mkfs.ext4", "-L", "COS_OEM", "/some/device2"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.state", "ext4", "264192", "31721471",
					}, {"mkfs.ext4", "-L", "COS_STATE", "/some/device3"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.recovery", "ext4", "31721472", "48498687",
					}, {"mkfs.ext4", "-L", "COS_RECOVERY", "/some/device4"}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.persistent", "ext4", "48498688", "100%",
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
						}
						return []byte(printOut), nil
					case "lsblk":
						return []byte(fmt.Sprintf("/some/device%d part", partNum)), nil
					default:
						return []byte{}, nil
					}
				}
				runner.SideEffect = runFunc
			})

			It("Successfully creates partitions and formats them, EFI boot", func() {
				action.InstallSetup(config)
				Expect(el.PartitionAndFormatDevice(dev)).To(BeNil())
				Expect(runner.MatchMilestones(append(efiPartCmds, partCmds...))).To(BeNil())
			})

			It("Successfully creates partitions and formats them, BIOS boot", func() {
				config.ForceGpt = true
				fs.Remove(cnst.EfiDevice)
				action.InstallSetup(config)
				el = elemental.NewElemental(config)
				Expect(el.PartitionAndFormatDevice(dev)).To(BeNil())
				Expect(runner.MatchMilestones(biosPartCmds)).To(BeNil())
			})

			It("Successfully creates boot partitions and runs 'partitioning' stage", func() {
				action.InstallSetup(config)
				config.PartLayout = "partitioning.yaml"
				err := el.PartitionAndFormatDevice(dev)
				Expect(err).To(BeNil())
				Expect(runner.MatchMilestones(efiPartCmds)).To(BeNil())
				Expect(len(cInit.ExecStages)).To(Equal(1))
				Expect(cInit.ExecStages[0]).To(Equal("partitioning"))
			})
		})

		Describe("Run with failures", func() {
			var runFunc func(cmd string, args ...string) ([]byte, error)
			BeforeEach(func() {
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
							if errPart == partNum {
								return []byte{}, errors.New("Failure")
							}
						}
						return []byte(printOut), nil
					case "lsblk":
						return []byte(fmt.Sprintf("/some/device%d part", partNum)), nil
					case "mkfs.ext4", "wipefs":
						return []byte{}, errors.New("Failure")
					case "mkfs.vfat":
						if failEfiFormat {
							return []byte{}, errors.New("Failure")
						}
						return []byte{}, nil
					default:
						return []byte{}, nil
					}
				}
				runner.SideEffect = runFunc
			})

			It("Fails creating efi partition", func() {
				action.InstallSetup(config)
				errPart, failEfiFormat = 1, false
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails formatting efi partition", func() {
				action.InstallSetup(config)
				errPart, failEfiFormat = 0, true
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(1))
			})

			It("Fails creating bios partition", func() {
				config.ForceGpt = true
				fs.Remove(cnst.EfiDevice)
				action.InstallSetup(config)
				el = elemental.NewElemental(config)
				errPart, failEfiFormat = 1, false
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails clearing filesystem on bios partition", func() {
				config.ForceGpt = true
				fs.Remove(cnst.EfiDevice)
				action.InstallSetup(config)
				el = elemental.NewElemental(config)
				errPart, failEfiFormat = 0, false
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(1))
			})

			It("Fails creating a data partition", func() {
				action.InstallSetup(config)
				errPart, failEfiFormat = 2, false
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails formatting a data partition", func() {
				action.InstallSetup(config)
				errPart, failEfiFormat = 0, false
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(2))
			})
		})
	})
	Describe("DeployImage", Label("DeployImage"), func() {
		var el *elemental.Elemental
		var img *v1.Image
		var destDir, sourceDir, cmdFail string
		BeforeEach(func() {
			sourceDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			destDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			el = elemental.NewElemental(config)
			img = &v1.Image{
				FS:         "ext2",
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
		AfterEach(func() {
			os.RemoveAll(sourceDir)
			os.RemoveAll(destDir)
		})
		It("Deploys an image from a directory and leaves it mounted", func() {
			Expect(el.DeployImage(img, true)).To(BeNil())
		})
		It("Deploys an image from a directory and leaves it unmounted", func() {
			Expect(el.DeployImage(img, false)).To(BeNil())
		})
		It("Deploys a file image and mounts it", func() {
			sourceImg := "source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destDir, err := afero.TempDir(fs, "", "elemental")
			Expect(err).To(BeNil())
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			Expect(el.DeployImage(img, true)).To(BeNil())
		})
		It("Deploys a file image and fails to mount it", func() {
			sourceImg := "source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destDir, err := afero.TempDir(fs, "", "elemental")
			Expect(err).To(BeNil())
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnMount = true
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails formatting the image", func() {
			cmdFail = "mkfs.ext2"
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails mounting the image", func() {
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnMount = true
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails copying the image if source does not exist", func() {
			img.Source = v1.NewDirSrc("/welp")
			Expect(el.DeployImage(img, true)).NotTo(BeNil())
		})
		It("Fails unmounting the image after copying", func() {
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnUnmount = true
			Expect(el.DeployImage(img, false)).NotTo(BeNil())
		})
	})
	Describe("CopyImage", Label("CopyImage"), func() {
		var img *v1.Image
		BeforeEach(func() {
			img = &v1.Image{}
		})
		It("Copies files from a directory source", func() {
			sourceDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(sourceDir)
			destDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(destDir)
			img.Source = v1.NewDirSrc(sourceDir)
			img.MountPoint = destDir
			c := elemental.NewElemental(config)
			Expect(c.CopyImage(img)).To(BeNil())
		})
		It("Fails if source directory does not exist", func() {
			c := elemental.NewElemental(config)
			img.Source = v1.NewDirSrc("/welp")
			Expect(c.CopyImage(img)).ToNot(BeNil())
		})
		It("Unpacks a docker image to target", Label("docker"), func() {
			luet := v1mock.NewFakeLuet()
			config.Luet = luet
			c := elemental.NewElemental(config)
			img.Source = v1.NewDockerSrc("docker/image:latest")
			Expect(c.CopyImage(img)).To(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})
		It("Unpacks a docker image to target with cosign validation", Label("docker", "cosign"), func() {
			config.Cosign = true
			luet := v1mock.NewFakeLuet()
			config.Luet = luet
			c := elemental.NewElemental(config)
			img.Source = v1.NewDockerSrc("docker/image:latest")
			Expect(c.CopyImage(img)).To(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
			Expect(runner.CmdsMatch([][]string{{"cosign", "verify", "docker/image:latest"}}))
		})
		It("Fails cosign validation", Label("cosign"), func() {
			runner.ReturnError = errors.New("cosign error")
			config.Cosign = true
			c := elemental.NewElemental(config)
			img.Source = v1.NewDockerSrc("docker/image:latest")
			Expect(c.CopyImage(img)).NotTo(BeNil())
			Expect(runner.CmdsMatch([][]string{{"cosign", "verify", "docker/image:latest"}}))
		})
		It("Fails to unpack a docker image to target", Label("docker"), func() {
			luet := v1mock.NewFakeLuet()
			luet.OnUnpackError = true
			config.Luet = luet
			c := elemental.NewElemental(config)
			img.Source = v1.NewDockerSrc("docker/image:latest")
			Expect(c.CopyImage(img)).NotTo(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})
		It("Copies image file to target", func() {
			sourceImg := "source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destDir, err := afero.TempDir(fs, "", "elemental")
			Expect(err).To(BeNil())
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			img.File = filepath.Join(destDir, "active.img")
			c := elemental.NewElemental(config)
			_, err = fs.Stat(img.File)
			Expect(err).NotTo(BeNil())
			Expect(c.CopyImage(img)).To(BeNil())
			_, err = fs.Stat(img.File)
			Expect(err).To(BeNil())
		})
		It("Fails to copy, source file is not present", func() {
			sourceImg := "source.img"
			destDir := "whatever"
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			c := elemental.NewElemental(config)
			Expect(c.CopyImage(img)).NotTo(BeNil())
		})
		It("Fails to set the label", Label("fails"), func() {
			runner.ReturnError = errors.New("run error")
			sourceImg := "source.img"
			_, err := fs.Create(sourceImg)
			Expect(err).To(BeNil())
			destDir, err := afero.TempDir(fs, "", "elemental")
			Expect(err).To(BeNil())
			img.Source = v1.NewFileSrc(sourceImg)
			img.MountPoint = destDir
			img.Label = "some_label"
			img.FS = cnst.LinuxFs
			img.File = filepath.Join(destDir, "active.img")
			el := elemental.NewElemental(config)
			Expect(el.CopyImage(img)).NotTo(BeNil())
		})
		It("Unpacks from channel to target", func() {
			luet := v1mock.NewFakeLuet()
			config.Luet = luet
			c := elemental.NewElemental(config)
			img.Source = v1.NewChannelSrc("somechannel")
			Expect(c.CopyImage(img)).To(BeNil())
			Expect(luet.UnpackChannelCalled()).To(BeTrue())
		})
		It("Fails to unpack from channel to target", func() {
			luet := v1mock.NewFakeLuet()
			luet.OnUnpackFromChannelError = true
			config.Luet = luet
			c := elemental.NewElemental(config)
			img.Source = v1.NewChannelSrc("somechannel")
			Expect(c.CopyImage(img)).NotTo(BeNil())
			Expect(luet.UnpackChannelCalled()).To(BeTrue())
		})
	})
	Describe("CheckNoFormat", Label("NoFormat", "format"), func() {
		BeforeEach(func() {
			config.Images.SetActive(&v1.Image{})
		})
		Describe("Labels exist", func() {
			Describe("Force is disabled", func() {
				It("Should error out", func() {
					config.NoFormat = true
					runner.ReturnValue = []byte("/dev/fake")
					e := elemental.NewElemental(config)
					err := e.CheckNoFormat()
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(ContainSubstring("There is already an active deployment"))
				})
			})
			Describe("Force is enabled", func() {
				It("Should not error out", func() {
					config.NoFormat = true
					config.Force = true
					runner.ReturnValue = []byte("/dev/fake")
					e := elemental.NewElemental(config)
					err := e.CheckNoFormat()
					Expect(err).To(BeNil())
				})
			})
		})
		Describe("Labels dont exist", func() {
			It("Should not error out", func() {
				config.NoFormat = true
				runner.ReturnValue = []byte("")
				e := elemental.NewElemental(config)
				err := e.CheckNoFormat()
				Expect(err).To(BeNil())
			})
		})
	})
	Describe("SelinuxRelabel", Label("SelinuxRelabel", "selinux"), func() {
		It("Works", func() {
			c := elemental.NewElemental(config)
			// This is actually failing but not sure we should return an error
			Expect(c.SelinuxRelabel("/", true)).ToNot(BeNil())
			fs = afero.NewMemMapFs()
			_, _ = fs.Create("/etc/selinux/targeted/contexts/files/file_contexts")
			Expect(c.SelinuxRelabel("/", false)).To(BeNil())
		})
	})
	Describe("GetIso", Label("GetIso", "iso"), func() {
		It("Gets the iso and returns the temporary where it is stored", func() {
			tmpDir, err := afero.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = afero.WriteFile(fs, fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), os.ModePerm)
			Expect(err).To(BeNil())

			config.Iso = fmt.Sprintf("%s/fake.iso", tmpDir)
			e := elemental.NewElemental(config)
			isoDir, err := e.GetIso()
			Expect(err).To(BeNil())
			// Confirm that the iso is stored in isoDir
			afero.Exists(fs, filepath.Join(isoDir, "cOs.iso"))
			// Confirm that we tried to mount it properly
			Expect(pathInMountPoints(mounter, cnst.DownloadedIsoMnt)).To(BeTrue())

		})
		It("Fails if it cant find the iso", func() {
			config.Iso = "whatever"
			e := elemental.NewElemental(config)
			_, err := e.GetIso()
			Expect(err).ToNot(BeNil())

		})
		It("Fails if it cannot mount the iso", func() {
			config.Mounter = v1mock.ErrorMounter{ErrorOnMount: true}
			tmpDir, err := afero.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = afero.WriteFile(fs, fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), os.ModePerm)
			Expect(err).To(BeNil())

			config.Iso = fmt.Sprintf("%s/fake.iso", tmpDir)
			e := elemental.NewElemental(config)
			_, err = e.GetIso()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("mount error"))
		})
	})
	Describe("CloudConfig", Label("CloudConfig", "cloud-config"), func() {
		It("Copies the cloud config file", func() {
			testString := "In a galaxy far far away..."
			err := afero.WriteFile(fs, "config.yaml", []byte(testString), os.ModePerm)
			Expect(err).To(BeNil())
			Expect(err).To(BeNil())
			config.CloudInit = "config.yaml"
			e := elemental.NewElemental(config)
			err = e.CopyCloudConfig()
			Expect(err).To(BeNil())
			copiedFile, err := afero.ReadFile(fs, fmt.Sprintf("%s/99_custom.yaml", cnst.OEMDir))
			Expect(err).To(BeNil())
			Expect(copiedFile).To(ContainSubstring(testString))
		})
		It("Doesnt do anything if the config file is not set", func() {
			e := elemental.NewElemental(config)
			err := e.CopyCloudConfig()
			Expect(err).To(BeNil())
		})
	})
	Describe("GetUrl", Label("GetUrl"), func() {
		It("Gets an http url", Label("http"), func() {
			e := elemental.NewElemental(config)
			Expect(e.GetUrl("http://fake.com/file.txt", "/file.txt")).To(BeNil())
			exists, err := afero.Exists(fs, "/file.txt")
			Expect(err).To(BeNil())
			Expect(exists).To(BeTrue())
		})
		It("Gets a file url", func() {
			e := elemental.NewElemental(config)
			Expect(afero.WriteFile(fs, "file1.txt", []byte("welcome to the jungle"), os.ModePerm)).To(BeNil())
			Expect(e.GetUrl("file1.txt", "/file.txt")).To(BeNil())
			exists, err := afero.Exists(fs, "/file.txt")
			Expect(err).To(BeNil())
			Expect(exists).To(BeTrue())
		})
	})
	Describe("SetDefaultGrubEntry", Label("SetDefaultGrubEntry", "grub"), func() {
		It("Sets the default grub entry without issues", func() {
			config.Partitions = append(config.Partitions, &v1.Partition{Name: cnst.StatePartName})
			el := elemental.NewElemental(config)
			Expect(el.SetDefaultGrubEntry()).To(BeNil())
		})
		It("Fails if state partition is not found", func() {
			el := elemental.NewElemental(config)
			Expect(el.SetDefaultGrubEntry()).NotTo(BeNil())
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
