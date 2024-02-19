/*
Copyright © 2022 - 2024 SUSE LLC

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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/block"
	ghwUtil "github.com/jaypipes/ghw/pkg/util"

	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
)

// ghwPartitionToInternalPartition transforms a block.Partition from ghw lib to our v2.Partition type
func ghwPartitionToInternalPartition(partition *block.Partition) *v2.Partition {
	return &v2.Partition{
		FilesystemLabel: partition.FilesystemLabel,
		Size:            uint(partition.SizeBytes / (1024 * 1024)), // Converts B to MB
		Name:            partition.Label,
		FS:              partition.Type,
		Flags:           nil,
		MountPoint:      partition.MountPoint,
		Path:            filepath.Join("/dev", partition.Name),
		Disk:            filepath.Join("/dev", partition.Disk.Name),
	}
}

// GetAllPartitions returns all partitions in the system for all disks
func GetAllPartitions() (v2.PartitionList, error) {
	var parts []*v2.Partition
	blockDevices, err := block.New(ghw.WithDisableTools(), ghw.WithDisableWarnings())
	if err != nil {
		return nil, err
	}
	for _, d := range blockDevices.Disks {
		for _, part := range d.Partitions {
			parts = append(parts, ghwPartitionToInternalPartition(part))
		}
	}

	return parts, nil
}

// GetPartitionFS gets the FS of a partition given
func GetPartitionFS(partition string) (string, error) {
	// We want to have the device always prefixed with a /dev
	if !strings.HasPrefix(partition, "/dev") {
		partition = filepath.Join("/dev", partition)
	}
	blockDevices, err := block.New(ghw.WithDisableTools(), ghw.WithDisableWarnings())
	if err != nil {
		return "", err
	}

	for _, disk := range blockDevices.Disks {
		for _, part := range disk.Partitions {
			if filepath.Join("/dev", part.Name) == partition {
				if part.Type == ghwUtil.UNKNOWN {
					return "", fmt.Errorf("could not find filesystem for partition %s", partition)
				}
				return part.Type, nil
			}
		}
	}
	return "", fmt.Errorf("could not find partition %s", partition)
}
