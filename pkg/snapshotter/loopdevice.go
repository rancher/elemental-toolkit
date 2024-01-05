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
	"sort"
	"strconv"
	"strings"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"

	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const (
	loopDeviceSnapsPath    = ".snapshots"
	loopDeviceImgName      = "snapshot.img"
	loopDeviceWorkDir      = "snapshot.workDir"
	loopDeviceLabelPattern = "EL_SNAP%d"
	loopDevicePassivePath  = loopDeviceSnapsPath + "/passives"
)

var _ v1.Snapshotter = (*LoopDevice)(nil)

type LoopDevice struct {
	cfg               v1.Config
	snapshotterCfg    v1.SnapshotterConfig
	loopDevCfg        v1.LoopDeviceConfig
	rootDir           string
	currentSnapshotID int
	activeSnapshotID  int
	bootloader        v1.Bootloader
}

func NewLoopDeviceSnapshotter(cfg v1.Config, snapCfg v1.SnapshotterConfig, bootloader v1.Bootloader) (*LoopDevice, error) {
	if snapCfg.Type != constants.LoopDeviceSnapshotterType {
		msg := "invalid snapshotter type ('%s'), must be of '%s' type"
		cfg.Logger.Errorf(msg, snapCfg.Type, constants.LoopDeviceSnapshotterType)
		return nil, fmt.Errorf(msg, snapCfg.Type, constants.LoopDeviceSnapshotterType)
	}
	loopDevCfg, ok := snapCfg.Config.(v1.LoopDeviceConfig)
	if !ok {
		msg := "failed casting LoopDeviceConfig type"
		cfg.Logger.Errorf(msg)
		return nil, fmt.Errorf(msg)
	}
	return &LoopDevice{cfg: cfg, snapshotterCfg: snapCfg, loopDevCfg: loopDevCfg, bootloader: bootloader}, nil
}

func (l *LoopDevice) InitSnapshotter(rootDir string) error {
	l.cfg.Logger.Infof("Initiating a LoopDevice snapshotter at %s", rootDir)
	l.rootDir = rootDir

	// TODO detect legacy layout in /cOS/active.img and /cOS/passive.img and create hard links to new locations

	return utils.MkdirAll(l.cfg.Fs, filepath.Join(rootDir, loopDevicePassivePath), constants.DirPerm)
}

func (l *LoopDevice) StartTransaction() (*v1.Snapshot, error) {
	l.cfg.Logger.Infof("Starting a snapshotter transaction")
	nextID, err := l.getNextSnapshotID()
	if err != nil {
		return nil, err
	}

	active, err := l.getActiveSnapshot()
	if err != nil {
		l.cfg.Logger.Errorf("failed to determine active snapshot: %v", err)
		return nil, err
	}
	if active == l.currentSnapshotID {
		l.activeSnapshotID = l.currentSnapshotID
	} else {
		l.activeSnapshotID = active
	}

	l.cfg.Logger.Debugf(
		"next snapshot: %d, current snapshot: %d, active snapshot: %d",
		nextID, l.currentSnapshotID, l.activeSnapshotID,
	)

	snapPath := filepath.Join(l.rootDir, loopDeviceSnapsPath, strconv.FormatInt(int64(nextID), 10))
	err = utils.MkdirAll(l.cfg.Fs, snapPath, constants.DirPerm)
	if err != nil {
		_ = l.cfg.Fs.RemoveAll(snapPath)
		return nil, err
	}

	workDir := filepath.Join(snapPath, loopDeviceWorkDir)
	err = utils.MkdirAll(l.cfg.Fs, workDir, constants.DirPerm)
	if err != nil {
		_ = l.cfg.Fs.RemoveAll(snapPath)
		return nil, err
	}

	err = utils.MkdirAll(l.cfg.Fs, constants.WorkingImgDir, constants.DirPerm)
	if err != nil {
		_ = l.cfg.Fs.RemoveAll(snapPath)
		return nil, err
	}

	err = l.cfg.Mounter.Mount(workDir, constants.WorkingImgDir, "bind", []string{"bind"})
	if err != nil {
		_ = l.cfg.Fs.RemoveAll(snapPath)
		_ = l.cfg.Fs.RemoveAll(constants.WorkingImgDir)
		return nil, err
	}

	snapshot := &v1.Snapshot{
		ID:         nextID,
		Path:       filepath.Join(snapPath, loopDeviceImgName),
		WorkDir:    workDir,
		MountPoint: constants.WorkingImgDir,
		Label:      fmt.Sprintf(loopDeviceLabelPattern, nextID),
		InProgress: true,
	}

	l.cfg.Logger.Infof("Transaction for snapshot %d successfully started", nextID)
	return snapshot, nil
}

func (l *LoopDevice) CloseTransactionOnError(snapshot *v1.Snapshot) error {
	var err error

	if snapshot == nil {
		return nil
	}

	if snapshot.InProgress {
		err = l.cfg.Mounter.Unmount(snapshot.MountPoint)
	}

	rErr := l.cfg.Fs.RemoveAll(filepath.Dir(snapshot.Path))
	if rErr != nil && err == nil {
		err = rErr
	}

	return err
}

func (l *LoopDevice) CloseTransaction(snapshot *v1.Snapshot) (err error) {
	var linkDst, newPassive, activeSnap string

	if !snapshot.InProgress {
		l.cfg.Logger.Debugf("No transaction to close for snapshot %d workdir", snapshot.ID)
		return l.cfg.Fs.RemoveAll(filepath.Dir(snapshot.Path))
	}
	defer func() {
		if err != nil {
			_ = l.CloseTransactionOnError(snapshot)
		}
	}()

	l.cfg.Logger.Infof("Closing transaction for snapshot %d workdir", snapshot.ID)
	l.cfg.Logger.Debugf("Unmount %s", constants.WorkingImgDir)
	err = l.cfg.Mounter.Unmount(snapshot.MountPoint)
	if err != nil {
		l.cfg.Logger.Errorf("failed umounting snapshot %d workdir bind mount", snapshot.ID)
		return err
	}

	err = elemental.CreateImageFromTree(l.cfg, l.snapshotToImage(snapshot), snapshot.WorkDir, false)
	if err != nil {
		l.cfg.Logger.Errorf("failed creating image for snapshot %d: %v", snapshot.ID, err)
		return err
	}

	err = l.cfg.Fs.RemoveAll(snapshot.WorkDir)
	if err != nil {
		return err
	}

	// Create fallback link for current active snapshot
	newPassive = filepath.Join(l.rootDir, loopDevicePassivePath, fmt.Sprintf(constants.PassiveSnapshot, l.activeSnapshotID))
	if l.activeSnapshotID > 0 {
		linkDst = fmt.Sprintf("../%d/%s", l.activeSnapshotID, loopDeviceImgName)
		l.cfg.Logger.Debugf("creating symlink %s to %s", newPassive, linkDst)
		err = l.cfg.Fs.Symlink(linkDst, newPassive)
		if err != nil {
			l.cfg.Logger.Errorf("failed creating the new passive link: %v", err)
			return err
		}
		l.cfg.Logger.Infof("New passive snapshot fallback from active (%d) created", l.activeSnapshotID)
	}

	// Remove old symlink and create a new one
	activeSnap = filepath.Join(l.rootDir, loopDeviceSnapsPath, constants.ActiveSnapshot)
	linkDst = fmt.Sprintf("%d/%s", snapshot.ID, loopDeviceImgName)
	l.cfg.Logger.Debugf("creating symlink %s to %s", activeSnap, linkDst)
	_ = l.cfg.Fs.Remove(activeSnap)
	err = l.cfg.Fs.Symlink(linkDst, activeSnap)
	if err != nil {
		l.cfg.Logger.Errorf("failed default snapshot image for snapshot %d: %v", snapshot.ID, err)
		_ = l.cfg.Fs.Remove(newPassive)
		sErr := l.cfg.Fs.Symlink(fmt.Sprintf("%d/%s", l.activeSnapshotID, loopDeviceImgName), activeSnap)
		if sErr != nil {
			l.cfg.Logger.Warnf("could not restore previous active link")
		}
		return err
	}
	// From now on we do not error out as the transaction is already done, cleanup steps are only logged
	// Active system does not require specific bootloader setup, only old snapshots
	_ = l.cleanOldSnapshots()
	_ = l.setBootloader()

	snapshot.InProgress = false
	return err
}

func (l *LoopDevice) DeleteSnapshot(id int) error {
	var err error
	var snapLink string

	l.cfg.Logger.Infof("Deleting snapshot %d", id)
	inUse, err := l.isSnapshotInUse(id)
	if err != nil {
		return err
	}

	if inUse {
		return fmt.Errorf("cannot delete a snapshot that is currently in use")
	}

	snaps, err := l.GetSnapshots()
	if err != nil {
		l.cfg.Logger.Errorf("failed getting current snapshots list: %v", err)
		return err
	}

	found := false
	for _, snap := range snaps {
		if snap == id {
			found = true
			break
		}
	}
	if !found {
		l.cfg.Logger.Warnf("Snapshot %d not found, nothing to delete", id)
		return nil
	}

	if l.activeSnapshotID == id {
		snapLink = filepath.Join(l.rootDir, loopDeviceSnapsPath, constants.ActiveSnapshot)
	} else {
		snapLink = filepath.Join(l.rootDir, loopDevicePassivePath, fmt.Sprintf(constants.PassiveSnapshot, id))
	}

	err = l.cfg.Fs.Remove(snapLink)
	if err != nil {
		l.cfg.Logger.Errorf("failed removing snapshot link %s: %v", snapLink, err)
		return err
	}

	snapDir := filepath.Join(l.rootDir, loopDeviceSnapsPath, strconv.Itoa(id))
	err = l.cfg.Fs.RemoveAll(snapDir)
	if err != nil {
		l.cfg.Logger.Errorf("failed removing snaphot dir %s: %v", snapDir, err)
	}
	return err
}

func (l *LoopDevice) GetSnapshots() ([]int, error) {
	var ids []int

	ids, err := l.getPassiveSnapshots()
	if err != nil {
		return nil, err
	}

	id, err := l.getActiveSnapshot()
	if err != nil {
		return nil, err
	}

	if id > 0 {
		return append(ids, id), nil
	}
	return ids, nil
}

func (l *LoopDevice) getNextSnapshotID() (int, error) {
	var id int

	ids, err := l.GetSnapshots()
	if err != nil {
		return -1, err
	}
	for _, i := range ids {
		inUse, err := l.isSnapshotInUse(i)
		if err != nil {
			l.cfg.Logger.Errorf("failed checking if snapshot %d is in use: %v", i, err)
			return -1, err
		}
		if inUse {
			l.cfg.Logger.Debugf("Current snapshot: %d", i)
			l.currentSnapshotID = i
		}
		if i > id {
			id = i
		}
	}
	return id + 1, nil
}

func (l *LoopDevice) getActiveSnapshot() (int, error) {
	snapPath := filepath.Join(l.rootDir, loopDeviceSnapsPath, constants.ActiveSnapshot)
	exists, err := utils.Exists(l.cfg.Fs, snapPath, true)
	if err != nil {
		l.cfg.Logger.Errorf("failed checking active snapshot (%s) existence: %v", snapPath, err)
		return -1, err
	}
	if !exists {
		l.cfg.Logger.Infof("path %s does not exist", snapPath)
		return 0, nil
	}

	resolved, err := utils.ResolveLink(l.cfg.Fs, snapPath, l.rootDir, constants.MaxLinkDepth)
	if err != nil {
		l.cfg.Logger.Errorf("failed to resolve %s link", snapPath)
		return -1, err
	}

	id, err := strconv.ParseInt(filepath.Base(filepath.Dir(resolved)), 10, 32)
	if err != nil {
		l.cfg.Logger.Errorf("failed parsing snapshot ID from path %s: %v", resolved, err)
		return -1, err
	}

	return int(id), nil
}

func (l *LoopDevice) isSnapshotInUse(id int) (bool, error) {
	backedFiles, err := l.cfg.Runner.Run("losetup", "-ln", "--output", "BACK-FILE")
	if err != nil {
		return false, err
	}

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(backedFiles))))
	for scanner.Scan() {
		backedFile := scanner.Text()
		suffix := filepath.Join(loopDeviceSnapsPath, strconv.Itoa(id), loopDeviceImgName)
		if strings.HasSuffix(backedFile, suffix) {
			return true, nil
		}
	}
	return false, nil
}

func (l *LoopDevice) snapshotToImage(snapshot *v1.Snapshot) *v1.Image {
	return &v1.Image{
		File:       snapshot.Path,
		Label:      snapshot.Label,
		Size:       l.loopDevCfg.Size,
		FS:         l.loopDevCfg.FS,
		MountPoint: snapshot.MountPoint,
	}
}

func (l *LoopDevice) cleanOldSnapshots() error {
	l.cfg.Logger.Infof("Cleaning old passive snapshots")
	ids, err := l.GetSnapshots()
	if err != nil {
		l.cfg.Logger.Warnf("could not get current snapshots")
		return err
	}

	sort.Ints(ids)
	for len(ids) > l.snapshotterCfg.MaxSnaps {
		err = l.DeleteSnapshot(ids[0])
		if err != nil {
			l.cfg.Logger.Warnf("could not delete snapshot %d", ids[0])
			return err
		}
		ids = ids[1:]
	}
	return nil
}

func (l *LoopDevice) setBootloader() error {
	var passives, fallbacks []string

	l.cfg.Logger.Infof("Setting bootloader with current passive snapshots")
	ids, err := l.getPassiveSnapshots()
	if err != nil {
		l.cfg.Logger.Warnf("failed getting current passive snapshots: %v", err)
		return err
	}
	for _, id := range ids {
		passives = append(passives, fmt.Sprintf(constants.PassiveSnapshot, id))
	}

	// We count first is active, then all passives and finally the recovery
	for i := 0; i <= len(ids)+1; i++ {
		fallbacks = append(fallbacks, strconv.Itoa(i))
	}
	snapsList := strings.Join(passives, " ")
	fallbackList := strings.Join(fallbacks, " ")
	envFile := filepath.Join(l.rootDir, constants.GrubOEMEnv)

	envs := map[string]string{
		constants.GrubFallback:         fallbackList,
		constants.GrubPassiveSnapshots: snapsList,
	}

	err = l.bootloader.SetPersistentVariables(envFile, envs)
	if err != nil {
		l.cfg.Logger.Warnf("failed setting bootloader environment file %s: %v", envFile, err)
		return err
	}

	return err
}

func (l *LoopDevice) getPassiveSnapshots() ([]int, error) {
	var ids []int

	snapsPath := filepath.Join(l.rootDir, loopDevicePassivePath)
	r := regexp.MustCompile(`passive_(\d+)`)
	if ok, _ := utils.Exists(l.cfg.Fs, snapsPath); ok {
		links, err := l.cfg.Fs.ReadDir(snapsPath)
		if err != nil {
			l.cfg.Logger.Errorf("failed reading %s contents", snapsPath)
			return ids, err
		}
		for _, link := range links {
			// Find snapshots based numeric directory names
			if !r.MatchString(link.Name()) {
				continue
			}
			matches := r.FindStringSubmatch(link.Name())
			id, err := strconv.ParseInt(matches[1], 10, 32)
			if err != nil {
				continue
			}
			linkPath := filepath.Join(snapsPath, link.Name())
			if exists, _ := utils.Exists(l.cfg.Fs, linkPath); exists {
				ids = append(ids, int(id))
			} else {
				l.cfg.Logger.Warnf("image for snapshot %d doesn't exist", id)
			}
		}
		l.cfg.Logger.Debugf("Passive snaps: %v", ids)
		return ids, nil
	}
	l.cfg.Logger.Errorf("path %s does not exist", snapsPath)
	return ids, fmt.Errorf("cannot determine passive snapshots, initate snapshotter first")
}
