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
	cfg               types.Config
	snapshotterCfg    types.SnapshotterConfig
	btrfsCfg          types.BtrfsConfig
	device            string
	rootDir           string
	efiDir            string
	currentSnapshotID int
	activeSnapshotID  int
	bootloader        types.Bootloader
	installing        bool
	snapperArgs       []string
	snapshotsUmount   func() error
	snapshotsMount    func() error
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
	b.device = state.Path
	b.rootDir = state.MountPoint
	b.efiDir = efiDir

	b.cfg.Logger.Debug("Checking if essential subvolumes are already created")
	if ok, err = b.isInitiated(state.MountPoint); ok {
		if elemental.IsActiveMode(b.cfg) || elemental.IsPassiveMode(b.cfg) {
			return b.configureSnapperAndRootDir(state)
		}
		b.cfg.Logger.Debug("Remount state partition at root subvolume")
		return b.remountStatePartition(state)
	} else if err != nil {
		b.cfg.Logger.Errorf("failed loading initial snapshotter state: %s", err.Error())
		return err
	}

	b.installing = true
	b.cfg.Logger.Debug("Running initial btrfs configuration")
	return b.setBtrfsForFirstTime(state)
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

	err = utils.MkdirAll(b.cfg.Fs, constants.WorkingImgDir, constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating working tree directory: %s", constants.WorkingImgDir)
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

	// locate mounted state directory
	stateDir, err := findStatePath(b.cfg.Runner, b.device)
	if err != nil {
		b.cfg.Logger.Errorf("unable to locate state directory: %s", err.Error())
		return err
	}

	// Remove old symlink and create a new one
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

	_ = b.setBootloader()
	if !b.installing {
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

	args := []string{"delete", "--sync", strconv.Itoa(id)}
	args = append(b.snapperArgs, args...)
	cmdOut, err = b.cfg.Runner.Run("snapper", args...)
	if err != nil {
		b.cfg.Logger.Errorf("snapper failed deleting snapshot %d: %s", id, string(cmdOut))
		return err
	}

	return nil
}

func (b *Btrfs) GetSnapshots() (snapshots []int, err error) {
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
			if match[3] == "yes" {
				b.currentSnapshotID = id
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
	stateDir, err := findStatePath(b.cfg.Runner, b.device)
	if err != nil {
		b.cfg.Logger.Errorf("unable to locate state directory: %s", err.Error())
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
		err = b.mountSnapshotsSubvolumeInSnapshot(state.MountPoint, state.Path, b.activeSnapshotID)
		b.snapperArgs = []string{"--no-dbus", "--root", filepath.Join(state.MountPoint, fmt.Sprintf(snapshotPathTmpl, b.activeSnapshotID))}
	}
	b.rootDir = state.MountPoint
	return err
}

func (b *Btrfs) mountSnapshotsSubvolumeInSnapshot(root, device string, snapshotID int) error {
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
	b.cfg.Logger.Debug("Enabling btrfs quota")
	cmdOut, err := b.cfg.Runner.Run("btrfs", "quota", "enable", state.MountPoint)
	if err != nil {
		b.cfg.Logger.Errorf("failed setting quota for btrfs partition at %s: %s", state.MountPoint, string(cmdOut))
		return err
	}

	b.cfg.Logger.Debug("Creating essential subvolumes")
	for _, subvolume := range []string{filepath.Join(state.MountPoint, rootSubvol), filepath.Join(state.MountPoint, rootSubvol, snapshotsPath)} {
		b.cfg.Logger.Debugf("Creating subvolume: %s", subvolume)
		cmdOut, err = b.cfg.Runner.Run("btrfs", "subvolume", "create", subvolume)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating subvolume %s: %s", subvolume, string(cmdOut))
			return err
		}
	}

	b.cfg.Logger.Debug("Create btrfs quota group")
	cmdOut, err = b.cfg.Runner.Run("btrfs", "qgroup", "create", "1/0", state.MountPoint)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating quota group for %s: %s", state.MountPoint, string(cmdOut))
		return err
	}
	return b.remountStatePartition(state)
}

func (b *Btrfs) configureSnapperAndRootDir(state *types.Partition) error {
	rootDir, stateMount, err := findSnapperStateAndRootMount(b.cfg.Runner, b.device)
	if err != nil {
		b.cfg.Logger.Errorf("failed setting snapper root and state partition mountpoint: %v", err)
		return err
	}

	state.MountPoint = stateMount
	b.rootDir = rootDir

	if b.rootDir != "/" {
		b.snapperArgs = []string{"--no-dbus", "--root", b.rootDir}
	}
	return nil
}

func findSnapperStateAndRootMount(runner types.Runner, device string) (rootDir string, stateMount string, err error) {
	output, err := runner.Run("findmnt", "-lno", "SOURCE,TARGET,OPTIONS", device)
	if err != nil {
		return "", "", err
	}
	rsnap := regexp.MustCompile(`@/.snapshots/\d+/snapshot`)
	rvol := regexp.MustCompile(`subvol=/([^,]*)`)

	var rootMount string

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(output))))
	for scanner.Scan() {
		lineFields := strings.Fields(scanner.Text())
		if len(lineFields) != 3 {
			continue
		}

		if rsnap.MatchString(lineFields[0]) {
			rootDir = lineFields[1]
		} else {
			volumeMatch := rvol.FindStringSubmatch(lineFields[2])

			if len(volumeMatch) > 1 {
				subvol := volumeMatch[1]

				if subvol == "" {
					rootMount = lineFields[1]
				} else if subvol == rootSubvol {
					stateMount = lineFields[1]
				}
			}
		}
	}

	if stateMount == "" && rootMount != "" {
		stateMount = filepath.Join(rootMount, rootSubvol)
	}

	if stateMount == "" || rootDir == "" {
		err = fmt.Errorf("could not find expected mountpoints, findmnt output: %s", string(output))
	}

	return rootDir, stateMount, err
}

func findStatePath(runner types.Runner, device string) (statePath string, err error) {
	output, err := runner.Run("findmnt", "-lno", "SOURCE,TARGET,OPTIONS", device)
	if err != nil {
		return "", err
	}

	rvol := regexp.MustCompile(`subvol=/([^,]*)`)

	var rootMount string

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(output))))
	for scanner.Scan() {
		lineFields := strings.Fields(scanner.Text())
		if len(lineFields) != 3 {
			continue
		}

		volumeMatch := rvol.FindStringSubmatch(lineFields[2])

		if len(volumeMatch) > 1 {
			subvol := volumeMatch[1]

			if subvol == "" {
				rootMount = lineFields[1]
			} else if subvol == rootSubvol {
				statePath = lineFields[1]
			}
		}
	}

	if statePath == "" && rootMount != "" {
		statePath = filepath.Join(rootMount, rootSubvol)
	}

	if statePath == "" {
		err = fmt.Errorf("could not find state mountpoint, findmnt output: %s", string(output))
	}

	return statePath, err
}
