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
	. "github.com/onsi/gomega"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	mocks "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"testing"
)

func TestWriteChanges(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	_, err := pc.WriteChanges()
	Expect(err).To(BeNil())

	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "mklabel", "gpt", "mkpart", "p.root", "ext4",
		"2048", "206847", "mkpart", "p.home", "ext4", "206848", "100%",
	}}
	part1 := part.Partition{
		Number: 0, StartS: 2048, SizeS: 204800,
		PLabel: "p.root", FileSystem: "ext4",
	}
	pc.CreatePartition(&part1)
	part2 := part.Partition{
		Number: 0, StartS: 206848, SizeS: 0,
		PLabel: "p.home", FileSystem: "ext4",
	}
	pc.CreatePartition(&part2)
	pc.WipeTable(true)
	_, err = pc.WriteChanges()
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

var printOutput = `BYT;
/dev/loop0:50593792s:loopback:512:512:msdos:Loopback device:;
1:2048s:98303s:96256s:ext4::type=83;
2:98304s:29394943s:29296640s:ext4::boot, type=83;
3:29394944s:45019135s:15624192s:ext4::type=83;
4:45019136s:50331647s:5312512s:ext4::type=83;`

func TestSetPartitionTableLabel(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "mklabel", "msdos",
	}}
	pc.SetPartitionTableLabel("msdos")
	pc.WipeTable(true)
	_, err := pc.WriteChanges()
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestCreatePartition(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "mkpart", "p.root", "ext4", "2048", "206847",
	}, {
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "mkpart", "p.root", "ext4", "2048", "100%",
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
}

func TestDeletePartition(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	cmd := []string{
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "rm", "1", "rm", "2",
	}
	pc.DeletePartition(1)
	pc.DeletePartition(2)
	_, err := pc.WriteChanges()
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch([][]string{cmd})).To(BeNil())
}

func TestSetPartitionFlag(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	cmds := [][]string{{
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "set", "1", "flag", "on", "set", "2", "flag", "off",
	}}
	pc.SetPartitionFlag(1, "flag", true)
	pc.SetPartitionFlag(2, "flag", false)
	_, err := pc.WriteChanges()
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
}

func TestWipeTable(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	cmd := []string{
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "mklabel", "gpt",
	}
	pc.WipeTable(true)
	_, err := pc.WriteChanges()
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch([][]string{cmd})).To(BeNil())
}

func TestPrint(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	cmd := []string{
		"parted", "--script", "--machine", "--", "/some/dev",
		"unit", "s", "print",
	}
	_, err := pc.Print()
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch([][]string{cmd})).To(BeNil())
}

func TestLastSector(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	lastSec, _ := pc.GetLastSector(printOutput)
	Expect(lastSec).To(Equal(uint(50593792)))
	_, err := pc.GetLastSector("invalid parted print output")
	Expect(err).NotTo(BeNil())
}

func TestSectorSize(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	secSize, _ := pc.GetSectorSize(printOutput)
	Expect(secSize).To(Equal(uint(512)))
	_, err := pc.GetSectorSize("invalid parted print output")
	Expect(err).NotTo(BeNil())
}

func TestGetPartitionTableLabel(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	label, _ := pc.GetPartitionTableLabel(printOutput)
	Expect(label).To(Equal("msdos"))
	_, err := pc.GetPartitionTableLabel("invalid parted print output")
	Expect(err).NotTo(BeNil())
}

func TestGetPartitions(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	pc := part.NewPartedCall("/some/dev", runner)
	parts := pc.GetPartitions(printOutput)
	Expect(len(parts)).To(Equal(4))
	Expect(parts[1].StartS).To(Equal(uint(98304)))
}
