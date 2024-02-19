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

package snapshotter

import (
	"fmt"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
)

type snapshotterFactory func(cfg v2.Config, snapCfg v2.SnapshotterConfig, bootloader v2.Bootloader) (v2.Snapshotter, error)

var snapshotterFactories = map[string]snapshotterFactory{}

func NewSnapshotter(cfg v2.Config, snapCfg v2.SnapshotterConfig, bootloader v2.Bootloader) (v2.Snapshotter, error) {
	factory := snapshotterFactories[snapCfg.Type]
	if factory != nil {
		return factory(cfg, snapCfg, bootloader)
	}
	return nil, fmt.Errorf("unsupported snapshotter type: %s", snapCfg.Type)
}

func init() {
	snapshotterFactories[constants.LoopDeviceSnapshotterType] = newLoopDeviceSnapshotter
	snapshotterFactories[constants.BtrfsSnapshotterType] = newBtrfsSnapshotter
}
