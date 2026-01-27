package plugins

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

type Disk struct {
	Device  string
	SectorS uint
	LastS   uint
	Parts   []Partition
}

// We only manage sizes in sectors unit for the Partition struct and sgdisk wrapper
type Partition struct {
	Number     int
	StartS     uint
	SizeS      uint
	PLabel     string
	FileSystem string
	FSLabel    string
	Type       string
}

type GdiskCall struct {
	dev       string
	wipe      bool
	parts     []*Partition
	deletions []int
	expand    bool
	pretend   bool
}

type MkfsCall struct {
	part       Partition
	customOpts []string
	dev        string
}

const (
	partitionTries = 10
)

func Layout(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.Layout.Device == nil {
		l.Debug("did not find any device in layout")
		return nil
	}

	var dev Disk
	var err error
	for _, l := range s.Layout.Parts {
		if l.FileSystem == "xfs" && len(l.FSLabel) > 12 {
			return errors.New(fmt.Sprintf("xfs filesystem %s cannot have a label longer than 12 chars", l.FSLabel))
		}
	}
	if len(strings.TrimSpace(s.Layout.Device.Label)) > 0 {
		l.Debugf("Using label %s for layout expansion", s.Layout.Device.Label)
		dev, err = FindDiskFromPartitionLabel(l, s.Layout.Device.Label, console)
		if err != nil {
			l.Warnf("Exiting, disk with label %s not found: %s", s.Layout.Device.Label, err.Error())
			return nil
		}
	} else if len(strings.TrimSpace(s.Layout.Device.Path)) > 0 {
		l.Debugf("Using path %s for layout expansion", s.Layout.Device.Path)
		dev, err = FindDiskFromPath(s.Layout.Device.Path, console)
		if err != nil {
			l.Warnf("Exiting, disk with path %s not found: %s", s.Layout.Device.Path, err.Error())
			return nil
		}
	} else {
		return nil
	}

	changed := false

	// Check there is a minimum of 32MiB of free space in disk
	if !dev.CheckDiskFreeSpaceMiB(l, 32, console) {
		l.Warnf("Not enough unpartitioned space in disk to operate")
		return nil
	}

	for _, part := range s.Layout.Parts {
		if match := MatchPartitionFSLabel(l, part.FSLabel, console); match != "" {
			l.Warnf("Partition with FSLabel: %s already exists, ignoring", part.FSLabel)
			continue
		} else if match := MatchPartitionPLabel(l, part.PLabel, console); match != "" {
			l.Warnf("Partition with PLabel: %s already exists, ignoring", part.PLabel)
			continue
		}
		// Set default filesystem
		if part.FileSystem == "" {
			part.FileSystem = "ext2"
		}

		l.Infof("Creating %s partition", part.FSLabel)
		out, err := dev.AddPartition(l, part.FSLabel, part.Size, part.FileSystem, part.PLabel, console)
		if err != nil {
			l.Error(out)
			return err
		}
		changed = true
	}

	if s.Layout.Expand != nil {
		if s.Layout.Expand.Size == 0 {
			l.Info("Extending last partition to max space")
		} else {
			l.Infof("Extending last partition up to %d MiB", s.Layout.Expand.Size)
		}
		out, err := dev.ExpandLastPartition(l, s.Layout.Expand.Size, console)
		if err != nil {
			l.Error(out)
			return err
		}
		changed = true
	}

	if changed {
		dev.ReloadPartitionTable(l, console)
	}
	return nil
}

func MatchPartitionFSLabel(l logger.Interface, label string, console Console) string {
	if label != "" {
		out, _ := console.Run("udevadm settle")
		l.Debugf("Output of udevadm settle: %s", out)
		out, err := console.Run(fmt.Sprintf("blkid -l --match-token LABEL=%s -o device", label))
		if err == nil {
			return out
		} else {
			l.Debugf("failed to get device for label %s: %s", label, err.Error())
		}

	}
	return ""
}

func MatchPartitionPLabel(l logger.Interface, label string, console Console) string {
	if label != "" {
		_, _ = console.Run("udevadm settle")
		out, err := console.Run(fmt.Sprintf("blkid -l --match-token PARTLABEL=%s -o device", label))
		if err == nil {
			return strings.TrimSpace(out)
		} else {
			l.Debugf("failed to get device for partition label %s: %s", label, err.Error())
		}
	}
	return ""
}

func FindDiskFromPath(path string, console Console) (Disk, error) {
	out, err := console.Run(fmt.Sprintf("lsblk -npo type %s", path))
	if err != nil {
		return Disk{}, errors.New(fmt.Sprintf("Output: %s Error: %s", out, err.Error()))
	}
	if strings.HasPrefix(strings.TrimSpace(out), "disk") {
		return Disk{Device: path}, nil
	} else if strings.HasPrefix(strings.TrimSpace(out), "loop") {
		return Disk{Device: path}, nil
	} else if strings.HasPrefix(strings.TrimSpace(out), "part") {
		device, err := console.Run(fmt.Sprintf("lsblk -npo pkname %s", path))
		device = strings.TrimSpace(device)
		if err == nil {
			return Disk{Device: device}, nil
		}
	}

	return Disk{}, errors.New(fmt.Sprintf("Could not verify %s is a block device", path))
}

func FindDiskFromPartitionLabel(l logger.Interface, label string, console Console) (Disk, error) {
	if partnode := MatchPartitionFSLabel(l, label, console); partnode != "" {
		device, err := console.Run(fmt.Sprintf("lsblk -npo pkname %s", partnode))
		if err == nil {
			device = strings.TrimSpace(device)
			l.Debugf("Got device %s for label %s", device, label)
			return Disk{Device: device}, nil
		} else {
			l.Debugf("Error getting partition fs label: %s", err.Error())
		}
	} else if partnode := MatchPartitionPLabel(l, label, console); partnode != "" {
		device, err := console.Run(fmt.Sprintf("lsblk -npo pkname %s", partnode))
		device = strings.TrimSpace(device)
		if err == nil {
			return Disk{Device: device}, nil
		}
	}
	return Disk{}, errors.New("Could not find device for the given label")
}

func (dev Disk) String() string {
	return dev.Device
}

func (dev *Disk) Reload(console Console) error {
	gd := NewGdiskCall(dev.String())
	prnt, err := gd.Print(console)
	if err != nil {
		return err
	}

	sectorS, err := gd.GetSectorSize(prnt)
	if err != nil {
		return err
	}
	lastS, err := gd.GetLastSector(prnt)
	if err != nil {
		return err
	}
	partitions := gd.GetPartitions(prnt)
	dev.SectorS = sectorS
	dev.LastS = lastS
	dev.Parts = partitions
	return nil
}

// Size is expressed in MiB here
func (dev *Disk) CheckDiskFreeSpaceMiB(l logger.Interface, minSpace uint, console Console) bool {
	freeS, err := dev.GetFreeSpace(l, console)
	if err != nil {
		l.Warnf("Could not calculate disk free space: %s", err.Error())
		return false
	}
	minSec := MiBToSectors(minSpace, dev.SectorS)
	if freeS < minSec {
		return false
	}
	return true
}

func (dev *Disk) GetFreeSpace(l logger.Interface, console Console) (uint, error) {
	gd := NewGdiskCall(dev.String())
	if gd.HasUnallocatedSpace(console) {
		gd.ExpandPTable()
		out, err := gd.WriteChanges(console)
		if err != nil {
			l.Errorf("Failed resizing the partition table: \n%s", out)
			return 0, err
		}
		err = dev.Reload(console)
		if err != nil {
			return 0, err
		}
	}

	//Check we have loaded partition table data
	if dev.SectorS == 0 {
		err := dev.Reload(console)
		if err != nil {
			l.Errorf("Failed analyzing disk: %v\n", err)
			return 0, err
		}
	}

	return dev.computeFreeSpace(), nil
}

func (dev Disk) computeFreeSpace() uint {
	if len(dev.Parts) > 0 {
		lastPart := dev.Parts[len(dev.Parts)-1]
		return dev.LastS - (lastPart.StartS + lastPart.SizeS - 1)
	} else {
		// Assume first partitions is alined to 1MiB
		return dev.LastS - (1024*1024/dev.SectorS - 1)
	}
}

func (dev Disk) computeFreeSpaceWithoutLast() uint {
	if len(dev.Parts) > 1 {
		part := dev.Parts[len(dev.Parts)-2]
		return dev.LastS - (part.StartS + part.SizeS - 1)
	} else {
		// Assume first partitions is alined to 1MiB
		return dev.LastS - (1024*1024/dev.SectorS - 1)
	}
}

// Size is expressed in MiB here
func (dev *Disk) AddPartition(l logger.Interface, label string, size uint, fileSystem string, pLabel string, console Console) (string, error) {
	gd := NewGdiskCall(dev.String())
	pType := "8300"
	if fatFS, _ := regexp.MatchString("fat|vfat", fileSystem); fatFS {
		// We are assuming Fat is only used for EFI partitions
		pType = "EF00"
	}

	//Check we have loaded partition table data
	if dev.SectorS == 0 {
		err := dev.Reload(console)
		if err != nil {
			l.Errorf("Failed analyzing disk: %v\n", err)
			return "", err
		}
	}

	var partNum int
	var startS uint
	if len(dev.Parts) > 0 {
		partNum = dev.Parts[len(dev.Parts)-1].Number
		startS = 0
	} else {
		//First partition is aligned at 1MiB
		startS = 1024 * 1024 / dev.SectorS
	}

	size = MiBToSectors(size, dev.SectorS)
	freeS := dev.computeFreeSpace()
	if size > freeS {
		return "", errors.New(fmt.Sprintf("Not enough free space in disk. Required: %d sectors; Available %d sectors", size, freeS))
	}

	partNum++
	var part = Partition{
		Number:     partNum,
		StartS:     startS,
		SizeS:      size,
		PLabel:     pLabel,
		FileSystem: fileSystem,
		FSLabel:    label,
		Type:       pType,
	}

	gd.CreatePartition(&part)

	out, err := gd.WriteChanges(console)
	if err != nil {
		return out, err
	}
	err = dev.Reload(console)
	if err != nil {
		l.Errorf("Failed analyzing disk: %v\n", err)
		return "", err
	}

	pDev, err := dev.FindPartitionDevice(l, part.Number, console)
	if err != nil {
		return "", err
	}

	if fileSystem != "-" {
		mkfs := MkfsCall{part: part, customOpts: []string{}, dev: pDev}
		return mkfs.Apply(console)
	}

	return out, nil
}

func (dev Disk) ReloadPartitionTable(l logger.Interface, console Console) error {
	for tries := 0; tries <= partitionTries; tries++ {
		l.Debugf("Trying to reread the partition table of %s (try number %d)", dev, tries+1)
		_, _ = console.Run("udevadm settle")

		out, err1 := console.Run(fmt.Sprintf("partprobe %s", dev))
		l.Debugf("output of partprobe: %s", out)
		if err1 != nil && tries == (partitionTries-1) {
			l.Debugf("Error of partprobe: %s", err1)
			return errors.New(fmt.Sprintf("Could not reload partition table: %s", out))
		}

		out, err2 := console.Run("sync")
		l.Debugf("Output of sync: %s", out)
		if err2 != nil && tries == (partitionTries-1) {
			l.Debugf("Error of sync: %s", err2)
			return errors.New(fmt.Sprintf("Could not sync: %s", out))
		}

		// If nothing failed exit
		if err1 == nil && err2 == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func (dev Disk) FindPartitionDevice(l logger.Interface, partNum int, console Console) (string, error) {
	var match string
	for tries := 0; tries <= partitionTries; tries++ {
		err := dev.ReloadPartitionTable(l, console)
		if err != nil {
			l.Errorf("Failed on reloading the partition table: %v\n", err)
			return "", err
		}
		l.Debugf("Trying to find the partition device %d of device %s (try number %d)", partNum, dev, tries+1)
		out, _ := console.Run("udevadm settle")
		l.Debugf("Output of udevadm settle: %s", out)
		if err != nil && tries == (partitionTries-1) {
			l.Debugf("Error of udevadm settle: %s", err)
			return "", errors.New(fmt.Sprintf("Could not list settle: %s", out))
		}
		out, err = console.Run(fmt.Sprintf("lsblk -ltnpo name,type %s", dev))
		l.Debugf("Output of lsblk: %s", out)
		if err != nil && tries == (partitionTries-1) {
			l.Debugf("Error of lsblk: %s", err)
			return "", errors.New(fmt.Sprintf("Could not list device partition nodes: %s", out))
		}

		re, err := regexp.Compile(fmt.Sprintf("(?m)^(/.*%d) part$", partNum))
		if err != nil && tries == 4 {
			return "", errors.New("Failed compiling regexp")
		}
		matched := re.FindStringSubmatch(out)
		if matched == nil && tries == (partitionTries-1) {
			return "", errors.New(fmt.Sprintf("Could not find partition device path for partition %d", partNum))
		}
		if matched != nil {
			match = matched[1]
			break
		}
		time.Sleep(1 * time.Second)
	}
	return match, nil
}

// ExpandLastPartition will call growpart + resize tool in a given partition to grow it up to max space available
func (dev *Disk) ExpandLastPartition(l logger.Interface, size uint, console Console) (string, error) {
	if len(dev.Parts) == 0 {
		return "", errors.New("There is no partition to expand")
	}

	//Check we have loaded partition table data
	if dev.SectorS == 0 {
		err := dev.Reload(console)
		if err != nil {
			l.Errorf("Failed analyzing disk: %v\n", err)
			return "", err
		}
	}

	if size > 0 {
		size = MiBToSectors(size, dev.SectorS)
		part := dev.Parts[len(dev.Parts)-1]
		if size < part.SizeS {
			return "", errors.New("Layout plugin can only expand a partition, not shrink it")
		}
		freeS := dev.computeFreeSpaceWithoutLast()
		if size > freeS {
			return "", fmt.Errorf("Not enough free space for to expand last partition up to %d sectors", size)
		}
	}

	part := dev.Parts[len(dev.Parts)-1]

	// Grow partition
	out, err := console.Run(fmt.Sprintf("growpart %s %d", dev.Device, part.Number))
	if err != nil {
		return out, err
	}

	// Expand FS
	fullDevice, err := dev.findFullPartName(console, part.Number)
	if err != nil {
		return fullDevice, err
	}

	out, err = dev.expandFilesystem(fullDevice, console)
	if err != nil {
		return out, err
	}

	err = dev.Reload(console)
	if err != nil {
		return "", err
	}

	return out, nil
}

func (dev Disk) findFullPartName(console Console, partNum int) (string, error) {
	allParts, err := console.Run(fmt.Sprintf("lsblk -ltnpo name %s", dev.Device))
	if err != nil {
		return allParts, fmt.Errorf("listing partitions %w", err)
	}

	for _, part := range strings.Split(allParts, "\n") {
		matches, err := regexp.MatchString(fmt.Sprintf("%s.*%d", dev.Device, partNum), part)
		if err != nil {
			return "", err
		}
		if matches {
			return part, nil
		}
	}
	return "", errors.New("no partition found")
}

func (dev Disk) expandFilesystem(device string, console Console) (string, error) {
	var out string
	var err error

	fs, _ := console.Run(fmt.Sprintf("blkid %s -s TYPE -o value", device))

	switch strings.TrimSpace(fs) {
	case "ext2", "ext3", "ext4":
		out, err = console.Run(fmt.Sprintf("e2fsck -fy %s", device))
		if err != nil {
			return out, err
		}
		out, err = console.Run(fmt.Sprintf("resize2fs %s", device))

		if err != nil {
			return out, err
		}
	case "xfs":
		// to grow an xfs fs it needs to be mounted :/
		tmpDir, err := os.MkdirTemp("", "yip")
		defer os.Remove(tmpDir)

		if err != nil {
			return out, err
		}
		out, err = console.Run(fmt.Sprintf("mount -t xfs %s %s", device, tmpDir))
		if err != nil {
			return out, err
		}
		out, err = console.Run(fmt.Sprintf("xfs_growfs %s", tmpDir))
		if err != nil {
			// If we error out, try to umount the dir to not leave it hanging
			out, err2 := console.Run(fmt.Sprintf("umount %s", tmpDir))
			if err2 != nil {
				return out, err2
			}
			return out, err
		}
		out, err = console.Run(fmt.Sprintf("umount %s", tmpDir))
		if err != nil {
			return out, err
		}
	default:
		return "", errors.New(fmt.Sprintf("Could not find filesystem for %s, not resizing the filesystem", device))
	}

	return "", nil
}

func NewGdiskCall(dev string) *GdiskCall {
	return &GdiskCall{dev: dev, wipe: false, parts: []*Partition{}, deletions: []int{}, expand: false, pretend: false}
}

func (gd GdiskCall) buildOptions() []string {
	opts := []string{}

	if gd.pretend {
		opts = append(opts, "-P")
	}

	if gd.wipe {
		opts = append(opts, "--zap-all")
	}

	if gd.expand {
		opts = append(opts, "-e")
	}

	for _, partnum := range gd.deletions {
		opts = append(opts, fmt.Sprintf("-d=%d", partnum))
	}

	for _, part := range gd.parts {
		opts = append(opts, fmt.Sprintf("-n=%d:%d:+%d", part.Number, part.StartS, part.SizeS))

		if part.PLabel != "" {
			opts = append(opts, fmt.Sprintf("-c=%d:%s", part.Number, part.PLabel))
		}

		if part.Type != "" {
			opts = append(opts, fmt.Sprintf("-t=%d:%s", part.Number, part.Type))
		}
	}

	if len(opts) == 0 {
		return nil
	}

	opts = append(opts, gd.dev)
	return opts
}

func (gd GdiskCall) Verify(console Console) (string, error) {
	return console.Run(fmt.Sprintf("sgdisk --verify %s", gd.dev))
}

func (gd GdiskCall) HasUnallocatedSpace(console Console) bool {
	out, _ := gd.Verify(console)
	if unallocated, _ := regexp.MatchString("the end of the disk", out); unallocated {
		return true
	}
	return false
}

func (gd GdiskCall) Print(console Console) (string, error) {
	return console.Run(fmt.Sprintf("sgdisk -p %s", gd.dev))
}

func (gd GdiskCall) Info(partNum int, console Console) (string, error) {
	return console.Run(fmt.Sprintf("sgdisk -i %d %s", partNum, gd.dev))
}

// Parses the output of a GdiskCall.Print call
func (gd GdiskCall) GetLastSector(printOut string) (uint, error) {
	re := regexp.MustCompile("last usable sector is (\\d+)")
	match := re.FindStringSubmatch(printOut)
	if match != nil {
		endS, err := strconv.ParseUint(match[1], 10, 0)
		return uint(endS), err
	}
	return 0, errors.New("Could not determine last usable sector")
}

// Parses the output of a GdiskCall.Print call
// There was a change in the output of sgdisk in version 1.0.2
// https://www.rodsbooks.com/gdisk/revisions.html
// We are trying to match both possible outputs
func (gd GdiskCall) GetSectorSize(printOut string) (uint, error) {

	// Matching: "Logical sector size: 512 bytes"
	re := regexp.MustCompile("sector size: (\\d+)")
	match := re.FindStringSubmatch(printOut)
	if match != nil {
		size, err := strconv.ParseUint(match[1], 10, 0)
		return uint(size), err
	}

	// Matching: "Sector size (logical/physical): 512/512 bytes"
	re = regexp.MustCompile(`Sector size \(logical\/physical\): (\d+)\/\d+ bytes`)
	match = re.FindStringSubmatch(printOut)
	if match != nil {
		size, err := strconv.ParseUint(match[1], 10, 0)
		return uint(size), err
	}

	return 0, errors.New("Could not determine sector size")
}

// Parses the output of a GdiskCall.Print call
func (gd GdiskCall) GetPartitions(printOut string) []Partition {
	re := regexp.MustCompile("^(\\d+)\\s+(\\d+)\\s+(\\d+).*(EF02|EF00|8300)\\s*(.*)$")
	var pType string
	var start uint
	var end uint
	var size uint
	var pLabel string
	var partNum int
	var partitions []Partition

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(printOut)))
	for scanner.Scan() {
		match := re.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if match != nil {
			partNum, _ = strconv.Atoi(match[1])
			parsed, _ := strconv.ParseUint(match[2], 10, 0)
			start = uint(parsed)
			parsed, _ = strconv.ParseUint(match[3], 10, 0)
			end = uint(parsed)
			size = end - start + 1
			pType = match[4]
			pLabel = match[5]

			partitions = append(partitions, Partition{
				Number:     partNum,
				StartS:     start,
				SizeS:      size,
				PLabel:     pLabel,
				FileSystem: "",
				FSLabel:    "",
				Type:       pType,
			})
		}
	}
	return partitions
}

func (gd GdiskCall) GetPartitionData(partNum int, console Console) (*Partition, error) {
	out, err := gd.Info(partNum, console)
	if err != nil {
		return nil, err
	}

	var pType string
	var start uint
	var size uint
	var pLabel string
	if match, _ := regexp.MatchString("Linux filesystem", out); match {
		pType = "8300"
	} else if match, _ = regexp.MatchString("EFI System", out); match {
		pType = "EF00"
	}
	re := regexp.MustCompile("First sector: (\\d+)")
	match := re.FindStringSubmatch(out)
	if match == nil {
		return nil, errors.New("Could not determine start sector")
	}
	parsed, _ := strconv.ParseUint(match[1], 10, 0)
	start = uint(parsed)

	re = regexp.MustCompile("Partition size: (\\d+) sectors")
	match = re.FindStringSubmatch(out)
	if match == nil {
		return nil, errors.New("Could not determine partition size")
	}
	parsed, _ = strconv.ParseUint(match[1], 10, 0)
	size = uint(parsed)

	re = regexp.MustCompile("Partition name: '(.*)'")
	match = re.FindStringSubmatch(out)
	if match == nil {
		return nil, errors.New("Could not determine partition name")
	}
	pLabel = match[1]

	part := Partition{
		Number:     partNum,
		StartS:     start,
		SizeS:      size,
		PLabel:     pLabel,
		FileSystem: "",
		FSLabel:    "",
		Type:       pType,
	}

	return &part, nil
}

func (gd *GdiskCall) WriteChanges(console Console) (string, error) {
	gd.SetPretend(true)
	opts := gd.buildOptions()

	// Run sgdisk with --pretend flag first to as a sanity check
	// before any change to disk happens
	out, err := console.Run(fmt.Sprintf("sgdisk %s", strings.Join(opts[:], " ")))
	if err != nil {
		return out, err
	}

	gd.SetPretend(false)
	opts = gd.buildOptions()
	return console.Run(fmt.Sprintf("sgdisk %s", strings.Join(opts[:], " ")))
}

func (gd *GdiskCall) CreatePartition(p *Partition) {
	gd.parts = append(gd.parts, p)
}

func (gd *GdiskCall) SetPretend(pretend bool) {
	gd.pretend = pretend
}

func (gd *GdiskCall) DeletePartition(num int) {
	gd.deletions = append(gd.deletions, num)
}

func (gd *GdiskCall) WipeTable(wipe bool) {
	gd.wipe = wipe
}

func (gd *GdiskCall) ExpandPTable() {
	gd.expand = true
}

func (mkfs MkfsCall) buildOptions() ([]string, error) {
	opts := []string{}

	linuxFS, _ := regexp.MatchString("ext[2-4]|xfs", mkfs.part.FileSystem)
	fatFS, _ := regexp.MatchString("fat|vfat", mkfs.part.FileSystem)

	switch {
	case linuxFS:
		if mkfs.part.FSLabel != "" {
			opts = append(opts, "-L")
			opts = append(opts, mkfs.part.FSLabel)
		}
		if len(mkfs.customOpts) > 0 {
			opts = append(opts, mkfs.customOpts...)
		}
		opts = append(opts, mkfs.dev)
	case fatFS:
		if mkfs.part.FSLabel != "" {
			opts = append(opts, "-i")
			opts = append(opts, mkfs.part.FSLabel)
		}
		if len(mkfs.customOpts) > 0 {
			opts = append(opts, mkfs.customOpts...)
		}
		opts = append(opts, mkfs.dev)
	default:
		return []string{}, errors.New(fmt.Sprintf("Unsupported filesystem: %s", mkfs.part.FileSystem))
	}
	return opts, nil
}

func (mkfs MkfsCall) Apply(console Console) (string, error) {
	opts, err := mkfs.buildOptions()
	if err != nil {
		return "", err
	}
	tool := fmt.Sprintf("mkfs.%s", mkfs.part.FileSystem)
	command := fmt.Sprintf("%s %s", tool, strings.Join(opts[:], " "))
	return console.Run(command)
}

func MiBToSectors(size uint, sectorSize uint) uint {
	return size * 1048576 / sectorSize
}
