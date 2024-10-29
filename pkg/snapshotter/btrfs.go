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
	rootSubvolID      = 257
	snapshotsSubvolID = 258
	snapshotsPath     = ".snapshots"
	snapshotPathTmpl  = ".snapshots/%d/snapshot"
	snapshotPathRegex = `.snapshots/(\d+)/snapshot`
	snapshotInfoPath  = ".snapshots/%d/info.xml"
	snapshotWorkDir   = "snapshot.workDir"
	dateFormat        = "2006-01-02 15:04:05"
	snapperRootConfig = "/etc/snapper/configs/root"
	snapperSysconfig  = "/etc/sysconfig/snapper"
	installProgress   = "install-in-progress"
	updateProgress    = "update-in-progress"
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
	var snapshot *types.Snapshot

	b.cfg.Logger.Info("Starting a btrfs snapshotter transaction")

	if !b.installing && b.activeSnapshotID == 0 {
		b.cfg.Logger.Errorf("Snapshotter should have been initalized before starting a transaction")
		return nil, fmt.Errorf("uninitialized snapshotter")
	}

	if !b.installing {
		snapshot, err = b.createSnapperSnapshot()
		if err != nil {
			return nil, err
		}
	} else {
		snapshot, err = b.createFirstBtrfsSnapshot()
		if err != nil {
			return nil, err
		}
	}

	err = utils.MkdirAll(b.cfg.Fs, constants.WorkingImgDir, constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating working tree directory: %s", constants.WorkingImgDir)
		return nil, err
	}

	err = b.cfg.Mounter.Mount(snapshot.WorkDir, constants.WorkingImgDir, "bind", []string{"bind"})
	if err != nil {
		_ = b.DeleteSnapshot(newID)
		return nil, err
	}
	snapshot.MountPoint = constants.WorkingImgDir
	snapshot.InProgress = true

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
	}

	extraBind := map[string]string{filepath.Join(b.rootDir, snapshotsPath): filepath.Join("/", snapshotsPath)}
	err = elemental.ApplySELinuxLabels(b.cfg, snapshot.Path, extraBind)
	if err != nil {
		b.cfg.Logger.Errorf("failed relabelling snapshot path: %s", snapshot.Path)
		return err
	}

	if !b.installing {
		err = b.closeSnapperSnapshot(snapshot)
		if err != nil {
			return err
		}
	} else {
		err = b.closeBtrfsSnapshot(snapshot)
		if err != nil {
			return err
		}
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
		snapshots, err = b.loadSnapperSnapshots()
		if err != nil {
			return nil, err
		}
		return snapshots, err
	} else if err != nil {
		return nil, err
	}
	return []int{}, err
}

func (b *Btrfs) createSnapperSnapshot() (*types.Snapshot, error) {
	b.cfg.Logger.Infof("Creating a new snapshot from %d", b.activeSnapshotID)
	args := []string{
		"create", "--from", strconv.Itoa(b.activeSnapshotID),
		"--read-write", "--print-number", "--description",
		fmt.Sprintf("Update for snapshot %d", b.activeSnapshotID),
		"-c", "number", "--userdata", fmt.Sprintf("%s=yes", updateProgress),
	}
	args = append(b.snapperArgs, args...)
	cmdOut, err := b.cfg.Runner.Run("snapper", args...)
	if err != nil {
		b.cfg.Logger.Errorf("snapper failed to create a new snapshot: %v", err)
		return nil, err
	}
	newID, err := strconv.Atoi(strings.TrimSpace(string(cmdOut)))
	if err != nil {
		b.cfg.Logger.Errorf("failed parsing new snapshot ID")
		return nil, err
	}

	workingDir := filepath.Join(b.rootDir, snapshotsPath, strconv.Itoa(newID), snapshotWorkDir)
	err = utils.MkdirAll(b.cfg.Fs, workingDir, constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating the snapshot working directory: %v", err)
		_ = b.DeleteSnapshot(newID)
		return nil, err
	}
	path := filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, newID))
	return &types.Snapshot{
		ID:      newID,
		WorkDir: workingDir,
		Path:    path,
	}, nil
}

func (b *Btrfs) createFirstBtrfsSnapshot() (*types.Snapshot, error) {
	b.cfg.Logger.Info("Creating first root filesystem as a snapshot")
	newID := 1
	err := utils.MkdirAll(b.cfg.Fs, filepath.Join(b.rootDir, snapshotsPath, strconv.Itoa(newID)), constants.DirPerm)
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
	err = b.writeSnapperSnapshotXML(snapperXML, newSnapperSnapshotXML(newID, "first root filesystem"))
	if err != nil {
		b.cfg.Logger.Errorf("failed creating snapper info XML")
		return nil, err
	}
	workingDir := filepath.Join(b.rootDir, fmt.Sprintf(snapshotPathTmpl, newID))
	return &types.Snapshot{
		ID:      newID,
		WorkDir: workingDir,
		Path:    workingDir,
	}, nil
}

func (b *Btrfs) closeSnapperSnapshot(snapshot *types.Snapshot) error {
	args := []string{
		"modify", "--read-only", "--default", "--userdata",
		fmt.Sprintf("%s=,%s=", installProgress, updateProgress), strconv.Itoa(snapshot.ID),
	}
	args = append(b.snapperArgs, args...)
	cmdOut, err := b.cfg.Runner.Run("snapper", args...)
	if err != nil {
		b.cfg.Logger.Errorf("failed clearing userdata for snapshot %d: %s", snapshot.ID, string(cmdOut))
		return err
	}
	return nil
}

func (b *Btrfs) closeBtrfsSnapshot(snapshot *types.Snapshot) error {
	snapperXML := filepath.Join(b.rootDir, fmt.Sprintf(snapshotInfoPath, snapshot.ID))

	snapshotData, err := b.loadSnapperSnapshotXML(snapperXML)
	if err != nil {
		b.cfg.Logger.Errorf("failed reading snapshot %d metadata: %v", snapshot.ID, err)
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
		b.cfg.Logger.Errorf("failed writing snapshot %d metadata: %v", snapshot.ID, err)
		return err
	}

	cmdOut, err := b.cfg.Runner.Run("btrfs", "property", "set", snapshot.Path, "ro", "true")
	if err != nil {
		b.cfg.Logger.Errorf("failed setting read only property to snapshot %d: %s", snapshot.ID, string(cmdOut))
		return err
	}

	subvolID, err := b.findSubvolumeByPath(fmt.Sprintf(snapshotPathTmpl, snapshot.ID))
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

func (b *Btrfs) loadSnapperSnapshots() ([]int, error) {
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

func (b *Btrfs) isInitiated(rootDir string) (bool, error) {
	var rootVolume, snapshotsVolume bool

	if b.activeSnapshotID > 0 {
		return true, nil
	}
	if b.installing {
		return false, nil
	}

	volumes, err := b.getSubvolumes(rootDir)
	if err != nil {
		return false, err
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

func newSnapperSnapshotXML(id int, desc string) SnapperSnapshotXML {
	var usrData UserData
	if id == 1 {
		usrData = UserData{Key: "install-in-progress", Value: "yes"}
	} else {
		usrData = UserData{Key: "install-in-progress", Value: "yes"}
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

func (b *Btrfs) loadSnapperSnapshotXML(filepath string) (SnapperSnapshotXML, error) {
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

	snapshots, err := b.GetSnapshots()
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if snapshot != b.activeSnapshotID {
			passives = append(passives, snapshot)
		}
	}

	return passives, nil
}

// setBootloader sets the bootloader variables to update new passives
func (b *Btrfs) setBootloader() error {
	var passives, fallbacks []string

	b.cfg.Logger.Infof("Setting bootloader with current passive snapshots")
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
	rootDir, stateMount, err := findStateMount(b.cfg.Runner, state.Path)
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

func findStateMount(runner types.Runner, device string) (rootDir string, stateMount string, err error) {
	output, err := runner.Run("findmnt", "-lno", "SOURCE,TARGET", device)
	if err != nil {
		return "", "", err
	}
	r := regexp.MustCompile(`@/.snapshots/\d+/snapshot`)

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(output))))
	for scanner.Scan() {
		lineFields := strings.Fields(scanner.Text())
		if len(lineFields) != 2 {
			continue
		}
		if strings.Contains(lineFields[1], constants.RunningStateDir) {
			stateMount = lineFields[1]
		} else if r.MatchString(lineFields[0]) {
			rootDir = lineFields[1]
		}
	}

	if stateMount == "" || rootDir == "" {
		err = fmt.Errorf("could not find expected mountpoints, findmnt output: %s", string(output))
	}

	return rootDir, stateMount, err
}
