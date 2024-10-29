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
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ subvolumeBackend = (*snapperBackend)(nil)

type snapperBackend struct {
	cfg      *types.Config
	activeID int
	device   string
	btrfs    *btrfsBackend
}

func newSnapperBackend(cfg *types.Config) *snapperBackend {
	return &snapperBackend{cfg: cfg, btrfs: newBtrfsBackend(cfg)}
}

func (s *snapperBackend) InitBackend(device string, activeID int) {
	s.activeID = activeID
	s.device = device
	// Also include init data to the underlaying btrfsBackend for consistentcy
	s.btrfs.InitBackend(device, activeID)
}

func (s *snapperBackend) InitBrfsPartition(rootDir string) error {
	// Snapper does not support initiating a just formated btrfs partition
	return s.btrfs.InitBrfsPartition(rootDir)
}

func (s snapperBackend) CreateNewSnapshot(rootDir string, baseID int) (*types.Snapshot, error) {
	if baseID == 0 {
		// Snapper does not support creating the very first empty snapshot yet
		return s.btrfs.CreateNewSnapshot(rootDir, baseID)
	}

	s.cfg.Logger.Infof("Creating a new snapshot from %d", baseID)
	args := []string{
		"create", "--from", strconv.Itoa(baseID),
		"--read-write", "--print-number", "--description",
		fmt.Sprintf("Update for snapshot %d", baseID),
		"-c", "number", "--userdata", fmt.Sprintf("%s=yes", updateProgress),
	}
	args = append(s.rootArgs(rootDir), args...)
	cmdOut, err := s.cfg.Runner.Run("snapper", args...)
	if err != nil {
		s.cfg.Logger.Errorf("snapper failed to create a new snapshot: %v", err)
		return nil, err
	}
	newID, err := strconv.Atoi(strings.TrimSpace(string(cmdOut)))
	if err != nil {
		s.cfg.Logger.Errorf("failed parsing new snapshot ID")
		return nil, err
	}

	workingDir := filepath.Join(rootDir, snapshotsPath, strconv.Itoa(newID), snapshotWorkDir)
	err = utils.MkdirAll(s.cfg.Fs, workingDir, constants.DirPerm)
	if err != nil {
		s.cfg.Logger.Errorf("failed creating the snapshot working directory: %v", err)
		_ = s.DeleteSnapshot(rootDir, newID)
		return nil, err
	}
	path := filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, newID))
	return &types.Snapshot{
		ID:      newID,
		WorkDir: workingDir,
		Path:    path,
	}, nil
}

func (s snapperBackend) CommitSnapshot(rootDir string, snapshot *types.Snapshot) error {
	if s.activeID == 0 {
		// Snapper does not support modifying a snapshot from a host not having a configured snapper
		// and this is the case for the installation media
		return s.btrfs.CommitSnapshot(rootDir, snapshot)
	}
	args := []string{
		"modify", "--read-only", "--default", "--userdata",
		fmt.Sprintf("%s=,%s=", installProgress, updateProgress), strconv.Itoa(snapshot.ID),
	}
	args = append(s.rootArgs(rootDir), args...)
	cmdOut, err := s.cfg.Runner.Run("snapper", args...)
	if err != nil {
		s.cfg.Logger.Errorf("failed clearing userdata for snapshot %d: %s", snapshot.ID, string(cmdOut))
		return err
	}
	return nil
}

func (s snapperBackend) ListSnapshots(rootDir string) (snapshotsList, error) {
	var sl snapshotsList
	ids := []int{}
	re := regexp.MustCompile(`^(\d+),(yes|no),(yes|no)$`)

	args := []string{"--csvout", "list", "--columns", "number,default,active"}
	args = append(s.rootArgs(rootDir), args...)
	cmdOut, err := s.cfg.Runner.Run("snapper", args...)
	if err != nil {
		// snapper tries to relabel even when listing subvolumes, skip this error.
		if !strings.HasPrefix(string(cmdOut), "fsetfilecon on") {
			s.cfg.Logger.Errorf("failed collecting current snapshots: %s", string(cmdOut))
			return sl, err
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(cmdOut))))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		match := re.FindStringSubmatch(line)
		if match != nil {
			id, _ := strconv.Atoi(match[1])
			if id == 0 {
				continue
			}
			ids = append(ids, id)
			if match[2] == "yes" {
				sl.activeID = id
			}
			if match[3] == "yes" {
				sl.currentID = id
			}
		}
	}
	sl.IDs = ids

	return sl, nil
}

func (s snapperBackend) DeleteSnapshot(rootDir string, id int) error {
	if s.activeID == 0 {
		// With snapper is not possible to delete any snapshot without an active one
		return s.btrfs.DeleteSnapshot(rootDir, id)
	}
	args := []string{"delete", "--sync", strconv.Itoa(id)}
	args = append(s.rootArgs(rootDir), args...)
	cmdOut, err := s.cfg.Runner.Run("snapper", args...)
	if err != nil {
		s.cfg.Logger.Errorf("snapper failed deleting snapshot %d: %s", id, string(cmdOut))
		return err
	}
	return nil
}

func (s snapperBackend) SnapshotsCleanup(rootDir string) error {
	args := []string{"cleanup", "--path", filepath.Join(rootDir, snapshotsPath), "number"}
	args = append(s.rootArgs(rootDir), args...)
	cmdOut, err := s.cfg.Runner.Run("snapper", args...)
	if err != nil {
		s.cfg.Logger.Warnf("failed snapshots cleanup request: %s", string(cmdOut))
	}
	return err
}

func (s snapperBackend) rootArgs(rootDir string) []string {
	args := []string{}
	if rootDir != "/" {
		args = []string{"--no-dbus", "--root", filepath.Join(rootDir, fmt.Sprintf(snapshotPathTmpl, s.activeID))}
	}
	return args
}
