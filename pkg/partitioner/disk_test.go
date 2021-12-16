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
	. "github.com/onsi/gomega"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	mocks "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/spf13/afero"
	"testing"
)

func TestReload(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "print",
	}}
	runner.ReturnValue = []byte(printOutput)
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	err := dev.Reload()
	Expect(err).To(BeNil())
	Expect(dev.String()).To(Equal("/some/device"))
	Expect(dev.GetSectorSize()).To(Equal(uint(512)))
	Expect(dev.GetLastSector()).To(Equal(uint(50593792)))
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestCheckDiskFreeSpaceMiB(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "print",
	}}
	runner.ReturnValue = []byte(printOutput)
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	//Disk has 128M free according to printOutput
	Expect(dev.CheckDiskFreeSpaceMiB(130)).To(Equal(false))
	Expect(dev.CheckDiskFreeSpaceMiB(128)).To(Equal(true))
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestGetFreeSpace(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "print",
	}}
	runner.ReturnValue = []byte(printOutput)
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	Expect(dev.GetFreeSpace()).To(Equal(uint(262145)))
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestNewPartitionTable(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "mklabel", "gpt",
	}, {
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "print",
	}}
	runner.ReturnValue = []byte(printOutput)
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	_, err := dev.NewPartitionTable("invalidLabel")
	Expect(err).NotTo(BeNil())
	_, err = dev.NewPartitionTable("gpt")
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestAddPartition(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "print",
	}, {
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "mkpart", "primary", "ext4", "50331648", "100%",
		"set", "5", "boot", "on",
	}, {
		"parted", "--script", "--machine", "--", "/some/device",
		"unit", "s", "print",
	}}
	runner.ReturnValue = []byte(printOutput)
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	num, err := dev.AddPartition(130, "ext4", "ignored")
	Expect(err).NotTo(BeNil())
	Expect(dev.GetLabel()).To(Equal("msdos"))
	num, err = dev.AddPartition(0, "ext4", "ignored", "boot")
	Expect(err).To(BeNil())
	Expect(num).To(Equal(5))
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestReloadPartitionTable(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{
		{"udevadm", "settle"},
		{"partprobe", "/some/device"},
	}
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	Expect(dev.ReloadPartitionTable()).To(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())

	//Test partprobe failure exhausting all tries
	triesCount := 11
	runner.ClearCmds()
	dev = part.NewDisk("/some/device", part.WithRunner(runner))
	runFunc := func(cmd string, args ...string) ([]byte, error) {
		switch cmd {
		case "partprobe":
			if triesCount > 0 {
				triesCount--
				return []byte{}, errors.New("Fake error")
			}
			return []byte{}, nil
		default:
			return []byte{}, nil
		}
	}
	runner.ReturnValue = []byte{}
	runner.SideEffect = runFunc
	tryCmds := [][]string{
		{"udevadm", "settle"},
		{"partprobe", "/some/device"},
	}
	cmds = [][]string{}
	for tries := 0; tries < triesCount-1; tries++ {
		cmds = append(cmds, tryCmds...)
	}
	Expect(dev.ReloadPartitionTable()).NotTo(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
	Expect(triesCount).To(Equal(1))
}

func TestFindPartitionDevice(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{
		{"udevadm", "settle"},
		{"partprobe", "/some/device"},
		{"lsblk", "-ltnpo", "name,type", "/some/device"},
	}
	runner.ReturnValue = []byte("/some/device4 part")
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	Expect(dev.FindPartitionDevice(4)).To(Equal("/some/device4"))
	Expect(runner.CmdsMatch(cmds)).To(BeNil())

	//Testing one retry needed
	triesCount := 1
	runner.ClearCmds()
	dev = part.NewDisk("/some/device", part.WithRunner(runner))
	runFunc := func(cmd string, args ...string) ([]byte, error) {
		switch cmd {
		case "partprobe":
			return []byte{}, nil
		case "lsblk":
			if triesCount > 0 {
				triesCount--
				return []byte{}, errors.New("Fake error")
			}
			return []byte("/some/device4 part"), nil
		default:
			return []byte{}, nil
		}
	}
	runner.ReturnValue = []byte{}
	runner.SideEffect = runFunc
	cmds = [][]string{
		{"udevadm", "settle"},
		{"partprobe", "/some/device"},
		{"lsblk", "-ltnpo", "name,type", "/some/device"},
		{"udevadm", "settle"},
		{"partprobe", "/some/device"},
		{"lsblk", "-ltnpo", "name,type", "/some/device"},
	}
	Expect(dev.FindPartitionDevice(4)).To(Equal("/some/device4"))
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestFormatPartition(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{
		{"udevadm", "settle"},
		{"partprobe", "/some/device"},
		{"lsblk", "-ltnpo", "name,type", "/some/device"},
		{"mkfs.xfs", "-L", "OEM", "/some/device4"},
	}
	runner.ReturnValue = []byte("/some/device4 part")
	dev := part.NewDisk("/some/device", part.WithRunner(runner))
	_, err := dev.FormatPartition(4, "xfs", "OEM")
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestExpandLastPartition(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{
		{
			"parted", "--script", "--machine", "--",
			"/some/device", "unit", "s", "print",
		}, {
			"parted", "--script", "--machine", "--", "/some/device",
			"unit", "s", "rm", "4", "mkpart", "primary", "45019136", "100%",
		}, {
			"parted", "--script", "--machine", "--",
			"/some/device", "unit", "s", "print",
		}, {"udevadm", "settle"}, {"partprobe", "/some/device"},
		{"lsblk", "-ltnpo", "name,type", "/some/device"},
		{"blkid", "/some/device4", "-s", "TYPE", "-o", "value"},
	}
	extCmds := [][]string{
		{"e2fsck", "-fy", "/some/device4"}, {"resize2fs", "/some/device4"},
	}
	xfsCmds := [][]string{
		{"mount", "-t", "xfs"}, {"xfs_growfs"}, {"umount"},
	}
	fileSystem := "ext4"
	runFunc := func(cmd string, args ...string) ([]byte, error) {
		switch cmd {
		case "parted":
			return []byte(printOutput), nil
		case "lsblk":
			return []byte("/some/device4 part"), nil
		case "blkid":
			return []byte(fileSystem), nil
		default:
			return []byte{}, nil
		}
	}
	runner.SideEffect = runFunc

	dev := part.NewDisk("/some/device", part.WithRunner(runner), part.WithFS(afero.NewMemMapFs()))
	_, err := dev.ExpandLastPartition(0)
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(append(cmds, extCmds...))).To(BeNil())

	runner.ClearCmds()
	fileSystem = "xfs"
	dev = part.NewDisk("/some/device", part.WithRunner(runner), part.WithFS(afero.NewMemMapFs()))
	_, err = dev.ExpandLastPartition(0)
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(append(cmds, xfsCmds...))).To(BeNil())
}
