/*
Copyright © 2022 - 2026 SUSE LLC

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

package partitioner

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

type partedCall struct {
	dev       string
	wipe      bool
	parts     []*Partition
	deletions []int
	label     string
	runner    types.Runner
	flags     []partFlag
}

type partFlag struct {
	flag   string
	active bool
	number int
}

var _ Partitioner = (*partedCall)(nil)

func newPartedCall(dev string, runner types.Runner) *partedCall {
	return &partedCall{dev: dev, wipe: false, parts: []*Partition{}, deletions: []int{}, label: "", runner: runner, flags: []partFlag{}}
}

func (pc partedCall) optionsBuilder() []string {
	opts := []string{}
	label := pc.label
	match, _ := regexp.MatchString(fmt.Sprintf("msdos|%s", constants.GPT), label)
	// Fallback to gpt if label is empty or invalid
	if !match {
		label = constants.GPT
	}

	if pc.wipe {
		opts = append(opts, "mklabel", label)
	}

	for _, partnum := range pc.deletions {
		opts = append(opts, "rm", fmt.Sprintf("%d", partnum))
	}

	isFat, _ := regexp.Compile("fat|vfat")
	for _, part := range pc.parts {
		var pLabel string
		if label == constants.GPT && part.PLabel != "" {
			pLabel = part.PLabel
		} else if label == constants.GPT {
			pLabel = fmt.Sprintf("part%d", part.Number)
		} else {
			pLabel = "primary"
		}

		opts = append(opts, "mkpart", pLabel)

		if isFat.MatchString(part.FileSystem) {
			opts = append(opts, "fat32")
		} else {
			opts = append(opts, part.FileSystem)
		}

		if part.SizeS == 0 {
			// Size set to zero means is interperted as all space available
			opts = append(opts, fmt.Sprintf("%d", part.StartS), "100%")
		} else {
			opts = append(opts, fmt.Sprintf("%d", part.StartS), fmt.Sprintf("%d", part.StartS+part.SizeS-1))
		}
	}

	for _, flag := range pc.flags {
		opts = append(opts, "set", fmt.Sprintf("%d", flag.number), flag.flag)
		if flag.active {
			opts = append(opts, "on")
		} else {
			opts = append(opts, "off")
		}
	}

	if len(opts) == 0 {
		return nil
	}

	return append([]string{"--script", "--machine", "--", pc.dev, "unit", "s"}, opts...)
}

func (pc *partedCall) WriteChanges() (string, error) {
	opts := pc.optionsBuilder()
	if len(opts) == 0 {
		return "", nil
	}
	out, err := pc.runner.Run("parted", opts...)

	// Notify kernel of partition table changes, swallows errors, just a best effort call
	_, _ = pc.runner.Run("partx", "-u", pc.dev)
	pc.wipe = false
	pc.parts = []*Partition{}
	pc.deletions = []int{}
	return string(out), err
}

func (pc *partedCall) SetPartitionTableLabel(label string) error {
	match, _ := regexp.MatchString("msdos|gpt", label)
	if !match {
		return fmt.Errorf("Invalid partition table type, only msdos and gpt are supported")
	}
	pc.label = label
	return nil
}

func (pc *partedCall) CreatePartition(p *Partition) {
	pc.parts = append(pc.parts, p)
}

func (pc *partedCall) DeletePartition(num int) {
	pc.deletions = append(pc.deletions, num)
}

func (pc *partedCall) SetPartitionFlag(num int, flag string, active bool) {
	pc.flags = append(pc.flags, partFlag{flag: flag, active: active, number: num})
}

func (pc *partedCall) WipeTable(wipe bool) {
	pc.wipe = wipe
}

func (pc partedCall) Print() (string, error) {
	out, err := pc.runner.Run("parted", "--script", "--machine", "--", pc.dev, "unit", "s", "print")
	return string(out), err
}

// Parses the output of a partedCall.Print call
func (pc partedCall) parseHeaderFields(printOut string, field int) (string, error) {
	re := regexp.MustCompile(`^(.*):(\d+)s:(.*):(\d+):(\d+):(.*):(.*):(.*);$`)

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(printOut)))
	for scanner.Scan() {
		match := re.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if match != nil {
			return match[field], nil
		}
	}
	return "", errors.New("failed parsing parted header data")
}

// Parses the output of a partedCall.Print call
func (pc partedCall) GetLastSector(printOut string) (uint, error) {
	field, err := pc.parseHeaderFields(printOut, 2)
	if err != nil {
		return 0, errors.New("Failed parsing last sector")
	}
	lastSec, err := strconv.ParseUint(field, 10, 0)
	return uint(lastSec), err
}

// Parses the output of a partedCall.Print call
func (pc partedCall) GetSectorSize(printOut string) (uint, error) {
	field, err := pc.parseHeaderFields(printOut, 4)
	if err != nil {
		return 0, errors.New("Failed parsing sector size")
	}
	secSize, err := strconv.ParseUint(field, 10, 0)
	return uint(secSize), err
}

// Parses the output of a partedCall.Print call
func (pc partedCall) GetPartitionTableLabel(printOut string) (string, error) {
	return pc.parseHeaderFields(printOut, 6)
}

// Parses the output of a GdiskCall.Print call
func (pc partedCall) GetPartitions(printOut string) []Partition { //nolint:dupl
	re := regexp.MustCompile(`^(\d+):(\d+)s:(\d+)s:(\d+)s:(.*):(.*):(.*);$`)
	var start uint
	var end uint
	var size uint
	var pLabel string
	var partNum int
	var partitions []Partition

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(printOut)))
	for scanner.Scan() {
		match := re.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if match != nil {
			partNum, _ = strconv.Atoi(match[1])
			parsed, _ := strconv.ParseUint(match[2], 10, 0)
			start = uint(parsed)
			parsed, _ = strconv.ParseUint(match[3], 10, 0)
			end = uint(parsed)
			size = end - start + 1
			pLabel = match[6]

			partitions = append(partitions, Partition{
				Number:     partNum,
				StartS:     start,
				SizeS:      size,
				PLabel:     pLabel,
				FileSystem: "",
			})
		}
	}

	return partitions
}
