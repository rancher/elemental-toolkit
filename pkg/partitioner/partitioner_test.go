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

package partitioner_test

import (
	"errors"
	"testing"

	"github.com/jaypipes/ghw/pkg/block"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	mocks "github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	part "github.com/rancher/elemental-toolkit/v2/pkg/partitioner"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

const partedPrint = `BYT;
/dev/loop0:50593792s:loopback:512:512:msdos:Loopback device:;
1:2048s:98303s:96256s:ext4::type=83;
2:98304s:29394943s:29296640s:ext4::boot, type=83;
3:29394944s:45019135s:15624192s:ext4::type=83;
4:45019136s:50331647s:5312512s:ext4::type=83;`

const sgdiskPrint = `Disk /dev/sda: 500118192 sectors, 238.5 GiB
Logical sector size: 512 bytes
Disk identifier (GUID): CE4AA9A2-59DF-4DCC-B55A-A27A80676B33
Partition table holds up to 128 entries
First usable sector is 34, last usable sector is 500118158
Partitions will be aligned on 2048-sector boundaries
Total free space is 2014 sectors (1007.0 KiB)

Number  Start (sector)    End (sector)  Size       Code  Name
   1            2048          526335   256.0 MiB   EF00
   2          526336        17303551   8.0 GiB     8200  
   3        17303552       500118158   230.2 GiB   8300  `

func TestElementalSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Partitioner test suite")
}

var _ = Describe("Partitioner", Label("disk", "partition", "partitioner"), func() {
	var runner *mocks.FakeRunner
	var mounter *mocks.FakeMounter
	BeforeEach(func() {
		runner = mocks.NewFakeRunner()
		mounter = mocks.NewFakeMounter()
	})
	Describe("Gdisk tests", Label("sgdisk"), func() {
		var gc part.Partitioner
		BeforeEach(func() {
			gc = part.NewPartitioner("/dev/device", runner, part.Gdisk)
		})
		It("Write changes does nothing with empty setup", func() {
			gc := part.NewPartitioner("/dev/device", runner, part.Gdisk)
			_, err := gc.WriteChanges()
			Expect(err).To(BeNil())
		})
		It("Runs complex command", func() {
			cmds := [][]string{
				{"sgdisk", "-P", "--zap-all", "-n=0:2048:+204800", "-c=0:p.efi", "-t=0:EF00",
					"-n=1:206848:+0", "-c=1:p.root", "-t=1:8300", "/dev/device"},
				{"sgdisk", "--zap-all", "-n=0:2048:+204800", "-c=0:p.efi", "-t=0:EF00",
					"-n=1:206848:+0", "-c=1:p.root", "-t=1:8300", "/dev/device"},
				{"partx", "-u", "/dev/device"},
			}
			part1 := part.Partition{
				Number: 0, StartS: 2048, SizeS: 204800,
				PLabel: "p.efi", FileSystem: "vfat",
			}
			gc.CreatePartition(&part1)
			part2 := part.Partition{
				Number: 1, StartS: 206848, SizeS: 0,
				PLabel: "p.root", FileSystem: "ext4",
			}
			gc.CreatePartition(&part2)
			gc.WipeTable(true)
			_, err := gc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Set a new partition label", func() {
			cmds := [][]string{
				{"sgdisk", "-P", "--zap-all", "/dev/device"},
				{"sgdisk", "--zap-all", "/dev/device"},
				{"partx", "-u", "/dev/device"},
			}
			Expect(gc.SetPartitionTableLabel(v2.GPT)).To(Succeed())
			gc.WipeTable(true)
			_, err := gc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Fails setting a new partition label", func() {
			Expect(gc.SetPartitionTableLabel(v2.MSDOS)).NotTo(Succeed())
		})
		It("Creates a new partition", func() {
			cmds := [][]string{
				{"sgdisk", "-n=0:2048:+204800", "-c=0:p.root", "-t=0:8300", "/dev/device"},
				{"partx", "-u", "/dev/device"},
				{"sgdisk", "-n=0:2048:+0", "-c=0:p.root", "-t=0:8300", "/dev/device"},
				{"partx", "-u", "/dev/device"},
			}
			partition := part.Partition{
				Number: 0, StartS: 2048, SizeS: 204800,
				PLabel: "p.root", FileSystem: "ext4",
			}
			gc.CreatePartition(&partition)
			_, err := gc.WriteChanges()
			Expect(err).To(BeNil())
			partition = part.Partition{
				Number: 0, StartS: 2048, SizeS: 0,
				PLabel: "p.root", FileSystem: "ext4",
			}
			gc.CreatePartition(&partition)
			_, err = gc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.MatchMilestones(cmds)).To(BeNil())
		})
		It("Deletes a partition", func() {
			cmds := [][]string{
				{"sgdisk", "-P", "-d=1", "-d=2", "/dev/device"},
				{"sgdisk", "-d=1", "-d=2", "/dev/device"},
				{"partx", "-u", "/dev/device"},
			}
			gc.DeletePartition(1)
			gc.DeletePartition(2)
			_, err := gc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Wipes partition table creating a new one", func() {
			cmds := [][]string{
				{"sgdisk", "-P", "--zap-all", "/dev/device"}, {"sgdisk", "--zap-all", "/dev/device"},
				{"partx", "-u", "/dev/device"},
			}
			gc.WipeTable(true)
			_, err := gc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Prints partition table info", func() {
			cmd := []string{"sgdisk", "-p", "/dev/device"}
			_, err := gc.Print()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch([][]string{cmd})).To(BeNil())
		})
		It("Gets last sector of the disk", func() {
			lastSec, _ := gc.GetLastSector(sgdiskPrint)
			Expect(lastSec).To(Equal(uint(500118158)))
			_, err := gc.GetLastSector("invalid parted print output")
			Expect(err).NotTo(BeNil())
		})
		It("Gets sector size of the disk", func() {
			secSize, _ := gc.GetSectorSize(sgdiskPrint)
			Expect(secSize).To(Equal(uint(512)))
			_, err := gc.GetSectorSize("invalid parted print output")
			Expect(err).NotTo(BeNil())
		})
		It("Gets partition table label", func() {
			label, _ := gc.GetPartitionTableLabel(sgdiskPrint)
			Expect(label).To(Equal(v2.GPT))
		})
		It("Gets partitions info of the disk", func() {
			parts := gc.GetPartitions(sgdiskPrint)
			// Ignores swap partition
			Expect(len(parts)).To(Equal(2))
			Expect(parts[1].StartS).To(Equal(uint(17303552)))
		})
	})
	Describe("Parted tests", Label("parted"), func() {
		var pc part.Partitioner
		BeforeEach(func() {
			pc = part.NewPartitioner("/dev/device", runner, part.Parted)
		})
		It("Write changes does nothing with empty setup", func() {
			pc := part.NewPartitioner("/dev/device", runner, part.Parted)
			_, err := pc.WriteChanges()
			Expect(err).To(BeNil())
		})
		It("Runs complex command", func() {
			cmds := [][]string{{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "mklabel", "gpt", "mkpart", "p.efi", "fat32",
				"2048", "206847", "mkpart", "p.root", "ext4", "206848", "100%",
			}, {
				"partx", "-u", "/dev/device",
			}}
			part1 := part.Partition{
				Number: 0, StartS: 2048, SizeS: 204800,
				PLabel: "p.efi", FileSystem: "vfat",
			}
			pc.CreatePartition(&part1)
			part2 := part.Partition{
				Number: 0, StartS: 206848, SizeS: 0,
				PLabel: "p.root", FileSystem: "ext4",
			}
			pc.CreatePartition(&part2)
			pc.WipeTable(true)
			_, err := pc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Set a new partition label", func() {
			cmds := [][]string{{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "mklabel", "msdos",
			}, {
				"partx", "-u", "/dev/device",
			}}
			pc.SetPartitionTableLabel("msdos")
			pc.WipeTable(true)
			_, err := pc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Creates a new partition", func() {
			cmds := [][]string{{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "mkpart", "p.root", "ext4", "2048", "206847",
			}, {
				"partx", "-u", "/dev/device",
			}, {
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "mkpart", "p.root", "ext4", "2048", "100%",
			}, {
				"partx", "-u", "/dev/device",
			}}
			partition := part.Partition{
				Number: 0, StartS: 2048, SizeS: 204800,
				PLabel: "p.root", FileSystem: "ext4",
			}
			pc.CreatePartition(&partition)
			_, err := pc.WriteChanges()
			Expect(err).To(BeNil())
			partition = part.Partition{
				Number: 0, StartS: 2048, SizeS: 0,
				PLabel: "p.root", FileSystem: "ext4",
			}
			pc.CreatePartition(&partition)
			_, err = pc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Deletes a partition", func() {
			cmds := [][]string{{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "rm", "1", "rm", "2",
			}, {
				"partx", "-u", "/dev/device",
			}}
			pc.DeletePartition(1)
			pc.DeletePartition(2)
			_, err := pc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Set a partition flag", func() {
			cmds := [][]string{{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "set", "1", "flag", "on", "set", "2", "flag", "off",
			}, {
				"partx", "-u", "/dev/device",
			}}
			pc.SetPartitionFlag(1, "flag", true)
			pc.SetPartitionFlag(2, "flag", false)
			_, err := pc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Wipes partition table creating a new one", func() {
			cmds := [][]string{{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "mklabel", "gpt",
			}, {
				"partx", "-u", "/dev/device",
			}}
			pc.WipeTable(true)
			_, err := pc.WriteChanges()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Prints partitin table info", func() {
			cmd := []string{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "print",
			}
			_, err := pc.Print()
			Expect(err).To(BeNil())
			Expect(runner.CmdsMatch([][]string{cmd})).To(BeNil())
		})
		It("Gets last sector of the disk", func() {
			lastSec, _ := pc.GetLastSector(partedPrint)
			Expect(lastSec).To(Equal(uint(50593792)))
			_, err := pc.GetLastSector("invalid parted print output")
			Expect(err).NotTo(BeNil())
		})
		It("Gets sector size of the disk", func() {
			secSize, _ := pc.GetSectorSize(partedPrint)
			Expect(secSize).To(Equal(uint(512)))
			_, err := pc.GetSectorSize("invalid parted print output")
			Expect(err).NotTo(BeNil())
		})
		It("Gets partition table label", func() {
			label, _ := pc.GetPartitionTableLabel(partedPrint)
			Expect(label).To(Equal("msdos"))
			_, err := pc.GetPartitionTableLabel("invalid parted print output")
			Expect(err).NotTo(BeNil())
		})
		It("Gets partitions info of the disk", func() {
			parts := pc.GetPartitions(partedPrint)
			Expect(len(parts)).To(Equal(4))
			Expect(parts[1].StartS).To(Equal(uint(98304)))
		})
	})
	Describe("Mkfs tests", Label("mkfs", "filesystem"), func() {
		It("Successfully formats a partition with xfs", func() {
			mkfs := part.NewMkfsCall("/dev/device", "xfs", "OEM", runner)
			_, err := mkfs.Apply()
			Expect(err).To(BeNil())
			cmds := [][]string{{"mkfs.xfs", "-L", "OEM", "/dev/device"}}
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Successfully formats a partition with vfat", func() {
			mkfs := part.NewMkfsCall("/dev/device", "vfat", "EFI", runner)
			_, err := mkfs.Apply()
			Expect(err).To(BeNil())
			cmds := [][]string{{"mkfs.vfat", "-n", "EFI", "/dev/device"}}
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("Fails for unsupported filesystem", func() {
			mkfs := part.NewMkfsCall("/dev/device", "zfs", "OEM", runner)
			_, err := mkfs.Apply()
			Expect(err).NotTo(BeNil())
		})
	})
	Describe("Disk tests", Label("mkfs", "filesystem"), func() {
		var dev *part.Disk
		var cmds [][]string
		var printCmd []string
		var fs vfs.FS
		var cleanup func()

		BeforeEach(func() {
			fs, cleanup, _ = vfst.NewTestFS(nil)

			err := utils.MkdirAll(fs, "/dev", constants.DirPerm)
			Expect(err).To(BeNil())
			_, err = fs.Create("/dev/device")
			Expect(err).To(BeNil())

			dev = part.NewDisk("/dev/device", part.WithRunner(runner), part.WithFS(fs), part.WithMounter(mounter))
			printCmd = []string{
				"parted", "--script", "--machine", "--", "/dev/device",
				"unit", "s", "print",
			}
			cmds = [][]string{printCmd}
		})
		AfterEach(func() { cleanup() })
		It("Creates a default disk", func() {
			dev = part.NewDisk("/dev/device")
		})
		Describe("Load data without changes", func() {
			BeforeEach(func() {
				runner.ReturnValue = []byte(partedPrint)
			})
			It("Loads disk layout data", func() {
				Expect(dev.Reload()).To(BeNil())
				Expect(dev.String()).To(Equal("/dev/device"))
				Expect(dev.GetSectorSize()).To(Equal(uint(512)))
				Expect(dev.GetLastSector()).To(Equal(uint(50593792)))
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Computes available free space", func() {
				Expect(dev.GetFreeSpace()).To(Equal(uint(262145)))
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Checks it has at least 128MB of free space", func() {
				Expect(dev.CheckDiskFreeSpaceMiB(128)).To(Equal(true))
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Checks it has less than 130MB of free space", func() {
				Expect(dev.CheckDiskFreeSpaceMiB(130)).To(Equal(false))
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Get partition label", func() {
				dev.Reload()
				Expect(dev.GetLabel()).To(Equal("msdos"))
			})
			It("It fixes GPT headers if the disk was expanded", func() {
				runner.ReturnValue = []byte("Warning: Not all of the space available to /dev/loop0...\n" + partedPrint)
				Expect(dev.Reload()).To(BeNil())
				Expect(runner.MatchMilestones([][]string{
					{"parted", "--script", "--machine", "--", "/dev/device", "unit", "s", "print"},
					{"sgdisk", "-e", "/dev/device"},
					{"parted", "--script", "--machine", "--", "/dev/device", "unit", "s", "print"},
				})).To(BeNil())
			})
		})
		Describe("Modify disk", func() {
			It("Format an already existing partition", func() {
				err := part.FormatDevice(runner, "/dev/device1", "ext4", "MY_LABEL")
				Expect(err).To(BeNil())
				Expect(runner.CmdsMatch([][]string{
					{"mkfs.ext4", "-L", "MY_LABEL", "/dev/device1"},
				})).To(BeNil())
			})
			It("Fails to create an unsupported partition table label", func() {
				runner.ReturnValue = []byte(partedPrint)
				_, err := dev.NewPartitionTable("invalidLabel")
				Expect(err).NotTo(BeNil())
			})
			It("Creates new partition table label", func() {
				cmds = [][]string{{
					"parted", "--script", "--machine", "--", "/dev/device",
					"unit", "s", "mklabel", "gpt",
				}, {
					"partx", "-u", "/dev/device",
				}, printCmd}
				runner.ReturnValue = []byte(partedPrint)
				_, err := dev.NewPartitionTable("gpt")
				Expect(err).To(BeNil())
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Adds a new partition", func() {
				cmds = [][]string{printCmd, {
					"parted", "--script", "--machine", "--", "/dev/device",
					"unit", "s", "mkpart", "primary", "ext4", "50331648", "100%",
					"set", "5", "boot", "on",
				}, {
					"partx", "-u", "/dev/device",
				}, printCmd}
				runner.ReturnValue = []byte(partedPrint)
				num, err := dev.AddPartition(0, "ext4", "ignored", "boot")
				Expect(err).To(BeNil())
				Expect(num).To(Equal(5))
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Fails to a new partition if there is not enough space available", func() {
				cmds = [][]string{printCmd}
				runner.ReturnValue = []byte(partedPrint)
				_, err := dev.AddPartition(130, "ext4", "ignored")
				Expect(err).NotTo(BeNil())
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Finds device for a given partition number", func() {
				_, err := fs.Create("/dev/device4")
				Expect(err).To(BeNil())
				cmds = [][]string{{"udevadm", "settle"}}
				Expect(dev.FindPartitionDevice(4)).To(Equal("/dev/device4"))
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Does not find device for a given partition number", func() {
				dev := part.NewDisk("/dev/lp0")
				_, err := dev.FindPartitionDevice(4)
				Expect(err).NotTo(BeNil())
			})
			It("Formats a partition", func() {
				_, err := fs.Create("/dev/device4")
				Expect(err).To(BeNil())
				cmds = [][]string{
					{"udevadm", "settle"},
					{"mkfs.xfs", "-L", "OEM", "/dev/device4"},
				}
				_, err = dev.FormatPartition(4, "xfs", "OEM")
				Expect(err).To(BeNil())
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Clears filesystem header from a partition", func() {
				cmds = [][]string{
					{"wipefs", "--all", "/dev/device1"},
				}
				Expect(dev.WipeFsOnPartition("/dev/device1")).To(BeNil())
				Expect(runner.CmdsMatch(cmds)).To(BeNil())
			})
			It("Fails while removing file system header", func() {
				runner.ReturnError = errors.New("some error")
				Expect(dev.WipeFsOnPartition("/dev/device1")).NotTo(BeNil())
			})
			Describe("Expanding partitions", func() {
				BeforeEach(func() {
					cmds = [][]string{
						printCmd, {
							"parted", "--script", "--machine", "--", "/dev/device",
							"unit", "s", "rm", "4", "mkpart", "primary", "", "45019136", "100%",
						}, {
							"partx", "-u", "/dev/device",
						}, printCmd, {"udevadm", "settle"},
					}
					runFunc := func(cmd string, args ...string) ([]byte, error) {
						switch cmd {
						case "parted":
							return []byte(partedPrint), nil
						default:
							return []byte{}, nil
						}
					}
					runner.SideEffect = runFunc
				})
				It("Expands ext4 partition", func() {
					_, err := fs.Create("/dev/device4")
					Expect(err).To(BeNil())
					extCmds := [][]string{
						{"e2fsck", "-fy", "/dev/device4"}, {"resize2fs", "/dev/device4"},
					}
					ghwTest := mocks.GhwMock{}
					disk := block.Disk{Name: "device", Partitions: []*block.Partition{
						{
							Name: "device4",
							Type: "ext4",
						},
					}}
					ghwTest.AddDisk(disk)
					ghwTest.CreateDevices()
					defer ghwTest.Clean()
					_, err = dev.ExpandLastPartition(0)
					Expect(err).To(BeNil())
					Expect(runner.CmdsMatch(append(cmds, extCmds...))).To(BeNil())
				})
				It("Expands xfs partition", func() {
					_, err := fs.Create("/dev/device4")
					Expect(err).To(BeNil())
					xfsCmds := [][]string{{"xfs_growfs"}}
					ghwTest := mocks.GhwMock{}
					disk := block.Disk{Name: "device", Partitions: []*block.Partition{
						{
							Name: "device4",
							Type: "xfs",
						},
					}}
					ghwTest.AddDisk(disk)
					ghwTest.CreateDevices()
					defer ghwTest.Clean()
					_, err = dev.ExpandLastPartition(0)
					Expect(err).To(BeNil())
					Expect(runner.CmdsMatch(append(cmds, xfsCmds...))).To(BeNil())
				})
				It("Expands btrfs partition", func() {
					_, err := fs.Create("/dev/device4")
					Expect(err).To(BeNil())
					xfsCmds := [][]string{{"btrfs", "filesystem", "resize"}}
					ghwTest := mocks.GhwMock{}
					disk := block.Disk{Name: "device", Partitions: []*block.Partition{
						{
							Name: "device4",
							Type: "btrfs",
						},
					}}
					ghwTest.AddDisk(disk)
					ghwTest.CreateDevices()
					defer ghwTest.Clean()
					_, err = dev.ExpandLastPartition(0)
					Expect(err).To(BeNil())
					Expect(runner.CmdsMatch(append(cmds, xfsCmds...))).To(BeNil())
				})
			})
		})
	})
})
