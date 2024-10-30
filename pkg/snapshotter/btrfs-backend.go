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
	"bufio"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/elemental"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ subvolumeBackend = (*btrfsBackend)(nil)

type SnapperSnapshotXML struct {
	XMLName     xml.Name   `xml:"snapshot"`
	Type        string     `xml:"type"`
	Num         int        `xml:"num"`
	Date        Date       `xml:"date"`
	Cleanup     string     `xml:"cleanup"`
	Description string     `xml:"description"`
	UserData    []UserData `xml:"userdata"`
}

type UserData struct {
	XMLName xml.Name `xml:"userdata"`
	Key     string   `xml:"key"`
	Value   string   `xml:"value"`
}

type btrfsSubvol struct {
	path string
	id   int
}

type btrfsSubvolList []btrfsSubvol

func newSnapperSnapshotXML(id int, desc string) SnapperSnapshotXML {
	var usrData UserData
	if id == 1 {
		usrData = UserData{Key: installProgress, Value: "yes"}
	} else {
		usrData = UserData{Key: updateProgress, Value: "yes"}
	}
	return SnapperSnapshotXML{
		Type:        "single",
		Num:         id,
		Date:        Date(time.Now()),
		Description: desc,
		Cleanup:     "number",
		UserData:    []UserData{usrData},
	}
}

func (d Date) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	t := time.Time(d)
	v := t.Format(dateFormat)
	return e.EncodeElement(v, start)
}

func (d *Date) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var s string
	err := dec.DecodeElement(&s, &start)
	if err != nil {
		return err
	}
	t, err := time.Parse(dateFormat, s)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

type btrfsBackend struct {
	cfg          *types.Config
	stat         backendStat
	maxSnapshots int
}

func newBtrfsBackend(cfg *types.Config, maxSnapshots int) *btrfsBackend {
	return &btrfsBackend{cfg: cfg, maxSnapshots: maxSnapshots}
}

func (b *btrfsBackend) Probe(device string, mountpoint string) (backendStat, error) {
	var rootVolume, snapshotsVolume bool

	// Do not compute it again if it was already done in advance
	if b.stat.rootDir != "" && b.stat.stateMount != "" {
		return b.stat, nil
	}

	volumes, err := b.getSubvolumes(mountpoint)
	if err != nil {
		return b.stat, err
	}

	b.cfg.Logger.Debugf(
		"Looking for subvolume ids %d and %d in subvolume list: %v",
		rootSubvolID, snapshotsSubvolID, volumes,
	)
	for _, vol := range volumes {
		if vol.id == rootSubvolID {
			rootVolume = true
		} else if vol.id == snapshotsSubvolID {
			snapshotsVolume = true
		}
	}

	if rootVolume && snapshotsVolume {
		id, err := b.getActiveSnapshot(mountpoint)
		if err != nil {
			return b.stat, err
		}
		if id > 0 {
			b.stat.activeID = id
		}
	}

	// On active or passive we must ensure the actual mountpoint reported by the state
	// partition is the actual root, ghw only reports a single mountpoint per device...
	if elemental.IsPassiveMode(*b.cfg) || elemental.IsActiveMode(*b.cfg) {
		rootDir, stateMount, currentID, err := b.findStateMount(device)
		if err != nil {
			return b.stat, err
		}
		b.stat.rootDir = rootDir
		b.stat.stateMount = stateMount
		b.stat.currentID = currentID
		return b.stat, nil
	}

	b.stat.rootDir = mountpoint
	b.stat.stateMount = mountpoint
	return b.stat, nil
}

func (b *btrfsBackend) InitBrfsPartition(rootDir string) error {
	b.cfg.Logger.Debug("Enabling btrfs quota")
	cmdOut, err := b.cfg.Runner.Run("btrfs", "quota", "enable", rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed setting quota for btrfs partition at %s: %s", rootDir, string(cmdOut))
		return err
	}

	b.cfg.Logger.Debug("Creating essential subvolumes")
	for _, subvolume := range []string{filepath.Join(rootDir, rootSubvol), filepath.Join(rootDir, rootSubvol, snapshotsPath)} {
		b.cfg.Logger.Debugf("Creating subvolume: %s", subvolume)
		cmdOut, err = b.cfg.Runner.Run("btrfs", "subvolume", "create", subvolume)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating subvolume %s: %s", subvolume, string(cmdOut))
			return err
		}
	}

	b.cfg.Logger.Debug("Create btrfs quota group")
	cmdOut, err = b.cfg.Runner.Run("btrfs", "qgroup", "create", "1/0", rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating quota group for %s: %s", rootDir, string(cmdOut))
		return err
	}

	return nil
}

func (b btrfsBackend) CreateNewSnapshot(rootDir string, baseID int) (*types.Snapshot, error) {
	var workingDir string

	newID, err := b.computeNewID(rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed computing a new snapshotID: %v", err)
		return nil, err
	}

	err = utils.MkdirAll(b.cfg.Fs, filepath.Join(rootDir, snapshotsPath, strconv.Itoa(newID)), constants.DirPerm)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, newID))

	if baseID == 0 {
		b.cfg.Logger.Debug("Creating first root filesystem as a snapshot")
		cmdOut, err := b.cfg.Runner.Run(
			"btrfs", "subvolume", "create",
			filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, newID)),
		)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating first snapshot volume: %s", string(cmdOut))
			return nil, err
		}
		workingDir = path
	} else {
		b.cfg.Logger.Debugf("Creating snapshot %d", newID)
		cmdOut, err := b.cfg.Runner.Run(
			"btrfs", "subvolume", "snapshot",
			filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, baseID)),
			filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, newID)),
		)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating first snapshot volume: %s", string(cmdOut))
			return nil, err
		}
		workingDir = filepath.Join(rootDir, snapshotsPath, strconv.Itoa(newID), snapshotWorkDir)
		err = utils.MkdirAll(b.cfg.Fs, workingDir, constants.DirPerm)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating the snapshot working directory: %v", err)
			_ = b.DeleteSnapshot(rootDir, newID)
			return nil, err
		}
	}
	snapperXML := filepath.Join(rootDir, fmt.Sprintf(snapshotInfoPath, newID))
	err = b.writeSnapperSnapshotXML(snapperXML, newSnapperSnapshotXML(newID, "first root filesystem"))
	if err != nil {
		b.cfg.Logger.Errorf("failed creating snapper info XML")
		return nil, err
	}

	return &types.Snapshot{
		ID:      newID,
		WorkDir: workingDir,
		Path:    path,
	}, nil
}

func (b btrfsBackend) CommitSnapshot(rootDir string, snapshot *types.Snapshot) error {
	err := b.clearInProgressMetadata(rootDir, snapshot.ID)
	if err != nil {
		b.cfg.Logger.Errorf("failed updating snapshot %d metadata: %v", snapshot.ID, err)
		return err
	}

	cmdOut, err := b.cfg.Runner.Run("btrfs", "property", "set", snapshot.Path, "ro", "true")
	if err != nil {
		b.cfg.Logger.Errorf("failed setting read only property to snapshot %d: %s", snapshot.ID, string(cmdOut))
		return err
	}

	subvolID, err := b.findSubvolumeByPath(rootDir, fmt.Sprintf(snapshotPathTmpl, snapshot.ID))
	if err != nil {
		b.cfg.Logger.Error("failed finding subvolume")
		return err
	}

	cmdOut, err = b.cfg.Runner.Run("btrfs", "subvolume", "set-default", strconv.Itoa(subvolID), snapshot.Path)
	if err != nil {
		b.cfg.Logger.Errorf("failed setting read only property to snapshot %d: %s", snapshot.ID, string(cmdOut))
		return err
	}
	return nil
}

func (b btrfsBackend) ListSnapshots(rootDir string) (snapshotsList, error) {
	var snaps snapshotsList

	list, err := b.getSubvolumes(rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed listing subvolumes: %v", err)
		return snaps, err
	}

	activeID, err := b.getActiveSnapshot(rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed looking for active snapshot: %v", err)
		return snaps, err
	}

	snaps.IDs = subvolumesListToSnapshotsIDs(list)
	snaps.activeID = activeID
	b.stat.activeID = activeID
	return snaps, nil
}

func (b btrfsBackend) DeleteSnapshot(rootDir string, id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid id, should be higher than zero")
	}
	if id == b.stat.currentID {
		return fmt.Errorf("invalid id, cannot delete current snapshot")
	}
	cmdOut, err := b.cfg.Runner.Run("btrfs", "subvolume", "delete", filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, id)))
	if err != nil {
		b.cfg.Logger.Errorf("failed deleting snapshot %d: %s", id, string(cmdOut))
		return err
	}
	err = utils.RemoveAll(b.cfg.Fs, filepath.Join(rootDir, snapshotsPath, strconv.Itoa(id)))
	if err != nil {
		b.cfg.Logger.Errorf("failed deleting snapshot %d folder: %v", id, err)
		return err
	}
	return nil
}

func (b btrfsBackend) SnapshotsCleanup(rootDir string) error {
	list, err := b.ListSnapshots(rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed cleaning up up snaphots, could not list them: %v", err)
		return err
	}
	snapsToDelete := len(list.IDs) - b.maxSnapshots
	if snapsToDelete > 0 {
		slices.Sort(list.IDs)
		for i := range snapsToDelete {
			if list.IDs[i] == b.stat.currentID {
				b.cfg.Logger.Warnf("current snapshot '%d' can't be cleaned up, stopping", list.IDs[i])
				break
			}
			err = b.DeleteSnapshot(rootDir, list.IDs[i])
			if err != nil {
				b.cfg.Logger.Errorf("failed cleaning up up snaphots, could delete snapshot '%d': %v", list.IDs[i], err)
				return err
			}
		}
	}
	return nil
}

func (b btrfsBackend) writeSnapperSnapshotXML(filepath string, snapshot SnapperSnapshotXML) error {
	data, err := xml.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		b.cfg.Logger.Errorf("failed marhsalling snapper's snapshot XML: %v", err)
		return err
	}
	err = b.cfg.Fs.WriteFile(filepath, data, constants.FilePerm)
	if err != nil {
		b.cfg.Logger.Errorf("failed writing snapper's snapshot XML: %v", err)
		return err
	}
	return nil
}

func (b btrfsBackend) loadSnapperSnapshotXML(filepath string) (SnapperSnapshotXML, error) {
	var data SnapperSnapshotXML

	bData, err := b.cfg.Fs.ReadFile(filepath)
	if err != nil {
		b.cfg.Logger.Errorf("failed reading '%s' file: %v", filepath, err)
		return data, err
	}

	err = xml.Unmarshal(bData, &data)
	if err != nil {
		b.cfg.Logger.Errorf("failed decoding '%s' file contents: %v", filepath, err)
		return data, err
	}

	return data, nil
}

func (b btrfsBackend) findSubvolumeByPath(rootDir, path string) (int, error) {
	subvolumes, err := b.getSubvolumes(rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed loading subvolumes: %v", err)
		return 0, err
	}

	for _, subvol := range subvolumes {
		if strings.Contains(subvol.path, path) {
			return subvol.id, nil
		}
	}

	b.cfg.Logger.Errorf("could not find subvolume with path '%s' in subvolumes list '%v'", path, subvolumes)
	return 0, fmt.Errorf("can't find subvolume '%s'", path)
}

func (b btrfsBackend) getSubvolumes(rootDir string) (btrfsSubvolList, error) {
	out, err := b.cfg.Runner.Run("btrfs", "subvolume", "list", "--sort=path", rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed listing btrfs subvolumes: %s", err.Error())
		return nil, err
	}
	return parseVolumes(strings.TrimSpace(string(out))), nil
}

func (b btrfsBackend) getActiveSnapshot(rootDir string) (int, error) {
	out, err := b.cfg.Runner.Run("btrfs", "subvolume", "get-default", rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed listing btrfs subvolumes: %s", err.Error())
		return 0, err
	}
	list := parseVolumes(strings.TrimSpace(string(out)))
	ids := subvolumesListToSnapshotsIDs(list)
	if len(ids) == 1 {
		return ids[0], nil
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return 0, fmt.Errorf("detected multiple active snapshots")
}

func parseVolumes(rawBtrfsList string) btrfsSubvolList {
	re := regexp.MustCompile(`^ID (\d+) gen \d+ top level \d+ path (.*)$`)
	list := btrfsSubvolList{}

	scanner := bufio.NewScanner(strings.NewReader(rawBtrfsList))
	for scanner.Scan() {
		match := re.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if match != nil {
			id, _ := strconv.Atoi(match[1])
			path := match[2]
			list = append(list, btrfsSubvol{id: id, path: path})
		}
	}
	return list
}

func subvolumesListToSnapshotsIDs(list btrfsSubvolList) []int {
	ids := []int{}
	re := regexp.MustCompile(snapshotPathRegex)
	for _, v := range list {
		match := re.FindStringSubmatch(v.path)
		if match != nil {
			id, _ := strconv.Atoi(match[1])
			ids = append(ids, id)
		}
	}
	return ids
}

func (b btrfsBackend) clearInProgressMetadata(rootDir string, id int) error {
	snapperXML := filepath.Join(rootDir, fmt.Sprintf(snapshotInfoPath, id))
	snapshotData, err := b.loadSnapperSnapshotXML(snapperXML)
	if err != nil {
		b.cfg.Logger.Errorf("failed reading snapshot %d metadata: %v", id, err)
		return err
	}

	var usrData []UserData
	for _, ud := range snapshotData.UserData {
		if ud.Key == updateProgress || ud.Key == installProgress {
			continue
		}
		usrData = append(usrData, ud)
	}
	snapshotData.UserData = usrData

	err = b.writeSnapperSnapshotXML(snapperXML, snapshotData)
	if err != nil {
		b.cfg.Logger.Errorf("failed writing snapshot %d metadata: %v", id, err)
		return err
	}
	return nil
}

func (b btrfsBackend) computeNewID(rootDir string) (int, error) {
	if b.stat.activeID == 0 {
		// If no active we assume this will be the first one
		return 1, nil
	}
	list, err := b.ListSnapshots(rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed computing new ID, could not list snapshots")
		return 0, err
	}
	if len(list.IDs) == 0 {
		return 0, fmt.Errorf("snapshots found, inconsistent state")
	}
	return slices.Max(list.IDs) + 1, nil
}

func (b btrfsBackend) findStateMount(device string) (rootDir string, stateMount string, snapshotID int, err error) {
	output, err := b.cfg.Runner.Run("findmnt", "-lno", "SOURCE,TARGET", device)
	if err != nil {
		return "", "", 0, err
	}
	r := regexp.MustCompile(snapshotPathRegex)

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(output))))
	for scanner.Scan() {
		lineFields := strings.Fields(scanner.Text())
		if len(lineFields) != 2 {
			continue
		}
		if strings.Contains(lineFields[1], constants.RunningStateDir) {
			stateMount = lineFields[1]
		} else if match := r.FindStringSubmatch(lineFields[0]); match != nil {
			rootDir = lineFields[1]
			snapshotID, _ = strconv.Atoi(match[1])
		}
	}

	if stateMount == "" || rootDir == "" {
		err = fmt.Errorf("could not find expected mountpoints, findmnt output: %s", string(output))
	}

	return rootDir, stateMount, snapshotID, err
}
