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
	"strconv"
	"strings"
	"time"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
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
	cfg      *types.Config
	activeID int
	device   string
}

func newBtrfsBackend(cfg *types.Config) *btrfsBackend {
	return &btrfsBackend{cfg: cfg}
}

func (b *btrfsBackend) InitBackend(device string, activeID int) {
	b.activeID = activeID
	b.device = device
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
	if baseID == 0 {
		b.cfg.Logger.Info("Creating first root filesystem as a snapshot")
		newID := 1
		err := utils.MkdirAll(b.cfg.Fs, filepath.Join(rootDir, snapshotsPath, strconv.Itoa(newID)), constants.DirPerm)
		if err != nil {
			return nil, err
		}
		cmdOut, err := b.cfg.Runner.Run(
			"btrfs", "subvolume", "create",
			filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, newID)),
		)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating first snapshot volume: %s", string(cmdOut))
			return nil, err
		}
		snapperXML := filepath.Join(rootDir, fmt.Sprintf(snapshotInfoPath, newID))
		err = b.writeSnapperSnapshotXML(snapperXML, newSnapperSnapshotXML(newID, "first root filesystem"))
		if err != nil {
			b.cfg.Logger.Errorf("failed creating snapper info XML")
			return nil, err
		}
		workingDir := filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, newID))
		return &types.Snapshot{
			ID:      newID,
			WorkDir: workingDir,
			Path:    workingDir,
		}, nil
	}

	return nil, fmt.Errorf("not implemented yet")
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

func (b btrfsBackend) ListSnapshots(_ string) (snapshotsList, error) {
	var snaps snapshotsList
	return snaps, fmt.Errorf("not implemented yet")
}

func (b btrfsBackend) DeleteSnapshot(_ string, _ int) error {
	return fmt.Errorf("not implemented yet")
}

func (b btrfsBackend) SnapshotsCleanup(_ string) error {
	return fmt.Errorf("not implemented yet")
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
	return b.parseVolumes(strings.TrimSpace(string(out))), nil
}

func (b btrfsBackend) parseVolumes(rawBtrfsList string) btrfsSubvolList {
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
