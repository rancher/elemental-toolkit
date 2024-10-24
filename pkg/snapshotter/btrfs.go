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

const (
	rootSubvol        = "@"
	snapshotsPath     = ".snapshots"
	snapshotPathTmpl  = ".snapshots/%d/snapshot"
	snapshotPathRegex = `.snapshots/(\d+)/snapshot`
	snapshotInfoPath  = ".snapshots/%d/info.xml"
	snapshotWorkDir   = "snapshot.workDir"
	dateFormat        = "2006-01-02 15:04:05"
	snapperRootConfig = "/etc/snapper/configs/root"
	snapperSysconfig  = "/etc/sysconfig/snapper"
)

func configTemplatesPaths() []string {
	return []string{
		"/etc/snapper/config-templates/default",
		"/usr/share/snapper/config-templates/default",
	}
}

var _ types.Snapshotter = (*Btrfs)(nil)

type Btrfs struct {
	cfg              types.Config
	snapshotterCfg   types.SnapshotterConfig
	btrfsCfg         types.BtrfsConfig
	rootDir          string
	efiDir           string
	activeSnapshotID int
	bootloader       types.Bootloader
	installing       bool
	snapperArgs      []string
	snapshotsUmount  func() error
	snapshotsMount   func() error
}

type btrfsSubvol struct {
	path string
	id   int
}

type btrfsSubvolList []btrfsSubvol

type Date time.Time

type SnapperSnapshotXML struct {
	XMLName     xml.Name `xml:"snapshot"`
	Type        string   `xml:"type"`
	Num         int      `xml:"num"`
	Date        Date     `xml:"date"`
	Cleanup     string   `xml:"cleanup"`
	Description string   `xml:"description"`
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

// NewLoopDeviceSnapshotter creates a new loop device snapshotter vased on the given configuration and the given bootloader
func newBtrfsSnapshotter(cfg types.Config, snapCfg types.SnapshotterConfig, bootloader types.Bootloader) (types.Snapshotter, error) {
	if snapCfg.Type != constants.BtrfsSnapshotterType {
		msg := "invalid snapshotter type ('%s'), must be of '%s' type"
		cfg.Logger.Errorf(msg, snapCfg.Type, constants.BtrfsSnapshotterType)
		return nil, fmt.Errorf(msg, snapCfg.Type, constants.BtrfsSnapshotterType)
	}
	var btrfsCfg *types.BtrfsConfig
	var ok bool
	if snapCfg.Config == nil {
		btrfsCfg = types.NewBtrfsConfig()
	} else {
		btrfsCfg, ok = snapCfg.Config.(*types.BtrfsConfig)
		if !ok {
			msg := "failed casting BtrfsConfig type"
			cfg.Logger.Errorf(msg)
			return nil, fmt.Errorf("%s", msg)
		}
		if !btrfsCfg.DisableSnapper && btrfsCfg.DisableDefaultSubVolume {
			msg := "requested snapshotter configuration is invalid"
			cfg.Logger.Errorf(msg)
			return nil, fmt.Errorf("%s", msg)
		}
	}
	return &Btrfs{
		cfg: cfg, snapshotterCfg: snapCfg, btrfsCfg: *btrfsCfg,
		bootloader: bootloader, snapshotsUmount: func() error { return nil },
		snapshotsMount: func() error { return nil },
	}, nil
}

func (b *Btrfs) InitSnapshotter(state *types.Partition, efiDir string) error {
	var err error
	var ok bool

	b.cfg.Logger.Infof("Initiate btrfs snapshotter at %s", state.MountPoint)
	b.rootDir = state.MountPoint
	b.efiDir = efiDir

	b.cfg.Logger.Debug("Checking if essential subvolumes are already created")
	ok, err = b.isInitiated(state.MountPoint)
	if ok && elemental.IsActiveMode(b.cfg) || elemental.IsPassiveMode(b.cfg) {
		return b.configureSnapperAndRootDir(state)
	}
	if err != nil {
		b.cfg.Logger.Errorf("failed loading initial snapshotter state: %v")
		return err
	}

	if !ok {
		b.installing = true
		b.cfg.Logger.Debug("Running initial btrfs configuration")
		err = b.setBtrfsForFirstTime(state)
		if err != nil {
			return err
		}
	}

	b.cfg.Logger.Debug("Remount state partition at root subvolume")
	return b.remountStatePartition(state)
}

func (b *Btrfs) StartTransaction() (*types.Snapshot, error) {
	var newID int
	var err error
	var workingDir, path string
	snapshot := &types.Snapshot{}

	b.cfg.Logger.Info("Starting a btrfs snapshotter transaction")

	if !b.installing && b.activeSnapshotID == 0 {
		b.cfg.Logger.Errorf("Snapshotter should have been initalized before starting a transaction")
		return nil, fmt.Errorf("uninitialized snapshotter")
	}

	if !b.btrfsCfg.DisableSnapper {
		if !b.installing {
			b.cfg.Logger.Infof("Creating a new snapshot from %d", b.activeSnapshotID)
			args := []string{
				"create", "--from", strconv.Itoa(b.activeSnapshotID),
				"--read-write", "--print-number", "--description",
				fmt.Sprintf("Update for snapshot %d", b.activeSnapshotID),
				"-c", "number", "--userdata", "update-in-progress=yes",
			}
			args = append(b.snapperArgs, args...)
			cmdOut, err := b.cfg.Runner.Run("snapper", args...)
			if err != nil {
				b.cfg.Logger.Errorf("snapper failed to create a new snapshot: %v", err)
				return nil, err
			}
			newID, err = strconv.Atoi(strings.TrimSpace(string(cmdOut)))
			if err != nil {
				b.cfg.Logger.Errorf("failed parsing new snapshot ID")
				return nil, err
			}

			workingDir = filepath.Join(b.rootDir, snapshotsPath, strconv.Itoa(newID), snapshotWorkDir)
			err = utils.MkdirAll(b.cfg.Fs, workingDir, constants.DirPerm)
			if err != nil {
				b.cfg.Logger.Errorf("failed creating the snapshot working directory: %v", err)
				_ = b.DeleteSnapshot(newID)
				return nil, err
			}
			path = filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, newID))
		} else {
			b.cfg.Logger.Info("Creating first root filesystem as a snapshot")
			newID = 1
			err = utils.MkdirAll(b.cfg.Fs, filepath.Join(b.rootDir, snapshotsPath, strconv.Itoa(newID)), constants.DirPerm)
			if err != nil {
				return nil, err
			}
			cmdOut, err := b.cfg.Runner.Run(
				"btrfs", "subvolume", "create",
				filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, newID)),
			)
			if err != nil {
				b.cfg.Logger.Errorf("failed creating first snapshot volume: %s", string(cmdOut))
				return nil, err
			}
			snapperXML := filepath.Join(b.rootDir, fmt.Sprintf(snapshotInfoPath, newID))
			err = b.writeSnapperSnapshotXML(snapperXML, firstSnapperSnapshotXML())
			if err != nil {
				b.cfg.Logger.Errorf("failed creating snapper info XML")
				return nil, err
			}
			workingDir = filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, newID))
			path = workingDir
		}
	} else {
		ids, err := b.GetSnapshots()
		if err != nil {
			b.cfg.Logger.Errorf("unable to get btrfs snapshots")
			return nil, err
		}

		// minimum ID (in case of initial installation)
		newID = 1

		for _, id := range ids {
			// search for next ID to be used
			newID = max(id+1, newID)
		}

		// compute snapshot workdir and path
		path = filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, newID))
		workingDir = path + ".inprogress"

		// create tree up to snapshot subvolume
		err = utils.MkdirAll(b.cfg.Fs, filepath.Dir(path), constants.DirPerm)
		if err != nil {
			return nil, err
		}

		if b.activeSnapshotID == 0 {
			cmdOut, err := b.cfg.Runner.Run("btrfs", "subvolume", "create", workingDir)
			if err != nil {
				b.cfg.Logger.Errorf("failed creating first snapshot volume: %s", string(cmdOut))
				_ = b.DeleteSnapshot(newID)
				return nil, err
			}
		} else {
			source := filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, b.activeSnapshotID))
			cmdOut, err := b.cfg.Runner.Run("btrfs", "subvolume", "snapshot", source, workingDir)
			if err != nil {
				b.cfg.Logger.Errorf("failed creating snapshot volume: %s", string(cmdOut))
				_ = b.DeleteSnapshot(newID)
				return nil, err
			}
		}
	}

	err = utils.MkdirAll(b.cfg.Fs, constants.WorkingImgDir, constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating working tree directory: %s", constants.WorkingImgDir)
		_ = b.DeleteSnapshot(newID)
		return nil, err
	}

	err = b.cfg.Mounter.Mount(workingDir, constants.WorkingImgDir, "bind", []string{"bind"})
	if err != nil {
		_ = b.DeleteSnapshot(newID)
		return nil, err
	}
	snapshot.MountPoint = constants.WorkingImgDir
	snapshot.ID = newID
	snapshot.InProgress = true
	snapshot.WorkDir = workingDir
	snapshot.Path = path

	return snapshot, err
}

func (b *Btrfs) CloseTransactionOnError(snapshot *types.Snapshot) (err error) {
	if snapshot.InProgress {
		err = b.cfg.Mounter.Unmount(snapshot.MountPoint)
	}
	defer func() {
		newErr := b.snapshotsUmount()
		if err == nil {
			err = newErr
		}
	}()
	err = b.DeleteSnapshot(snapshot.ID)
	return err
}

func (b *Btrfs) CloseTransaction(snapshot *types.Snapshot) (err error) {
	var cmdOut []byte
	var subvolID int

	if !snapshot.InProgress {
		b.cfg.Logger.Debugf("No transaction to close for snapshot %d workdir", snapshot.ID)
		return fmt.Errorf("given snapshot is not in progress")
	}
	defer func() {
		if err != nil {
			_ = b.DeleteSnapshot(snapshot.ID)
		}
		newErr := b.snapshotsUmount()
		if err == nil {
			err = newErr
		}
	}()
	b.cfg.Logger.Infof("Closing transaction for snapshot %d workdir", snapshot.ID)

	// Make sure snapshots mountpoint folder is part of the resulting snapshot image
	err = utils.MkdirAll(b.cfg.Fs, filepath.Join(snapshot.WorkDir, snapshotsPath), constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating snapshots folder: %v", err)
		return err
	}

	if !b.btrfsCfg.DisableSnapper {
		// Configure snapper
		err = b.configureSnapper(snapshot)
		if err != nil {
			b.cfg.Logger.Errorf("failed configuring snapper: %v", err)
			return err
		}

		b.cfg.Logger.Debugf("Unmount %s", snapshot.MountPoint)
		err = b.cfg.Mounter.Unmount(snapshot.MountPoint)
		if err != nil {
			b.cfg.Logger.Errorf("failed umounting snapshot %d workdir bind mount", snapshot.ID)
			return err
		}

		if !b.installing {
			err = utils.MirrorData(b.cfg.Logger, b.cfg.Runner, b.cfg.Fs, snapshot.WorkDir, snapshot.Path)
			if err != nil {
				b.cfg.Logger.Errorf("failed syncing working directory with snapshot directory")
				return err
			}

			err = b.cfg.Fs.RemoveAll(snapshot.WorkDir)
			if err != nil {
				b.cfg.Logger.Errorf("failed deleting snapshot's workdir '%s': %s", snapshot.WorkDir, err)
				return err
			}

			args := []string{"modify", "--userdata", "update-in-progress=no", strconv.Itoa(snapshot.ID)}
			args = append(b.snapperArgs, args...)
			cmdOut, err = b.cfg.Runner.Run("snapper", args...)
			if err != nil {
				b.cfg.Logger.Errorf("failed setting read only property to snapshot %d: %s", snapshot.ID, string(cmdOut))
				return err
			}
		}
	} else {
		b.cfg.Logger.Debugf("Unmount %s", snapshot.MountPoint)
		err = b.cfg.Mounter.Unmount(snapshot.MountPoint)
		if err != nil {
			b.cfg.Logger.Errorf("failed umounting snapshot %d workdir bind mount", snapshot.ID)
			return err
		}

		// rename subvolume to its definitive name
		err := b.cfg.Fs.Rename(snapshot.WorkDir, snapshot.Path)
		if err != nil {
			b.cfg.Logger.Errorf("unable to set snapshot to its definitive path %d: %v", snapshot.ID, err)
			return err
		}
	}

	extraBind := map[string]string{filepath.Join(b.rootDir, snapshotsPath): filepath.Join("/", snapshotsPath)}
	err = elemental.ApplySELinuxLabels(b.cfg, snapshot.Path, extraBind)
	if err != nil {
		b.cfg.Logger.Errorf("failed relabelling snapshot path: %s", snapshot.Path)
		return err
	}

	cmdOut, err = b.cfg.Runner.Run("btrfs", "property", "set", snapshot.Path, "ro", "true")
	if err != nil {
		b.cfg.Logger.Errorf("failed setting read only property to snapshot %d: %s", snapshot.ID, string(cmdOut))
		return err
	}

	if !b.btrfsCfg.DisableDefaultSubVolume {
		subvolID, err = b.findSubvolumeByPath(fmt.Sprintf(snapshotPathTmpl, snapshot.ID))
		if err != nil {
			b.cfg.Logger.Error("failed finding subvolume")
			return err
		}

		cmdOut, err = b.cfg.Runner.Run("btrfs", "subvolume", "set-default", strconv.Itoa(subvolID), snapshot.Path)
		if err != nil {
			b.cfg.Logger.Errorf("failed setting default subvolume property to snapshot %d: %s", snapshot.ID, string(cmdOut))
			return err
		}
	} else {
		_, _, _, stateDir, err := findStateMount(b.cfg.Runner, b.rootDir)
		if err != nil {
			b.cfg.Logger.Errorf("unable to find btrfs state directory: %s", err.Error())
			return err
		}

		activeSnap := filepath.Join(stateDir, constants.ActiveSnapshot)
		linkDst := fmt.Sprintf(snapshotPathTmpl, snapshot.ID)
		b.cfg.Logger.Debugf("creating symlink %s to %s", activeSnap, linkDst)
		_ = b.cfg.Fs.Remove(activeSnap)
		err = b.cfg.Fs.Symlink(linkDst, activeSnap)
		if err != nil {
			b.cfg.Logger.Errorf("failed default snapshot image for snapshot %d: %v", snapshot.ID, err)
			sErr := b.cfg.Fs.Symlink(fmt.Sprintf(snapshotPathTmpl, b.activeSnapshotID), activeSnap)
			if sErr != nil {
				b.cfg.Logger.Warnf("could not restore previous active link")
			}
			return err
		}
	}

	_ = b.setBootloader()

	if (!b.btrfsCfg.DisableSnapper) && (!b.installing) {
		args := []string{"cleanup", "--path", filepath.Join(b.rootDir, snapshotsPath), "number"}
		args = append(b.snapperArgs, args...)
		_, _ = b.cfg.Runner.Run("snapper", args...)
	}

	return nil
}

func (b *Btrfs) DeleteSnapshot(id int) error {
	var cmdOut []byte

	b.cfg.Logger.Infof("Deleting snapshot %d", id)

	snapshots, err := b.GetSnapshots()
	if err != nil {
		b.cfg.Logger.Errorf("failed listing available snapshots: %v", err)
		return err
	}
	if !slices.Contains(snapshots, id) {
		b.cfg.Logger.Debugf("snapshot %d not found, nothing has been deleted", id)
		return nil
	}

	if !b.btrfsCfg.DisableSnapper {
		args := []string{"delete", "--sync", strconv.Itoa(id)}
		args = append(b.snapperArgs, args...)
		cmdOut, err = b.cfg.Runner.Run("snapper", args...)
		if err != nil {
			b.cfg.Logger.Errorf("snapper failed deleting snapshot %d: %s", id, string(cmdOut))
			return err
		}
	} else {
		// Remove btrfs subvolume first
		basePath := filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, id))
		snapshotDir := basePath
		if ok, err := utils.Exists(b.cfg.Fs, snapshotDir, false); !ok {
			snapshotDir = basePath + ".inprogress"
		} else if err != nil {
			b.cfg.Logger.Errorf("unable to stat snapshot subvolume %d: %s", id, snapshotDir)
			return err
		}

		if ok, err := utils.Exists(b.cfg.Fs, snapshotDir, false); ok {
			args := []string{"subvolume", "delete", "-c", snapshotDir}
			cmdOut, err = b.cfg.Runner.Run("btrfs", args...)
			if err != nil {
				b.cfg.Logger.Errorf("failed deleting snapshot subvolume %d: %s", id, string(cmdOut))
				return err
			}
		} else if err != nil {
			b.cfg.Logger.Errorf("unable to stat snapshot subvolume %d: %s", id, snapshotDir)
			return err
		} else {
			b.cfg.Logger.Warnf("no snapshot subvolume %d exists", id)
		}

		// then remove associated directory
		parent := filepath.Dir(snapshotDir)
		err = b.cfg.Fs.RemoveAll(parent)
		if err != nil {
			b.cfg.Logger.Errorf("failed deleting snapshot parent directory '%s': %s", parent, err)
			return err
		}
	}

	return nil
}

func (b *Btrfs) GetSnapshots() (snapshots []int, err error) {
	if !b.btrfsCfg.DisableSnapper {
		var ok bool

		if ok, err = b.isInitiated(b.rootDir); ok {
			// Check if snapshots subvolume is mounted
			snapshotsSubolume := filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, b.activeSnapshotID), snapshotsPath)
			if notMnt, _ := b.cfg.Mounter.IsLikelyNotMountPoint(snapshotsSubolume); notMnt {
				err = b.snapshotsMount()
				if err != nil {
					return nil, err
				}
				defer func() {
					nErr := b.snapshotsUmount()
					if err == nil && nErr != nil {
						err = nErr
						snapshots = nil
					}
				}()
			}
			snapshots, err = b.loadSnapshots()
			if err != nil {
				return nil, err
			}
			return snapshots, err
		} else if err != nil {
			return nil, err
		}
	} else {
		// btrfs subvolume list is not safe here. Use snapshot directory
		_, _, snapshotDir, _, err := findStateMount(b.cfg.Runner, b.rootDir)
		if err != nil {
			b.cfg.Logger.Errorf("unable to find btrfs snapshots directory: %v", err)
			return nil, err
		}

		list, err := b.cfg.Fs.ReadDir(snapshotDir)
		if err != nil {
			b.cfg.Logger.Errorf("failed listing btrfs snapshots directory: %v", err)
			return nil, err
		}

		re := regexp.MustCompile(`^\d+$`)
		ids := []int{}
		for _, entry := range list {
			if entry.IsDir() {
				entryName := entry.Name()

				if re.MatchString(entryName) {
					exists, _ := utils.Exists(b.cfg.Fs, filepath.Join(snapshotDir, entryName, "snapshot"), false)
					inprogress, _ := utils.Exists(b.cfg.Fs, filepath.Join(snapshotDir, entryName, "snapshot.inprogress"), false)
					if exists || inprogress {
						id, _ := strconv.Atoi(entryName)
						ids = append(ids, id)
					}
				}
			}
		}
		return ids, nil
	}

	return []int{}, err
}

func (b *Btrfs) loadSnapshots() ([]int, error) {
	ids := []int{}
	re := regexp.MustCompile(`^(\d+),(yes|no),(yes|no)$`)

	args := []string{"--csvout", "list", "--columns", "number,default,active"}
	args = append(b.snapperArgs, args...)
	cmdOut, err := b.cfg.Runner.Run("snapper", args...)
	if err != nil {
		// snapper tries to relabel even when listing subvolumes, skip this error.
		if !strings.HasPrefix(string(cmdOut), "fsetfilecon on") {
			b.cfg.Logger.Errorf("failed collecting current snapshots: %s", string(cmdOut))
			return nil, err
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
				b.activeSnapshotID = id
			}
		}
	}

	return ids, nil
}

// SnapshotImageToSource converts the given snapshot into an ImageSource. This is useful to deploy a system
// from a given snapshot, for instance setting the recovery image from a snapshot.
func (b *Btrfs) SnapshotToImageSource(snap *types.Snapshot) (*types.ImageSource, error) {
	ok, err := utils.Exists(b.cfg.Fs, snap.Path)
	if err != nil || !ok {
		msg := fmt.Sprintf("snapshot path does not exist: %s.", snap.Path)
		b.cfg.Logger.Errorf(msg)
		if err == nil {
			err = fmt.Errorf("%s", msg)
		}
		return nil, err
	}
	return types.NewDirSrc(snap.Path), nil
}

func (b *Btrfs) getSubvolumes(rootDir string) (btrfsSubvolList, error) {
	out, err := b.cfg.Runner.Run("btrfs", "subvolume", "list", "--sort=path", rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed listing btrfs subvolumes: %s", err.Error())
		return nil, err
	}
	return b.parseVolumes(strings.TrimSpace(string(out))), nil
}

func (b *Btrfs) getActiveSnapshot() (int, error) {
	if !b.btrfsCfg.DisableDefaultSubVolume {
		out, err := b.cfg.Runner.Run("btrfs", "subvolume", "get-default", b.rootDir)
		if err != nil {
			b.cfg.Logger.Errorf("failed listing btrfs subvolumes: %s", err.Error())
			return 0, err
		}
		re := regexp.MustCompile(snapshotPathRegex)
		list := b.parseVolumes(strings.TrimSpace(string(out)))
		for _, v := range list {
			match := re.FindStringSubmatch(v.path)
			if match != nil {
				id, _ := strconv.Atoi(match[1])
				return id, nil
			}
		}
	} else {
		_, _, _, stateDir, err := findStateMount(b.cfg.Runner, b.rootDir)
		if err != nil {
			b.cfg.Logger.Errorf("unable to find btrfs sate directory: %s", err.Error())
			return 0, err
		}

		activeSnap := filepath.Join(stateDir, constants.ActiveSnapshot)
		activePath, err := b.cfg.Fs.Readlink(activeSnap)
		if err != nil {
			b.cfg.Logger.Errorf("failed reading active symlink %s: %s", activeSnap, err.Error())
			return 0, err
		}
		b.cfg.Logger.Debugf("active snapshot path is %s", activePath)

		re := regexp.MustCompile(snapshotPathRegex)
		match := re.FindStringSubmatch(activePath)
		if match != nil {
			id, _ := strconv.Atoi(match[1])
			return id, nil
		}
	}
	return 0, nil
}

func (b *Btrfs) parseVolumes(rawBtrfsList string) btrfsSubvolList {
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

func (b *Btrfs) getStateSubvolumes(rootDir string) (rootVolume *btrfsSubvol, snapshotsVolume *btrfsSubvol, err error) {
	volumes, err := b.getSubvolumes(rootDir)
	if err != nil {
		return nil, nil, err
	}

	snapshots := filepath.Join(rootSubvol, snapshotsPath)

	b.cfg.Logger.Debugf(
		"Looking for subvolumes %s and %s in subvolume list: %v",
		rootSubvol, snapshots, volumes,
	)
	for _, vol := range volumes {
		if vol.path == rootSubvol {
			rootVolume = &vol
		} else if vol.path == snapshots {
			snapshotsVolume = &vol
		}
	}

	return rootVolume, snapshotsVolume, err
}

func (b *Btrfs) isInitiated(rootDir string) (bool, error) {
	if b.activeSnapshotID > 0 {
		return true, nil
	}
	if b.installing {
		return false, nil
	}

	rootVolume, snapshotsVolume, err := b.getStateSubvolumes(rootDir)
	if err != nil {
		return false, err
	}

	if (rootVolume != nil) && (snapshotsVolume != nil) {
		id, err := b.getActiveSnapshot()
		if err != nil {
			return false, err
		}
		if id > 0 {
			b.activeSnapshotID = id
			return true, nil
		}
	}

	return false, nil
}

func firstSnapperSnapshotXML() SnapperSnapshotXML {
	return SnapperSnapshotXML{
		Type:        "single",
		Num:         1,
		Date:        Date(time.Now()),
		Description: "first root filesystem",
		Cleanup:     "number",
	}
}

func (b *Btrfs) writeSnapperSnapshotXML(filepath string, snapshot SnapperSnapshotXML) error {
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

func (b *Btrfs) findSubvolumeByPath(path string) (int, error) {
	subvolumes, err := b.getSubvolumes(b.rootDir)
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

func (b *Btrfs) getPassiveSnapshots() ([]int, error) {
	passives := []int{}

	active, err := b.getActiveSnapshot()
	if err != nil {
		b.cfg.Logger.Warnf("failed getting current active snapshot: %v", err)
		return nil, err
	}

	snapshots, err := b.GetSnapshots()
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if snapshot != active {
			passives = append(passives, snapshot)
		}
	}

	return passives, nil
}

// setBootloader sets the bootloader variables to update new passives
func (b *Btrfs) setBootloader() error {
	var passives, fallbacks []string

	b.cfg.Logger.Infof("Setting bootloader with current passive snapshots")

	active, err := b.getActiveSnapshot()
	if err != nil {
		b.cfg.Logger.Warnf("failed getting current active snapshot: %v", err)
		return err
	}

	ids, err := b.getPassiveSnapshots()
	if err != nil {
		b.cfg.Logger.Warnf("failed getting current passive snapshots: %v", err)
		return err
	}

	for i := len(ids) - 1; i >= 0; i-- {
		passives = append(passives, strconv.Itoa(ids[i]))
	}

	// We count first is active, then all passives and finally the recovery
	for i := 0; i <= len(ids)+1; i++ {
		fallbacks = append(fallbacks, strconv.Itoa(i))
	}
	snapsList := strings.Join(passives, " ")
	fallbackList := strings.Join(fallbacks, " ")
	envFile := filepath.Join(b.efiDir, constants.GrubOEMEnv)

	envs := map[string]string{
		constants.GrubFallback:         fallbackList,
		constants.GrubPassiveSnapshots: snapsList,
		constants.GrubActiveSnapshot:   strconv.Itoa(active),
		"snapshotter":                  constants.BtrfsSnapshotterType,
	}

	err = b.bootloader.SetPersistentVariables(envFile, envs)
	if err != nil {
		b.cfg.Logger.Warnf("failed setting bootloader environment file %s: %v", envFile, err)
		return err
	}

	return err
}

func (b *Btrfs) configureSnapper(snapshot *types.Snapshot) error {
	defaultTmpl, err := utils.FindFile(b.cfg.Fs, snapshot.WorkDir, configTemplatesPaths()...)
	if err != nil {
		b.cfg.Logger.Errorf("failed to find default snapper configuration template")
		return err
	}

	sysconfigData := map[string]string{}
	sysconfig := filepath.Join(snapshot.WorkDir, snapperSysconfig)
	if ok, _ := utils.Exists(b.cfg.Fs, sysconfig); ok {
		sysconfigData, err = utils.LoadEnvFile(b.cfg.Fs, sysconfig)
		if err != nil {
			b.cfg.Logger.Errorf("failed to load global snapper sysconfig")
			return err
		}
	}
	sysconfigData["SNAPPER_CONFIGS"] = "root"

	b.cfg.Logger.Debugf("Creating sysconfig snapper configuration at '%s'", sysconfig)
	err = utils.WriteEnvFile(b.cfg.Fs, sysconfigData, sysconfig)
	if err != nil {
		b.cfg.Logger.Errorf("failed writing snapper global configuration file: %v", err)
		return err
	}

	snapCfg, err := utils.LoadEnvFile(b.cfg.Fs, defaultTmpl)
	if err != nil {
		b.cfg.Logger.Errorf("failed to load default snapper templage configuration")
		return err
	}

	snapCfg["TIMELINE_CREATE"] = "no"
	snapCfg["QGROUP"] = "1/0"
	snapCfg["NUMBER_LIMIT"] = strconv.Itoa(b.snapshotterCfg.MaxSnaps)
	snapCfg["NUMBER_LIMIT_IMPORTANT"] = strconv.Itoa(b.snapshotterCfg.MaxSnaps)

	rootCfg := filepath.Join(snapshot.WorkDir, snapperRootConfig)
	b.cfg.Logger.Debugf("Creating 'root' snapper configuration at '%s'", rootCfg)
	err = utils.WriteEnvFile(b.cfg.Fs, snapCfg, rootCfg)
	if err != nil {
		b.cfg.Logger.Errorf("failed writing snapper root configuration file: %v", err)
		return err
	}
	return nil
}

func (b *Btrfs) remountStatePartition(state *types.Partition) error {
	b.cfg.Logger.Debugf("Umount %s", state.MountPoint)
	err := b.cfg.Mounter.Unmount(state.MountPoint)
	if err != nil {
		b.cfg.Logger.Errorf("failed unmounting %s: %v", state.MountPoint, err)
		return err
	}

	b.cfg.Logger.Debugf("Remount root '%s' on top level subvolume '%s'", state.MountPoint, rootSubvol)
	err = b.cfg.Mounter.Mount(state.Path, state.MountPoint, "btrfs", []string{"rw", fmt.Sprintf("subvol=%s", rootSubvol)})
	if err != nil {
		b.cfg.Logger.Errorf("failed mounting subvolume %s at %s", rootSubvol, state.MountPoint)
		return err
	}

	if b.activeSnapshotID > 0 {
		// XXX dynamically find active snapshot here
		err = b.mountSnapshotsSubvolumeInSnapshot(state.Path, state.MountPoint, b.activeSnapshotID)
		b.snapperArgs = []string{"--no-dbus", "--root", filepath.Join(state.MountPoint, fmt.Sprintf(snapshotPathTmpl, b.activeSnapshotID))}
	}
	b.rootDir = state.MountPoint
	return err
}

func (b *Btrfs) mountSnapshotsSubvolumeInSnapshot(device, root string, snapshotID int) error {
	var mountpoint, subvol string

	b.snapshotsMount = func() error {
		b.cfg.Logger.Debugf("Mount snapshots subvolume in active snapshot %d", snapshotID)
		mountpoint = filepath.Join(filepath.Join(root, fmt.Sprintf(snapshotPathTmpl, snapshotID)), snapshotsPath)
		subvol = fmt.Sprintf("subvol=%s", filepath.Join(rootSubvol, snapshotsPath))
		return b.cfg.Mounter.Mount(device, mountpoint, "btrfs", []string{"rw", subvol})
	}
	err := b.snapshotsMount()
	if err != nil {
		b.cfg.Logger.Errorf("failed mounting subvolume %s at %s", subvol, mountpoint)
		return err
	}
	b.snapshotsUmount = func() error { return b.cfg.Mounter.Unmount(mountpoint) }
	return nil
}

func (b *Btrfs) setBtrfsForFirstTime(state *types.Partition) error {
	topDir, _, _, _, err := findStateMount(b.cfg.Runner, state.Path)
	if err != nil {
		b.cfg.Logger.Errorf("could not find expected mountpoints")
		return err
	}
	if topDir == "" {
		b.cfg.Logger.Errorf("btrfs root is not mounted, can't initialize the snapshotter within an existing subvolume")
		return err
	}

	b.cfg.Logger.Debug("Enabling btrfs quota")
	cmdOut, err := b.cfg.Runner.Run("btrfs", "quota", "enable", topDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed setting quota for btrfs partition at %s: %s", topDir, string(cmdOut))
		return err
	}

	b.cfg.Logger.Debug("Creating essential subvolumes")
	for _, subvolume := range []string{filepath.Join(topDir, rootSubvol), filepath.Join(topDir, rootSubvol, snapshotsPath)} {
		b.cfg.Logger.Debugf("Creating subvolume: %s", subvolume)
		cmdOut, err = b.cfg.Runner.Run("btrfs", "subvolume", "create", subvolume)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating subvolume %s: %s", subvolume, string(cmdOut))
			return err
		}
	}

	b.cfg.Logger.Debug("Create btrfs quota group")
	cmdOut, err = b.cfg.Runner.Run("btrfs", "qgroup", "create", "1/0", topDir)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating quota group for %s: %s", topDir, string(cmdOut))
		return err
	}
	return nil
}

func (b *Btrfs) configureSnapperAndRootDir(state *types.Partition) error {
	_, rootDir, _, stateDir, err := findStateMount(b.cfg.Runner, state.Path)

	if stateDir == "" || rootDir == "" {
		err = fmt.Errorf("could not find expected mountpoints")
		return err
	}

	if err != nil {
		b.cfg.Logger.Errorf("failed setting snapper root and state partition mountpoint: %v", err)
		return err
	}

	// state.MountPoint must be updated otherwise state.yaml will fail to update
	state.MountPoint = stateDir
	b.rootDir = rootDir

	if b.rootDir != "/" {
		b.snapperArgs = []string{"--no-dbus", "--root", b.rootDir}
	}
	return nil
}

// General purpose function to retrieve all btrfs mount points for a given state partition
// incoming path can be either a disk device or the path of a mounted btrfs filesystem
// goal of this function is to be able to resolve path to all relevant btrfs directories
func findStateMount(runner types.Runner, path string) (topDir string, rootDir string, snapshotDir string, stateDir string, err error) {
	output, err := runner.Run("findmnt", "-lno", "SOURCE,TARGET,FSTYPE", path)
	if err != nil {
		return "", "", "", "", err
	}

	// first pass accumulate findmnt lines.
	// This allow to ensure there is only one result when using a mounted btrfs filesystem path as argument
	var lines [][]string
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(output))))
	for scanner.Scan() {
		lineFields := strings.Fields(scanner.Text())
		if len(lineFields) != 3 {
			continue
		}
		// Only handle lines with "btrfs" type
		if lineFields[2] == "btrfs" {
			lines = append(lines, lineFields)
		}
	}

	r := regexp.MustCompile(`^@/\.snapshots/\d+/snapshot$`)
	snapshotsSubvol := filepath.Join(rootSubvol, snapshotsPath)

	// second pass over parsed findmnt lines. search each mounted subvolume of interest.
	var rootDirMatches []string
	for _, lineFields := range lines {
		subStart := strings.Index(lineFields[0], "[/")

		// Additional feature: recursive logic if array length is 1 and device matches target
		if len(lines) == 1 && path == lineFields[1] {
			// Handle subStart logic for recursive call
			if subStart != -1 {
				return findStateMount(runner, lineFields[0][0:subStart])
			}
			return findStateMount(runner, lineFields[0])
		}

		subEnd := strings.LastIndex(lineFields[0], "]")

		// Check if no subvolume is present
		if subStart == -1 && subEnd == -1 {
			topDir = lineFields[1] // this is the btrfs root
		} else {
			subVolume := lineFields[0][subStart+2 : subEnd]

			if subVolume == rootSubvol {
				stateDir = lineFields[1]
			} else if subVolume == snapshotsSubvol {
				snapshotDir = lineFields[1]
			} else if r.MatchString(subVolume) {
				rootDirMatches = append(rootDirMatches, lineFields[1]) // accumulate rootDir matches
			}
		}
	}

	// assume that is there is only one match for a snapshot, this is the rootDir
	if len(rootDirMatches) == 1 {
		rootDir = rootDirMatches[0]
	}

	// If stateDir isn't found but topDir exists, append the rootSubvol to topDir
	if stateDir == "" && topDir != "" {
		stateDir = filepath.Join(topDir, rootSubvol)
	}

	// If snapshotDir isn't found but stateDir exists, append the subvolume to stateDir
	if snapshotDir == "" && stateDir != "" {
		snapshotDir = filepath.Join(stateDir, snapshotsPath)
	}

	return topDir, rootDir, snapshotDir, stateDir, err
}
