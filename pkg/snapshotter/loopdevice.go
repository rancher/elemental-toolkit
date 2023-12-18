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

package snapshotter

import (
	"path/filepath"

	"github.com/rancher/elemental-toolkit/pkg/constants"

	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const (
	loopDeviceSnapsPath    = ".snapshots"
	loopDeviceImgName      = "snapshot.img"
	loopDeviceWorkDir      = "snapshot.workDir"
	loopDeviceLabelPattern = "EL_SNAP%d"
	loopDevicePassiveSnaps = loopDeviceSnapsPath + "/passives"
)

var _ v1.Snapshotter = (*LoopDevice)(nil)

type LoopDevice struct {
	cfg            v1.Config
	snapshotterCfg v1.SnapshotterConfig
	loopDevCfg     v1.LoopDeviceConfig
	rootDir        string
	/*currentSnapshotID int
	activeSnapshotID  int*/
	bootloader v1.Bootloader
}

func NewLoopDeviceSnapshotter(cfg v1.Config, snapCfg v1.SnapshotterConfig, bootloader v1.Bootloader) *LoopDevice {
	loopDevCfg := snapCfg.Config.(v1.LoopDeviceConfig)
	return &LoopDevice{cfg: cfg, snapshotterCfg: snapCfg, loopDevCfg: loopDevCfg, bootloader: bootloader}
}

func (l *LoopDevice) InitSnapshotter(rootDir string) error {
	l.cfg.Logger.Infof("Initiating a LoopDevice snapshotter at %s", rootDir)
	l.rootDir = rootDir
	return utils.MkdirAll(l.cfg.Fs, filepath.Join(rootDir, loopDevicePassiveSnaps), constants.DirPerm)
}

func (l *LoopDevice) StartTransaction() (*v1.Snapshot, error) {
	var snap *v1.Snapshot

	return snap, nil
}

func (l *LoopDevice) CloseTransactionOnError(_ *v1.Snapshot) error {
	var err error
	return err
}

func (l *LoopDevice) CloseTransaction(_ *v1.Snapshot) (err error) {
	return err
}

func (l *LoopDevice) DeleteSnapshot(_ int) error {
	var err error
	return err
}
