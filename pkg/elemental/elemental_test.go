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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cnst "github.com/rancher-sandbox/elemental-cli/pkg/constants"
	"github.com/rancher-sandbox/elemental-cli/pkg/elemental"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"os"
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

var _ = Describe("Elemental", func() {
	var config *v1.RunConfig
	var runner v1.Runner
	var logger v1.Logger
	var syscall v1.SyscallInterface
	var client v1.HTTPClient
	var mounter mount.Interface
	var fs afero.Fs

	BeforeEach(func() {
		runner = &v1mock.FakeRunner{}
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHttpClient{}
		logger = v1.NewNullLogger()
		fs = afero.NewMemMapFs()
		config = v1.NewRunConfig(
			v1.WithFs(fs),
			v1.WithRunner(runner),
			v1.WithLogger(logger),
			v1.WithMounter(mounter),
			v1.WithSyscall(syscall),
			v1.WithClient(client),
		)
	})

	Context("MountPartitions", func() {
		var runner *v1mock.TestRunnerV2
		BeforeEach(func() {
			runner = v1mock.NewTestRunnerV2()
			config.Runner = runner
		})

		It("Mounts disk partitions", func() {
			runner.ReturnValue = []byte("/some/device")
			el := elemental.NewElemental(config)
			err := el.MountPartitions()
			Expect(err).To(BeNil())
		})

		It("Fails if state partition resists to mount ", func() {
			runner.ReturnValue = []byte("/some/device")
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnMount = true
			el := elemental.NewElemental(config)
			err := el.MountPartitions()
			Expect(err).NotTo(BeNil())
		})

		It("Fails if oem partition is not found ", func() {
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if len(args) >= 2 && args[1] == fmt.Sprintf("LABEL=%s", config.OEMPart.Label) {
					return []byte{}, nil
				}
				return []byte("/some/device"), nil
			}
			el := elemental.NewElemental(config)
			err := el.MountPartitions()
			Expect(err).NotTo(BeNil())
		})

		It("Fails if recovery partition is not found ", func() {
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if len(args) >= 2 && args[1] == fmt.Sprintf("LABEL=%s", config.RecoveryPart.Label) {
					return []byte{}, nil
				}
				return []byte("/some/device"), nil
			}
			el := elemental.NewElemental(config)
			err := el.MountPartitions()
			Expect(err).NotTo(BeNil())
		})
	})

	Context("UnmountPartitions", func() {
		var runner *v1mock.TestRunnerV2
		var el *elemental.Elemental
		BeforeEach(func() {
			runner = v1mock.NewTestRunnerV2()
			config.Runner = runner
			runner.ReturnValue = []byte("/some/device")
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

	Context("MountImage", func() {
		var runner *v1mock.TestRunnerV2
		var el *elemental.Elemental
		BeforeEach(func() {
			runner = v1mock.NewTestRunnerV2()
			config.Runner = runner
			el = elemental.NewElemental(config)
			config.ActiveImage.MountPoint = "/some/mountpoint"
		})

		It("Mounts file system image", func() {
			runner.ReturnValue = []byte("/dev/loop")
			Expect(el.MountImage(&config.ActiveImage)).To(BeNil())
			Expect(config.ActiveImage.LoopDevice).To(Equal("/dev/loop"))
		})

		It("Fails to set a loop device", func() {
			runner.ReturnError = errors.New("failed to set a loop device")
			Expect(el.MountImage(&config.ActiveImage)).NotTo(BeNil())
			Expect(config.ActiveImage.LoopDevice).To(Equal(""))
		})

		It("Fails to mount a loop device", func() {
			runner.ReturnValue = []byte("/dev/loop")
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnMount = true
			Expect(el.MountImage(&config.ActiveImage)).NotTo(BeNil())
			Expect(config.ActiveImage.LoopDevice).To(Equal(""))
		})
	})

	Context("UnmountImage", func() {
		var runner *v1mock.TestRunnerV2
		var el *elemental.Elemental
		BeforeEach(func() {
			runner = v1mock.NewTestRunnerV2()
			runner.ReturnValue = []byte("/dev/loop")
			config.Runner = runner
			el = elemental.NewElemental(config)
			config.ActiveImage.MountPoint = "/some/mountpoint"
			Expect(el.MountImage(&config.ActiveImage)).To(BeNil())
			Expect(config.ActiveImage.LoopDevice).To(Equal("/dev/loop"))
		})

		It("Unmounts file system image", func() {
			Expect(el.UnmountImage(&config.ActiveImage)).To(BeNil())
			Expect(config.ActiveImage.LoopDevice).To(Equal(""))
		})

		It("Fails to unmount a mountpoint", func() {
			mounter := mounter.(*v1mock.ErrorMounter)
			mounter.ErrorOnUnmount = true
			Expect(el.UnmountImage(&config.ActiveImage)).NotTo(BeNil())
		})

		It("Fails to unset a loop device", func() {
			runner.ReturnError = errors.New("failed to unset a loop device")
			Expect(el.UnmountImage(&config.ActiveImage)).NotTo(BeNil())
		})
	})

	Context("CreateFileSystemImage", func() {
		var el *elemental.Elemental
		BeforeEach(func() {
			el = elemental.NewElemental(config)
			config.ActiveImage.Size = 32
		})

		It("Creates a new file system image", func() {
			_, err := fs.Stat(config.ActiveImage.File)
			Expect(err).NotTo(BeNil())
			err = el.CreateFileSystemImage(config.ActiveImage)
			Expect(err).To(BeNil())
			stat, err := fs.Stat(config.ActiveImage.File)
			Expect(err).To(BeNil())
			Expect(stat.Size()).To(Equal(int64(32 * 1024 * 1024)))
		})

		It("Fails formatting a file system image", func() {
			runner := runner.(*v1mock.FakeRunner)
			runner.ErrorOnCommand = true
			_, err := fs.Stat(config.ActiveImage.File)
			Expect(err).NotTo(BeNil())
			err = el.CreateFileSystemImage(config.ActiveImage)
			Expect(err).NotTo(BeNil())
			_, err = fs.Stat(config.ActiveImage.File)
			Expect(err).NotTo(BeNil())
		})
	})

	Context("PartitionAndFormatDevice", func() {
		var el *elemental.Elemental
		var dev *part.Disk
		var runner *v1mock.TestRunnerV2
		var cInit *v1mock.FakeCloudInitRunner
		var partNum, devNum, errPart int
		var printOut, errFormat string

		BeforeEach(func() {
			runner = v1mock.NewTestRunnerV2()
			cInit = &v1mock.FakeCloudInitRunner{ExecStages: []string{}, Error: false}
			config.Runner = runner
			config.CloudInitRunner = cInit
			fs.Create(cnst.EfiDevice)
			config.SetupStyle()
			el = elemental.NewElemental(config)
			dev = part.NewDisk(
				"/some/device",
				part.WithRunner(runner),
				part.WithFS(fs),
				part.WithLogger(logger),
			)
		})

		Context("Successful run", func() {
			var runFunc func(cmd string, args ...string) ([]byte, error)
			var initCmds, partCmds [][]string
			BeforeEach(func() {
				initCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mklabel", "gpt",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.grub", "vfat", "2048", "133119", "set", "1", "esp", "on",
					}, {"mkfs.vfat", "-i", "COS_GRUB", "/some/device1"},
				}
				partCmds = [][]string{
					{
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.oem", "ext4", "133120", "264191",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.state", "ext4", "264192", "31721471",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.recovery", "ext4", "31721472", "48498687",
					}, {
						"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
						"mkpart", "p.persistent", "ext4", "48498688", "100%",
					}, {"mkfs.ext4", "-L", "COS_OEM", "/some/device2"},
					{"mkfs.ext4", "-L", "COS_STATE", "/some/device3"},
					{"mkfs.ext4", "-L", "COS_RECOVERY", "/some/device4"},
					{"mkfs.ext4", "-L", "COS_PERSISTENT", "/some/device5"},
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
						devNum++
						return []byte(fmt.Sprintf("/some/device%d part", devNum)), nil
					default:
						return []byte{}, nil
					}
				}
				runner.SideEffect = runFunc
			})

			It("Successfully creates partitions and formats them", func() {
				partNum, devNum, printOut = 0, 0, printOutput
				err := el.PartitionAndFormatDevice(dev)
				Expect(err).To(BeNil())
				Expect(runner.MatchMilestones(append(initCmds, partCmds...))).To(BeNil())
			})

			It("Successfully creates boot partitions and runs 'partitioning' stage", func() {
				config.PartLayout = "partitioning.yaml"
				partNum, devNum, printOut = 0, 0, printOutput
				err := el.PartitionAndFormatDevice(dev)
				Expect(err).To(BeNil())
				Expect(runner.MatchMilestones(initCmds)).To(BeNil())
				Expect(len(cInit.ExecStages)).To(Equal(1))
				Expect(cInit.ExecStages[0]).To(Equal("partitioning"))
			})
		})

		Context("Run with failures", func() {
			var runFunc func(cmd string, args ...string) ([]byte, error)
			BeforeEach(func() {
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
						devNum++
						return []byte(fmt.Sprintf("/some/device%d part", devNum)), nil
					case "mkfs.ext4", "mkfs.vfat":
						if args[1] == errFormat {
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
				errPart, partNum, devNum, errFormat, printOut = 1, 0, 0, "COS_GRUB", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails formatting efi partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_GRUB", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(devNum).To(Equal(1))
			})

			It("Fails creating oem partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 2, 0, 0, "", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails creating state partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 3, 0, 0, "", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails creating recovery partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 4, 0, 0, "", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails creating persistent partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 5, 0, 0, "", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(partNum).To(Equal(errPart))
			})

			It("Fails formatting oem partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_OEM", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(devNum).To(Equal(2))
			})

			It("Fails formatting state partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_STATE", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(devNum).To(Equal(3))
			})

			It("Fails formatting recovery partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_RECOVERY", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(devNum).To(Equal(4))
			})

			It("Fails formatting persistent partition", func() {
				errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_PERSISTENT", printOutput
				Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
				Expect(devNum).To(Equal(5))
			})
		})
	})

	Context("DoCopy", func() {
		It("Copies all files from source to target", func() {
			sourceDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(sourceDir)
			destDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(destDir)
			config.ActiveImage.RootTree = sourceDir
			config.ActiveImage.MountPoint = destDir
			c := elemental.NewElemental(config)
			err = c.CopyCos()
			Expect(c.CopyCos()).To(BeNil())
		})
		It("should fail if source does not exist", func() {
			config.ActiveImage.RootTree = "/welp"
			c := elemental.NewElemental(config)
			err := c.CopyCos()
			Expect(err).ToNot(BeNil())
		})
	})
	Context("NoFormat", func() {
		Context("is disabled", func() {
			It("Should not error out", func() {
				c := elemental.NewElemental(config)
				err := c.CheckNoFormat()
				Expect(err).To(BeNil())
			})
		})
		Context("is enabled", func() {
			Context("Labels exist", func() {
				Context("Force is disabled", func() {
					It("Should error out", func() {
						config.NoFormat = true
						runner := v1mock.NewTestRunnerV2()
						runner.ReturnValue = []byte("/dev/fake")
						config.Runner = runner
						e := elemental.NewElemental(config)
						err := e.CheckNoFormat()
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(ContainSubstring("There is already an active deployment"))
					})
				})
				Context("Force is enabled", func() {
					It("Should not error out", func() {
						config.NoFormat = true
						config.Force = true
						runner := v1mock.NewTestRunnerV2()
						runner.ReturnValue = []byte("/dev/fake")
						config.Runner = runner
						e := elemental.NewElemental(config)
						err := e.CheckNoFormat()
						Expect(err).To(BeNil())
					})
				})
			})
			Context("Labels dont exist", func() {
				It("Should not error out", func() {
					config.NoFormat = true
					runner := v1mock.NewTestRunnerV2()
					runner.ReturnValue = []byte("")
					config.Runner = runner
					e := elemental.NewElemental(config)
					err := e.CheckNoFormat()
					Expect(err).To(BeNil())
				})
			})
		})
	})
	Context("SelinuxRelabel", func() {
		It("Works", func() {
			c := elemental.NewElemental(config)
			// This is actually failing but not sure we should return an error
			Expect(c.SelinuxRelabel(true)).ToNot(BeNil())
			fs = afero.NewMemMapFs()
			_, _ = fs.Create("/etc/selinux/targeted/contexts/files/file_contexts")
			Expect(c.SelinuxRelabel(false)).To(BeNil())
		})
	})
	Context("BootedFromSquash", func() {
		It("Returns true if booted from squashfs", func() {
			runner := v1mock.NewTestRunnerV2()
			runner.ReturnValue = []byte(cnst.RecoveryLabel)
			config.Runner = runner
			e := elemental.NewElemental(config)
			Expect(e.BootedFromSquash()).To(BeTrue())
		})
		It("Returns false if not booted from squashfs", func() {
			e := elemental.NewElemental(config)
			Expect(e.BootedFromSquash()).To(BeFalse())
		})
	})
	Context("GetIso", func() {
		It("Does nothing if iso is not set", func() {
			e := elemental.NewElemental(config)
			Expect(e.GetIso()).To(BeNil())
		})
		It("Modifies the IsoMnt var to point to the mounted iso", func() {
			Expect(config.IsoMnt).To(Equal(cnst.IsoMnt))
			tmpDir, err := afero.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = afero.WriteFile(fs, fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), os.ModePerm)
			Expect(err).To(BeNil())

			config.Iso = fmt.Sprintf("%s/fake.iso", tmpDir)
			e := elemental.NewElemental(config)
			Expect(e.GetIso()).To(BeNil())
			// Confirm that the isomnt value was set to a new path
			Expect(config.IsoMnt).ToNot(Equal(cnst.IsoMnt))
			// Confirm that we tried to mount it properly
			Expect(pathInMountPoints(mounter, config.IsoMnt)).To(BeTrue())

		})
		It("Fails if it cant find the iso", func() {
			config.Iso = "whatever"
			e := elemental.NewElemental(config)
			Expect(e.GetIso()).ToNot(BeNil())

		})
		It("Fails if it cannot mount the iso", func() {
			config.Mounter = v1mock.ErrorMounter{ErrorOnMount: true}
			Expect(config.IsoMnt).To(Equal(cnst.IsoMnt))
			tmpDir, err := afero.TempDir(fs, "", "elemental-test")
			Expect(err).To(BeNil())
			err = afero.WriteFile(fs, fmt.Sprintf("%s/fake.iso", tmpDir), []byte("Hi"), os.ModePerm)
			Expect(err).To(BeNil())

			config.Iso = fmt.Sprintf("%s/fake.iso", tmpDir)
			e := elemental.NewElemental(config)
			Expect(e.GetIso()).ToNot(BeNil())
			Expect(e.GetIso().Error()).To(ContainSubstring("mount error"))
		})
	})
	Context("CloudConfig", func() {
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
	Context("GetUrl", func() {
		It("Gets an http url", func() {
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
	Context("CopyRecovery", func() {
		Context("Squashfs", func() {
			Context(fmt.Sprintf("squash file %s exists", cnst.RecoverySquashFile), func() {
				It("should copy squash file", func() {
					runner := v1mock.NewTestRunnerV2()
					runner.ReturnValue = []byte(cnst.RecoveryLabel)
					config.Runner = runner
					// Create recovery.squashfs file
					squashfile := fmt.Sprintf("%s/%s", config.IsoMnt, cnst.RecoverySquashFile)
					_, _ = config.Fs.Create(squashfile)
					e := elemental.NewElemental(config)
					Expect(e.CopyRecovery()).To(BeNil())
					// Target should be there
					exists, err := afero.Exists(fs, fmt.Sprintf("%s/cOS/%s", cnst.RecoveryDir, cnst.RecoverySquashFile))
					Expect(exists).To(BeTrue())
					Expect(err).To(BeNil())
				})
			})
			Context(fmt.Sprintf("squash file %s does not exists", cnst.RecoverySquashFile), func() {
				It(fmt.Sprintf("should copy img file %s", cnst.ActiveImgFile), func() {
					runner := v1mock.NewTestRunnerV2()
					runner.ReturnValue = []byte(cnst.RecoveryLabel)
					config.Runner = runner
					// Create active image file
					imgfile := fmt.Sprintf("%s/cOS/%s", cnst.StateDir, config.ActiveImage.File)
					_, _ = config.Fs.Create(imgfile)
					e := elemental.NewElemental(config)
					Expect(e.CopyRecovery()).To(BeNil())
					// Target should be there
					exists, err := afero.Exists(fs, fmt.Sprintf("%s/cOS/%s", cnst.RecoveryDir, cnst.RecoveryImgFile))
					Expect(exists).To(BeTrue())
					Expect(err).To(BeNil())
				})
			})

		})
		Context("Non-Squashfs", func() {
			It("should not do anything", func() {
				runner := v1mock.NewTestRunnerV2()
				runner.ReturnValue = []byte("")
				config.Runner = runner
				e := elemental.NewElemental(config)
				Expect(e.CopyRecovery()).To(BeNil())
				// Target file should not be there
				exists, err := afero.Exists(fs, fmt.Sprintf("%s/cOS/%s", cnst.RecoveryDir, cnst.RecoverySquashFile))
				Expect(exists).To(BeFalse())
				Expect(err).To(BeNil())
			})
		})
	})

	Context("CopyPassive", func() {
		var actImgFile, passImgFile string
		BeforeEach(func() {
			actImgFile = fmt.Sprintf("%s/cOS/%s", cnst.StateDir, config.ActiveImage.File)
			passImgFile = fmt.Sprintf("%s/cOS/%s", cnst.StateDir, cnst.PassiveImgFile)
		})

		It("Copies active image to passive", func() {
			_, err := fs.Create(actImgFile)
			Expect(err).To(BeNil())
			_, err = fs.Stat(passImgFile)
			Expect(err).NotTo(BeNil())
			el := elemental.NewElemental(config)
			Expect(el.CopyPassive()).To(BeNil())
			_, err = fs.Stat(passImgFile)
			Expect(err).To(BeNil())
		})

		It("Fails to copy, active file is not present", func() {
			_, err := fs.Stat(passImgFile)
			Expect(err).NotTo(BeNil())
			el := elemental.NewElemental(config)
			Expect(el.CopyPassive()).NotTo(BeNil())
		})

		It("Fails to set the passive label", func() {
			runner := runner.(*v1mock.FakeRunner)
			runner.ErrorOnCommand = true
			_, err := fs.Create(actImgFile)
			Expect(err).To(BeNil())
			_, err = fs.Stat(passImgFile)
			Expect(err).NotTo(BeNil())
			el := elemental.NewElemental(config)
			Expect(el.CopyPassive()).NotTo(BeNil())
			_, err = fs.Stat(passImgFile)
			Expect(err).NotTo(BeNil())
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
