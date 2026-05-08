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

package mocks

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

// FakeLoopDeviceSnapshotsStatus creates fake snapshots files according to the LoopDevice behavior.
// Used for unit testing only.
func FakeLoopDeviceSnapshotsStatus(fs types.FS, rootDir string, snapsCount int) error {
	var snapshotFile, snapshotsPrefix string
	var i int
	var err error

	snapshotsPrefix = filepath.Join(rootDir, ".snapshots")
	for i = 1; i <= snapsCount; i++ {
		err = utils.MkdirAll(fs, filepath.Join(rootDir, ".snapshots", strconv.Itoa(i)), constants.DirPerm)
		if err != nil {
			return err
		}
		snapshotFile = filepath.Join(snapshotsPrefix, strconv.Itoa(i), "snapshot.img")
		err = fs.WriteFile(snapshotFile, []byte(fmt.Sprintf("This is snapshot %d", i)), constants.FilePerm)
		if err != nil {
			return err
		}
	}
	err = fs.Symlink(filepath.Join(strconv.Itoa(i-1), "snapshot.img"), filepath.Join(snapshotsPrefix, constants.ActiveSnapshot))
	if err != nil {
		return err
	}
	passivesPath := filepath.Join(snapshotsPrefix, "passives")
	err = utils.MkdirAll(fs, passivesPath, constants.DirPerm)
	if err != nil {
		return err
	}
	return err
}
