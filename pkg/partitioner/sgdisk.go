/*
Copyright © 2022 - 2023 SUSE LLC

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

	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
)

const efiType = "EF00"
const biosType = "EF02" // unused
const linuxType = "8300"

type GdiskCall struct {
	dev       string
	wipe      bool
	parts     []*Partition
	deletions []int
	label     string
	runner    v1.Runner
	expand    bool
	pretend   bool
}

func NewGdiskCall(dev string, runner v1.Runner) *GdiskCall {
	return &GdiskCall{
		dev:       dev,
		runner:    runner,
		parts:     []*Partition{},
		deletions: []int{},
	}
}

func (gd GdiskCall) buildOptions() []string {
	opts := []string{}
	isFat, _ := regexp.Compile("fat|vfat")

	if gd.pretend {
		opts = append(opts, "-P")
	}

	if gd.wipe {
		opts = append(opts, "--zap-all")
	}

	if gd.expand {
		opts = append(opts, "-e")
	}

	for _, partnum := range gd.deletions {
		opts = append(opts, fmt.Sprintf("-d=%d", partnum))
	}

	for _, part := range gd.parts {
		opts = append(opts, fmt.Sprintf("-n=%d:%d:+%d", part.Number, part.StartS, part.SizeS))

		if part.PLabel != "" {
			opts = append(opts, fmt.Sprintf("-c=%d:%s", part.Number, part.PLabel))
		}

		if isFat.MatchString(part.FileSystem) {
			opts = append(opts, fmt.Sprintf("-t=%d:%s", part.Number, efiType))
		} else if part.FileSystem != "" {
			opts = append(opts, fmt.Sprintf("-t=%d:%s", part.Number, linuxType))
		}
	}

	// TODO Handle flags

	if len(opts) == 0 {
		return nil
	}

	opts = append(opts, gd.dev)
	return opts
}

func (gd GdiskCall) Verify() (string, error) {
	out, err := gd.runner.Run("sgdisk", "--verify", gd.dev)
	return string(out), err
}

func (gd *GdiskCall) WriteChanges() (string, error) {
	// Run sgdisk with --pretend flag first to as a sanity check
	// before any change to disk happens
	gd.SetPretend(true)
	opts := gd.buildOptions()
	out, err := gd.runner.Run("sgdisk", opts...)
	if err != nil {
		return string(out), err
	}

	gd.SetPretend(false)
	opts = gd.buildOptions()
	out, err = gd.runner.Run("sgdisk", opts...)
	return string(out), err
}

func (gd *GdiskCall) SetPartitionTableLabel(label string) error {
	if label != "gpt" {
		return fmt.Errorf("Invalid partition table type (%s), only GPT is supported by sgdisk", label)
	}
	return nil
}

func (gd *GdiskCall) CreatePartition(p *Partition) {
	gd.parts = append(gd.parts, p)
}

func (gd *GdiskCall) DeletePartition(num int) {
	gd.deletions = append(gd.deletions, num)
}

func (gd *GdiskCall) SetPartitionFlag(num int, flag string, active bool) {
	// TODO set flags
}

func (gd *GdiskCall) WipeTable(wipe bool) {
	gd.wipe = wipe
}

func (gd GdiskCall) Print() (string, error) {
	out, err := gd.runner.Run("sgdisk", "-p", gd.dev)
	return string(out), err
}

// Parses the output of a GdiskCall.Print call
func (gd GdiskCall) GetLastSector(printOut string) (uint, error) {
	re := regexp.MustCompile("last usable sector is (\\d+)")
	match := re.FindStringSubmatch(printOut)
	if match != nil {
		endS, err := strconv.ParseUint(match[1], 10, 0)
		return uint(endS), err
	}
	return 0, errors.New("Could not determine last usable sector")
}

// Parses the output of a GdiskCall.Print call
func (gd GdiskCall) GetSectorSize(printOut string) (uint, error) {
	re := regexp.MustCompile("sector size: (\\d+)")
	match := re.FindStringSubmatch(printOut)
	if match != nil {
		size, err := strconv.ParseUint(match[1], 10, 0)
		return uint(size), err
	}
	return 0, errors.New("Could not determine sector size")
}

// TODO parse printOut from a non gpt disk and return error here
func (gd GdiskCall) GetPartitionTableLabel(printOut string) (string, error) {
	return "gpt", nil
}

// Parses the output of a GdiskCall.Print call
func (gd GdiskCall) GetPartitions(printOut string) []Partition {
	re := regexp.MustCompile("^(\\d+)\\s+(\\d+)\\s+(\\d+).*(EF02|EF00|8300)\\s*(.*)$")
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
			pLabel = match[5]

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

func (gd *GdiskCall) SetPretend(pretend bool) {
	gd.pretend = pretend
}

func (gd *GdiskCall) ExpandPTable() {
	gd.expand = true
}
