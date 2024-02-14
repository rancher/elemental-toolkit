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

package action

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/yip/pkg/schema"

	"github.com/rancher/elemental-toolkit/pkg/bootloader"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	"github.com/rancher/elemental-toolkit/pkg/partitioner"
	"github.com/rancher/elemental-toolkit/pkg/snapshotter"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const (
	MB             = int64(1024 * 1024)
	GB             = 1024 * MB
	rootSuffix     = ".root"
	layoutSetStage = "rootfs.before"
	deployStage    = "network"
	postResetHook  = "post-reset"
	cloudinitFile  = "00_disk_layout_setup.yaml"
	defSectorSize  = 512
)

type BuildDiskAction struct {
	cfg         *v1.BuildConfig
	spec        *v1.DiskSpec
	bootloader  v1.Bootloader
	snapshotter v1.Snapshotter
	snapshot    *v1.Snapshot
	// holds the root path within the working directory of all partitions
	roots map[string]string
}

type BuildDiskActionOption func(b *BuildDiskAction) error

func NewBuildDiskAction(cfg *v1.BuildConfig, spec *v1.DiskSpec, opts ...BuildDiskActionOption) (*BuildDiskAction, error) {
	var err error

	b := &BuildDiskAction{cfg: cfg, spec: spec}

	for _, o := range opts {
		err = o(b)
		if err != nil {
			cfg.Logger.Errorf("error applying config option: %s", err.Error())
			return nil, err
		}
	}

	if b.bootloader == nil {
		b.bootloader = bootloader.NewGrub(&cfg.Config)
	}

	if b.snapshotter == nil {
		b.snapshotter, err = snapshotter.NewSnapshotter(cfg.Config, cfg.Snapshotter, b.bootloader)
	}

	if b.cfg.Snapshotter.Type == constants.BtrfsSnapshotterType {
		if !b.spec.Expandable {
			cfg.Logger.Errorf("Non expandable disk images are not supported for btrfs snapshotter")
			return nil, fmt.Errorf("Not supported")
		}
		if spec.Partitions.State.FS != constants.Btrfs {
			cfg.Logger.Warning("Btrfs snapshotter type, forcing btrfs filesystem on state partition")
			spec.Partitions.State.FS = constants.Btrfs
		}
	}

	return b, err
}

func WithDiskBootloader(bootloader v1.Bootloader) BuildDiskActionOption {
	return func(b *BuildDiskAction) error {
		b.bootloader = bootloader
		return nil
	}
}

func (b *BuildDiskAction) buildDiskHook(hook string) error {
	return Hook(&b.cfg.Config, hook, b.cfg.Strict, b.cfg.CloudInitPaths...)
}

func (b *BuildDiskAction) buildDiskChrootHook(hook string, root string) error {
	extraMounts := map[string]string{}
	return ChrootHook(&b.cfg.Config, hook, b.cfg.Strict, root, extraMounts, b.cfg.CloudInitPaths...)
}

func (b *BuildDiskAction) preparePartitionsRoot() error {
	var err error
	var excludes []*v1.Partition

	rootMap := map[string]string{}

	if b.spec.Expandable {
		excludes = append(excludes, b.spec.Partitions.Persistent, b.spec.Partitions.State)
	}
	for _, part := range b.spec.Partitions.PartitionsByInstallOrder(v1.PartitionList{}, excludes...) {
		rootMap[part.Name] = strings.TrimSuffix(part.Path, filepath.Ext(part.Path))
		err = utils.MkdirAll(b.cfg.Fs, rootMap[part.Name], constants.DirPerm)
		if err != nil {
			return err
		}
	}
	b.roots = rootMap
	return nil
}

func (b *BuildDiskAction) BuildDiskRun() (err error) { //nolint:gocyclo
	var rawImg string

	b.cfg.Logger.Infof("Building disk image type %s for arch %s", b.spec.Type, b.cfg.Arch)

	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()
	workdir := filepath.Join(b.cfg.OutDir, constants.DiskWorkDir)
	cleanup.Push(func() error { return b.cfg.Fs.RemoveAll(workdir) })

	// Set output image file
	if b.cfg.Date {
		currTime := time.Now()
		rawImg = fmt.Sprintf("%s.%s.raw", b.cfg.Name, currTime.Format("20060102"))
	} else {
		rawImg = fmt.Sprintf("%s.raw", b.cfg.Name)
	}
	rawImg = filepath.Join(b.cfg.OutDir, rawImg)

	// Before disk hook happens before doing anything
	err = b.buildDiskHook(constants.BeforeDiskHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookBeforeDisk)
	}

	// Prepare partition root folders
	err = b.preparePartitionsRoot()
	if err != nil {
		b.cfg.Logger.Errorf("failed preparing working directories: %s", err.Error())
		return err
	}
	recRoot := filepath.Join(workdir, filepath.Base(b.spec.RecoverySystem.File)+rootSuffix)

	// Create recovery root
	err = elemental.DumpSource(b.cfg.Config, recRoot, b.spec.RecoverySystem.Source)
	if err != nil {
		b.cfg.Logger.Errorf("failed loading recovery image source tree: %s", err.Error())
		return err
	}

	// Copy cloud-init if any
	err = elemental.CopyCloudConfig(b.cfg.Config, b.roots[constants.OEMPartName], b.spec.CloudInit)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}

	// Install grub
	err = b.bootloader.InstallConfig(recRoot, b.roots[constants.EfiPartName])
	if err != nil {
		b.cfg.Logger.Errorf("failed installing grub configuration: %s", err.Error())
		return err
	}

	if b.spec.Expandable {
		err = b.bootloader.SetPersistentVariables(
			filepath.Join(b.roots[constants.OEMPartName], constants.GrubEnv),
			map[string]string{
				"next_entry": constants.RecoveryImgName,
			},
		)
		if err != nil {
			b.cfg.Logger.Errorf("failed setting firstboot menu entry: %s", err.Error())
			return err
		}
	}

	grubVars := b.spec.GetGrubLabels()
	err = b.bootloader.SetPersistentVariables(
		filepath.Join(b.roots[constants.EfiPartName], constants.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		b.cfg.Logger.Errorf("failed setting grub environment variables: %s", err.Error())
		return err
	}

	err = b.bootloader.InstallEFI(
		recRoot, b.roots[constants.EfiPartName],
	)
	if err != nil {
		b.cfg.Logger.Errorf("failed installing grub efi binaries: %s", err.Error())
		return err
	}

	// Rebrand
	err = b.bootloader.SetDefaultEntry(b.roots[constants.EfiPartName], recRoot, b.spec.GrubDefEntry)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}

	// Relabel SELinux
	err = b.applySelinuxLabels(recRoot, b.spec.Expandable)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SelinuxRelabel)
	}

	// After disk hook happens after deploying the OS tree into a temporary folder
	if !b.spec.Expandable {
		err = b.buildDiskChrootHook(constants.AfterDiskChrootHook, recRoot)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.HookAfterDiskChroot)
		}
	}
	err = b.buildDiskHook(constants.AfterDiskHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookAfterDisk)
	}

	// Create recovery image and removes recovery root when done
	err = elemental.CreateImageFromTree(
		b.cfg.Config, &b.spec.RecoverySystem, recRoot, b.spec.Expandable,
		func() error { return b.cfg.Fs.RemoveAll(recRoot) },
	)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating recovery image from root-tree: %s", err.Error())
		return err
	}

	if b.spec.Expandable {
		err = b.SetExpandableCloudInitStage()
		if err != nil {
			b.cfg.Logger.Errorf("failed creating expandable cloud-config: %s", err.Error())
			return err
		}
	} else {
		// Run a snapshotter transaction for System source in state partition
		err = b.snapshotter.InitSnapshotter(b.roots[constants.StatePartName])
		if err != nil {
			b.cfg.Logger.Errorf("failed initializing snapshotter")
			return elementalError.NewFromError(err, elementalError.SnapshotterInit)
		}
		// Starting snapshotter transaction
		b.cfg.Logger.Info("Starting snapshotter transaction")
		b.snapshot, err = b.snapshotter.StartTransaction()
		if err != nil {
			b.cfg.Logger.Errorf("failed to start snapshotter transaction")
			return elementalError.NewFromError(err, elementalError.SnapshotterStart)
		}
		cleanup.PushErrorOnly(func() error { return b.snapshotter.CloseTransactionOnError(b.snapshot) })

		system := b.spec.System
		if b.spec.RecoverySystem.Source.String() == b.spec.System.String() {
			// Reuse already deployed root-tree from recovery image
			system = v1.NewFileSrc(b.spec.RecoverySystem.File)
			b.spec.System.SetDigest(b.spec.RecoverySystem.Source.GetDigest())
		}

		// Deploy system image
		err = elemental.DumpSource(b.cfg.Config, b.snapshot.WorkDir, system)
		if err != nil {
			b.cfg.Logger.Errorf("failed deploying source: %s", system.String())
			return elementalError.NewFromError(err, elementalError.DumpSource)
		}

		// Closing snapshotter transaction
		b.cfg.Logger.Info("Closing snapshotter transaction")
		err = b.snapshotter.CloseTransaction(b.snapshot)
		if err != nil {
			b.cfg.Logger.Errorf("failed closing snapshot transaction: %v", err)
			return err
		}
	}

	// Add state.yaml file on state and recovery partitions
	err = b.createBuildDiskStateYaml(
		b.roots[constants.StatePartName],
		b.roots[constants.RecoveryPartName],
	)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateFile)
	}

	// Creates RAW disk image
	err = b.CreateRAWDisk(rawImg)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating RAW disk: %s", err.Error())
		return err
	}

	err = b.buildDiskHook(constants.PostDiskHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookPostDisk)
	}

	// Convert image to desired format
	switch b.spec.Type {
	case constants.RawType:
		// Nothing to do here
		b.cfg.Logger.Infof("Done! Image created at %s", rawImg)
	case constants.AzureType:
		err = Raw2Azure(rawImg, b.cfg.Fs, b.cfg.Logger, false)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating Azure image: %s", err.Error())
			return err
		}
		b.cfg.Logger.Infof("Done! Image created at %s", fmt.Sprintf("%s.vhd", rawImg))
	case constants.GCEType:
		err = Raw2Gce(rawImg, b.cfg.Fs, b.cfg.Logger, false)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating GCE image: %s", err.Error())
			return err
		}
		b.cfg.Logger.Infof("Done! Image created at %s", fmt.Sprintf("%s.tar.gz", rawImg))
	}

	return elementalError.NewFromError(err, elementalError.Unknown)
}

// CreateRAWDisk creates the RAW disk image file including all required partitions
func (b *BuildDiskAction) CreateRAWDisk(rawImg string) error {
	// Creates all partition image files
	images, err := b.CreatePartitionImages()
	if err != nil {
		b.cfg.Logger.Errorf("failed creating partition images: %s", err.Error())
		return err
	}

	// Check if disk already exists
	if exists, _ := utils.Exists(b.cfg.Fs, rawImg); exists {
		b.cfg.Logger.Warnf("Overwriting already existing %s", rawImg)
		err := b.cfg.Fs.Remove(rawImg)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.RemoveFile)
		}
	}

	// Ensamble disk with all partitions
	err = b.CreateDiskImage(rawImg, images...)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating disk image: %s", err.Error())
		return err
	}

	// Write partition headers to disk
	err = b.CreateDiskPartitionTable(rawImg)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating partition table: %s", err.Error())
		return err
	}
	return nil
}

// CreatePartitionImage creates partition image files and returns a slice of the created images
func (b *BuildDiskAction) CreatePartitionImages() ([]*v1.Image, error) {
	var err error
	var img *v1.Image
	var images []*v1.Image
	var excludes v1.PartitionList

	excludes = append(excludes, b.spec.Partitions.EFI)
	if b.spec.Expandable {
		excludes = append(excludes, b.spec.Partitions.State, b.spec.Partitions.Persistent)
	}

	b.cfg.Logger.Infof("Creating EFI partition image")
	img = b.spec.Partitions.EFI.ToImage()
	err = elemental.CreateFileSystemImage(b.cfg.Config, img, "", false)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating EFI image: %s", err.Error())
		return nil, err
	}

	err = utils.WalkDirFs(b.cfg.Fs, b.roots[constants.EfiPartName], func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path != b.roots[constants.EfiPartName] {
			rel, err := filepath.Rel(b.roots[constants.EfiPartName], path)
			if err != nil {
				return err
			}

			b.cfg.Logger.Debugf("copying file %s to %s", path, rel)
			_, err = b.cfg.Runner.Run("mcopy", "-n", "-o", "-i", img.File, path, fmt.Sprintf("::%s", rel))
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		b.cfg.Logger.Errorf("failed copying files to EFI img: %s", err.Error())
		return nil, err
	}

	images = append(images, img)

	// Create all partitions after EFI
	for _, part := range b.spec.Partitions.PartitionsByInstallOrder(v1.PartitionList{}, excludes...) {
		b.cfg.Logger.Infof("Creating %s partition image", part.Name)
		img = part.ToImage()
		err = elemental.CreateImageFromTree(
			b.cfg.Config, img, b.roots[part.Name], b.spec.Expandable,
			func() error { return b.cfg.Fs.RemoveAll(b.roots[part.Name]) },
		)
		if err != nil {
			b.cfg.Logger.Errorf("failed creating %s partition image: %s", part.Name, err.Error())
			return nil, err
		}
		images = append(images, img)
	}

	return images, nil
}

// CreateDiskImage creates the final image by truncating the image with the proper size and
// concatenating the contents of the given partitions. No partition table is written
func (b *BuildDiskAction) CreateDiskImage(rawDiskFile string, partImgs ...*v1.Image) error {
	var initDiskFile, endDiskFile string
	var err error
	var partFiles []string

	initDiskFile = filepath.Join(b.cfg.OutDir, constants.DiskWorkDir, "initdisk.img")
	endDiskFile = filepath.Join(b.cfg.OutDir, constants.DiskWorkDir, "enddisk.img")

	b.cfg.Logger.Infof("Creating RAW disk %s", rawDiskFile)

	// create 1MB of initial free space to disk for proper alignment and leave
	// room for GPT headers. Extra space of, at least, 1MB is also considered at the
	// end of the disk for GPT headers.
	err = utils.CreateRAWFile(b.cfg.Fs, initDiskFile, 1)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating RAW file: %s", err.Error())
		return err
	}

	// Compute extra space required at the end
	eSize := uint(1)
	if b.spec.Size > 0 {
		minSize := b.spec.MinDiskSize()
		if b.spec.Size > minSize {
			eSize = b.spec.Size - b.spec.MinDiskSize()
		} else {
			return elementalError.New(
				fmt.Sprintf("Configured size (%dMiB) is not big enough, minimum requested is %dMiB ", b.spec.Size, minSize),
				elementalError.InvalidSize,
			)
		}
	}
	err = utils.CreateRAWFile(b.cfg.Fs, endDiskFile, eSize)
	if err != nil {
		b.cfg.Logger.Errorf("failed creating RAW file: %s", err.Error())
		return err
	}

	// List and concatenate all image files
	partFiles = append(partFiles, initDiskFile)
	for _, img := range partImgs {
		partFiles = append(partFiles, img.File)
	}
	partFiles = append(partFiles, endDiskFile)
	err = utils.ConcatFiles(b.cfg.Fs, partFiles, rawDiskFile)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyData)
	}

	return nil
}

// Raw2Gce transforms an image from RAW format into GCE format
// THIS REMOVES THE SOURCE IMAGE BY DEFAULT
func Raw2Gce(source string, fs v1.FS, logger v1.Logger, keepOldImage bool) error {
	// The RAW image file must have a size in an increment of 1 GB. For example, the file must be either 10 GB or 11 GB but not 10.5 GB.
	// The disk image filename must be disk.raw.
	// The compressed file must be a .tar.gz file that uses gzip compression and the --format=oldgnu option for the tar utility.
	logger.Info("Transforming raw image into gce format")
	actImg, err := fs.OpenFile(source, os.O_CREATE|os.O_APPEND|os.O_WRONLY, constants.FilePerm)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.OpenFile)
	}
	info, err := actImg.Stat()
	if err != nil {
		return elementalError.NewFromError(err, elementalError.StatFile)
	}
	actualSize := info.Size()
	finalSizeGB := actualSize/GB + 1
	finalSizeBytes := finalSizeGB * GB
	logger.Infof("Resizing img from %d to %d", actualSize, finalSizeBytes)
	// REMEMBER TO SEEK!
	_, _ = actImg.Seek(0, io.SeekEnd)
	_ = actImg.Truncate(finalSizeBytes)
	_ = actImg.Close()

	// Tar gz the image
	logger.Infof("Compressing raw image into a tar.gz")
	// Create destination file
	file, err := fs.Create(fmt.Sprintf("%s.tar.gz", source))
	logger.Debugf(fmt.Sprintf("destination: %s.tar.gz", source))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateFile)
	}
	defer file.Close()
	// Create gzip writer
	gzipWriter, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.GzipWriter)
	}
	defer gzipWriter.Close()
	// Create tarwriter pointing to our gzip writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Open disk.raw
	sourceFile, _ := fs.Open(source)
	sourceStat, _ := sourceFile.Stat()
	defer sourceFile.Close()

	// Add disk.raw file
	header := &tar.Header{
		Name:   sourceStat.Name(),
		Size:   sourceStat.Size(),
		Mode:   int64(sourceStat.Mode()),
		Format: tar.FormatGNU,
	}
	// Write header with all the info
	err = tarWriter.WriteHeader(header)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.TarHeader)
	}
	// copy the actual data
	_, err = io.Copy(tarWriter, sourceFile)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyData)
	}
	// Remove full raw image, we already got the compressed one
	if !keepOldImage {
		_ = fs.RemoveAll(source)
	}
	return nil
}

// Raw2Azure transforms an image from RAW format into Azure format
// THIS REMOVES THE SOURCE IMAGE BY DEFAULT
func Raw2Azure(source string, fs v1.FS, logger v1.Logger, keepOldImage bool) error {
	// All VHDs on Azure must have a virtual size aligned to 1 MB (1024 × 1024 bytes)
	// The Hyper-V virtual hard disk (VHDX) format isn't supported in Azure, only fixed VHD
	logger.Info("Transforming raw image into azure format")
	// Copy raw to new image with VHD appended
	err := utils.CopyFile(fs, source, fmt.Sprintf("%s.vhd", source))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}
	// Open it
	vhdFile, _ := fs.OpenFile(fmt.Sprintf("%s.vhd", source), os.O_APPEND|os.O_WRONLY, 0600)
	// Calculate rounded size
	info, _ := vhdFile.Stat()
	actualSize := info.Size()
	finalSizeBytes := ((actualSize + MB - 1) / MB) * MB
	// Don't forget to remove 512 bytes for the header that we are going to add afterwards!
	finalSizeBytes = finalSizeBytes - 512
	// For smaller than 1 MB images, this calculation doesn't work, so we round up to 1 MB
	if finalSizeBytes == 0 {
		finalSizeBytes = 1*1024*1024 - 512
	}
	if actualSize != finalSizeBytes {
		logger.Infof("Resizing img from %d to %d", actualSize, finalSizeBytes+512)
		_, _ = vhdFile.Seek(0, io.SeekEnd)
		_ = vhdFile.Truncate(finalSizeBytes)
	}
	// Transform it to VHD
	utils.RawDiskToFixedVhd(vhdFile)
	_ = vhdFile.Close()
	// Remove raw image
	if !keepOldImage {
		_ = fs.RemoveAll(source)
	}
	return nil
}

func (b *BuildDiskAction) CreateDiskPartitionTable(disk string) error {
	var secSize, startS, sizeS uint
	var excludes v1.PartitionList

	gd := partitioner.NewPartitioner(disk, b.cfg.Runner, partitioner.Gdisk)
	dData, err := gd.Print()
	if err != nil {
		return err
	}
	secSize, err = gd.GetSectorSize(dData)
	if err != nil {
		secSize = defSectorSize
		b.cfg.Logger.Warnf("Could not determine disk sector size, using default value (%d bytes)", defSectorSize)
	}

	if b.spec.Expandable {
		excludes = append(excludes, b.spec.Partitions.State, b.spec.Partitions.Persistent)
	}
	elParts := b.spec.Partitions.PartitionsByInstallOrder(v1.PartitionList{}, excludes...)
	for i, part := range elParts {
		if i == 0 {
			//First partition is aligned at 1MiB
			startS = 1024 * 1024 / secSize
		} else {
			// reuse startS and SizeS from previous partition
			startS = startS + sizeS
		}
		sizeS = partitioner.MiBToSectors(part.Size, secSize)
		var gdPart = partitioner.Partition{
			Number:     i + 1,
			StartS:     startS,
			SizeS:      sizeS,
			PLabel:     part.Name,
			FileSystem: part.FS,
		}
		gd.CreatePartition(&gdPart)
	}
	out, err := gd.WriteChanges()
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating partitions. stdout: %s\nerr:%v", out, err)
		return err
	}
	return nil
}

// applySelinuxLabels sets SELinux extended attributes to the root-tree being installed. Swallows errors, label on a best effort
func (b *BuildDiskAction) applySelinuxLabels(root string, unprivileged bool) error {
	if unprivileged {
		// Swallow errors, label on a best effort when not chrooting
		return elemental.SelinuxRelabel(b.cfg.Config, root, false)
	}
	binds := map[string]string{}
	return utils.ChrootedCallback(
		&b.cfg.Config, root, binds, func() error { return elemental.SelinuxRelabel(b.cfg.Config, "/", true) },
	)
}

func (b *BuildDiskAction) createBuildDiskStateYaml(stateRoot, recoveryRoot string) error {
	if b.spec.Partitions.Recovery == nil {
		return fmt.Errorf("undefined recovery partition")
	}
	if b.spec.Partitions.State == nil && !b.spec.Expandable {
		return fmt.Errorf("undefined state partition")
	}

	snapshots := map[int]*v1.SystemState{}
	if !b.spec.Expandable {
		snapshots[b.snapshot.ID] = &v1.SystemState{
			Source: b.spec.System,
			Digest: b.spec.System.GetDigest(),
			Active: true,
		}
	}

	installState := &v1.InstallState{
		Date:        time.Now().Format(time.RFC3339),
		Snapshotter: b.cfg.Snapshotter,
		Partitions: map[string]*v1.PartitionState{
			constants.StatePartName: {
				FSLabel:   b.spec.Partitions.State.FilesystemLabel,
				Snapshots: snapshots,
			},
			constants.RecoveryPartName: {
				FSLabel: b.spec.Partitions.Recovery.FilesystemLabel,
				RecoveryImage: &v1.SystemState{
					Source: b.spec.RecoverySystem.Source,
					Digest: b.spec.RecoverySystem.Source.GetDigest(),
					Label:  b.spec.RecoverySystem.Label,
					FS:     b.spec.RecoverySystem.FS,
				},
			},
		},
	}

	if b.spec.Partitions.OEM != nil {
		installState.Partitions[constants.OEMPartName] = &v1.PartitionState{
			FSLabel: b.spec.Partitions.OEM.FilesystemLabel,
		}
	}
	if b.spec.Partitions.Persistent != nil {
		installState.Partitions[constants.PersistentPartName] = &v1.PartitionState{
			FSLabel: b.spec.Partitions.Persistent.FilesystemLabel,
		}
	}
	if b.spec.Partitions.EFI != nil {
		installState.Partitions[constants.EfiPartName] = &v1.PartitionState{
			FSLabel: b.spec.Partitions.EFI.FilesystemLabel,
		}
	}

	statePath := ""
	if !b.spec.Expandable {
		statePath = filepath.Join(stateRoot, constants.InstallStateFile)
	}

	return b.cfg.WriteInstallState(
		installState, statePath,
		filepath.Join(recoveryRoot, constants.InstallStateFile),
	)
}

func (b *BuildDiskAction) SetExpandableCloudInitStage() error {
	var deployCmd []string

	deployCmd = b.spec.DeployCmd
	if b.spec.System.String() != b.spec.RecoverySystem.Source.String() && !b.spec.System.IsEmpty() {
		deployCmd = append(deployCmd, "--system", b.spec.System.String())
	}

	conf := &schema.YipConfig{
		Name: "Expand disk layout",
		Stages: map[string][]schema.Stage{
			layoutSetStage: {
				schema.Stage{
					Name: "Add state partition",
					Layout: schema.Layout{
						Device: &schema.Device{
							Label: b.spec.Partitions.Recovery.FilesystemLabel,
						},
						Parts: []schema.Partition{
							{
								FSLabel:    b.spec.Partitions.State.FilesystemLabel,
								Size:       b.spec.Partitions.State.Size,
								PLabel:     b.spec.Partitions.State.Name,
								FileSystem: b.spec.Partitions.State.FS,
							},
						},
					},
				}, schema.Stage{
					Name: "Add persistent partition",
					Layout: schema.Layout{
						Device: &schema.Device{
							Label: b.spec.Partitions.Recovery.FilesystemLabel,
						},
						Parts: []schema.Partition{
							{
								FSLabel:    b.spec.Partitions.Persistent.FilesystemLabel,
								Size:       b.spec.Partitions.Persistent.Size,
								PLabel:     b.spec.Partitions.Persistent.Name,
								FileSystem: b.spec.Partitions.Persistent.FS,
							},
						},
					},
				},
			}, deployStage: {
				schema.Stage{
					If:   `[ -f "/run/elemental/recovery_mode" ]`,
					Name: "Deploy active system",
					Commands: []string{
						strings.Join(deployCmd, " "),
					},
				},
			}, postResetHook: {
				schema.Stage{
					If:   `[ -f "/oem/` + cloudinitFile + `" ]`,
					Name: "Cleanup expand disk init stages",
					Commands: []string{
						fmt.Sprintf("rm /oem/%s", cloudinitFile),
					},
				},
			},
		},
	}

	return b.cfg.CloudInitRunner.CloudInitFileRender(filepath.Join(b.roots[constants.OEMPartName], cloudinitFile), conf)
}
