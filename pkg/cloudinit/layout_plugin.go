/*
Copyright © 2022 - 2025 SUSE LLC

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

package cloudinit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/partitioner"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

// layoutPlugin is the elemental's implementation of Layout yip's plugin based
// on partitioner package
func layoutPlugin(l logger.Interface, s schema.Stage, fs vfs.FS, console plugins.Console) (err error) {
	if s.Layout.Device == nil {
		return nil
	}

	var dev *partitioner.Disk
	elemConsole, ok := console.(*cloudInitConsole)
	if !ok {
		return errors.New("provided console is not an instance of 'cloudInitConsole' type")
	}
	runner := elemConsole.getRunner()
	log, ok := l.(v1.Logger)
	if !ok {
		return errors.New("provided logger is not implementing v1.Logger interface")
	}

	if len(strings.TrimSpace(s.Layout.Device.Label)) > 0 {
		partDevice, err := utils.GetFullDeviceByLabel(runner, s.Layout.Device.Label, 5)
		if err != nil {
			l.Errorf("Exiting, disk not found:\n %s", err.Error())
			return err
		}
		dev = partitioner.NewDisk(
			partDevice.Disk,
			partitioner.WithRunner(runner),
			partitioner.WithLogger(log),
			partitioner.WithFS(fs),
		)
	} else if len(strings.TrimSpace(s.Layout.Device.Path)) > 0 {
		dev = partitioner.NewDisk(
			s.Layout.Device.Path,
			partitioner.WithRunner(runner),
			partitioner.WithLogger(log),
			partitioner.WithFS(fs),
		)
	} else {
		l.Warnf("No target device defined, nothing to do")
		return nil
	}

	if !dev.Exists() {
		l.Errorf("Exiting, disk not found:\n %s", s.Layout.Device.Path)
		return errors.New("Target disk not found")
	}

	if s.Layout.Expand != nil {
		l.Infof("Extending last partition up to %d MiB", s.Layout.Expand.Size)
		out, err := dev.ExpandLastPartition(s.Layout.Expand.Size)
		if err != nil {
			l.Error(out)
			return err
		}
	}

	for _, part := range s.Layout.Parts {
		_, err := utils.GetFullDeviceByLabel(runner, part.FSLabel, 1)
		if err == nil {
			l.Warnf("Partition with FSLabel: %s already exists, ignoring", part.FSLabel)
			continue
		}

		// Set default filesystem
		if part.FileSystem == "" {
			part.FileSystem = constants.LinuxFs
		}

		l.Infof("Creating %s partition", part.FSLabel)
		partNum, err := dev.AddPartition(part.Size, part.FileSystem, part.PLabel)
		if err != nil {
			return fmt.Errorf("Failed creating partitions: %w", err)
		}
		out, err := dev.FormatPartition(partNum, part.FileSystem, part.FSLabel)
		if err != nil {
			return fmt.Errorf("Formatting partition failed: %s\nError: %w", out, err)
		}
	}
	return nil
}
