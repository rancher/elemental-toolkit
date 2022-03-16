/*
Copyright © 2022 SUSE LLC

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

package utils

import (
	"encoding/json"
	"errors"

	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
)

type jPart struct {
	Label      string `json:"label,omitempty"`
	Size       uint64 `json:"size,omitempty"`
	FS         string `json:"fstype,omitempty"`
	MountPoint string `json:"mountpoint,omitempty"`
	Path       string `json:"path,omitempty"`
	Disk       string `json:"pkname,omitempty"`
	Type       string `json:"type,omitempty"`
}

type jParts []*v1.Partition

func (p jPart) Partition() *v1.Partition {
	// Converts B to MB
	return &v1.Partition{
		Label:      p.Label,
		Size:       uint(p.Size / (1024 * 1024)),
		FS:         p.FS,
		Flags:      []string{},
		MountPoint: p.MountPoint,
		Path:       p.Path,
		Disk:       p.Disk,
	}
}

func (p *jParts) UnmarshalJSON(data []byte) error {
	var parts []jPart

	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}

	var partitions jParts
	for _, part := range parts {
		// filter only partition or loop devices
		if part.Type == "part" || part.Type == "loop" {
			partitions = append(partitions, part.Partition())
		}
	}
	*p = partitions
	return nil
}

func unmarshalLsblk(lsblkOut []byte) ([]*v1.Partition, error) {
	var objmap map[string]*json.RawMessage
	err := json.Unmarshal(lsblkOut, &objmap)
	if err != nil {
		return nil, err
	}

	if _, ok := objmap["blockdevices"]; !ok {
		return nil, errors.New("Invalid json object, no 'blockdevices' key found")
	}

	var parts jParts
	err = json.Unmarshal(*objmap["blockdevices"], &parts)
	if err != nil {
		return nil, err
	}

	return parts, nil
}

// GetAllPartitions gets a slice of all partition devices found in the host
// mapped into a v1.PartitionList object.
func GetAllPartitions(runner v1.Runner) (v1.PartitionList, error) {
	out, err := runner.Run("lsblk", "-p", "-b", "-n", "-J", "--output", "LABEL,SIZE,FSTYPE,MOUNTPOINT,PATH,PKNAME,TYPE")
	if err != nil {
		return nil, err
	}

	return unmarshalLsblk(out)
}

// GetDevicePartitions gets a slice of partitions found in the given device mapped
// into a v1.PartitionList object. If the device is a disk it will list all disk
// partitions, if the device is already a partition it will simply list a single partition.
func GetDevicePartitions(runner v1.Runner, device string) (v1.PartitionList, error) {
	out, err := runner.Run("lsblk", "-p", "-b", "-n", "-J", "--output", "LABEL,SIZE,FSTYPE,MOUNTPOINT,PATH,PKNAME,TYPE", device)
	if err != nil {
		return nil, err
	}

	return unmarshalLsblk(out)
}
