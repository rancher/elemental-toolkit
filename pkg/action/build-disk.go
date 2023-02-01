/*
Copyright © 2022 - 2023 SUSE LLC

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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/elemental"
	eleError "github.com/rancher/elemental-cli/pkg/error"
	elementalError "github.com/rancher/elemental-cli/pkg/error"
	"github.com/rancher/elemental-cli/pkg/partitioner"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

const (
	MB = int64(1024 * 1024)
	GB = 1024 * MB
)

func BuildDiskRun(cfg *v1.BuildConfig, spec *v1.Disk) error {
	var err error
	//var recInfo, activeInfo interface{}
	cfg.Logger.Infof("Building disk image type %s for arch %s", spec.Type, cfg.Arch)

	e := elemental.NewElemental(&cfg.Config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	cleanup.Push(func() error {
		return cfg.Fs.RemoveAll(filepath.Join(cfg.OutDir, constants.DiskWorkDir))
	})

	// Create recovery root
	recRoot := strings.TrimSuffix(spec.Recovery.File, filepath.Ext(spec.Recovery.File))
	activeRoot := recRoot
	// TODO keep and store meta
	_, err = e.DumpSource(recRoot, spec.Recovery.Source)
	if err != nil {
		return err
	}

	// Create recovery image
	err = e.CreateImgFromTreeNoMounts(recRoot, &spec.Recovery, nil)
	if err != nil {
		return err
	}

	if !spec.RecoveryOnly {
		//activeInfo = recInfo
		if spec.Active.Source.Value() != spec.Recovery.File {
			// Create active root
			activeRoot = strings.TrimSuffix(spec.Active.File, filepath.Ext(spec.Active.File))
			// TODO keep and store meta
			_, err = e.DumpSource(recRoot, spec.Active.Source)
			if err != nil {
				return err
			}
		}

		// Create active image
		err = e.CreateImgFromTreeNoMounts(activeRoot, &spec.Active, nil)
		if err != nil {
			return err
		}

		// Create passive image
		err = e.CreateImgFromTreeNoMounts(activeRoot, &spec.Passive, nil)
		if err != nil {
			return err
		}
	}

	// Prepare partition images
	efiImg := spec.Partitions.EFI.ToImage()
	oemImg := spec.Partitions.OEM.ToImage()
	recoveryImg := spec.Partitions.Recovery.ToImage()
	stateImg := spec.Partitions.State.ToImage()

	stateRoot := strings.TrimSuffix(stateImg.File, filepath.Ext(stateImg.File))
	recoveryRoot := strings.TrimSuffix(recoveryImg.File, filepath.Ext(recoveryImg.File))
	oemRoot := strings.TrimSuffix(oemImg.File, filepath.Ext(oemImg.File))
	efiRoot := strings.TrimSuffix(efiImg.File, filepath.Ext(efiImg.File))

	// Install grub
	grub := utils.NewGrub(&cfg.Config)
	err = grub.InstallConfig(activeRoot, stateRoot, spec.GrubConf)
	if err != nil {
		return err
	}
	_, err = grub.InstallEFI(activeRoot, stateRoot, efiRoot, spec.Partitions.State.FilesystemLabel)
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return cfg.Fs.RemoveAll(efiRoot) })

	// Rebrand
	err = e.SetDefaultGrubEntry(stateRoot, activeRoot, spec.GrubDefEntry)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}

	// Include additional cloud-init files
	err = e.CopyCloudConfig(oemRoot, spec.CloudInit)
	if err != nil {
		return err
	}

	// Remove prepared root trees, they are already synched
	err = cfg.Fs.RemoveAll(recRoot)
	if err != nil {
		return err
	}
	err = cfg.Fs.RemoveAll(activeRoot)
	if err != nil {
		return err
	}

	// TODO create state yaml files for recovery and state partitions

	// Create EFI part image
	err = e.CreateFileSystemImage(efiImg)
	if err != nil {
		return err
	}
	_, err = cfg.Runner.Run("mcopy", "-s", "-i", efiImg.File, filepath.Join(efiRoot, "EFI"), "::EFI")
	if err != nil {
		return eleError.NewFromError(err, eleError.CommandRun)
	}

	// Create OEM part image
	err = e.CreateImgFromTreeNoMounts(oemRoot, oemImg, func() error { return cfg.Fs.RemoveAll(oemRoot) })
	if err != nil {
		return err
	}

	// Create recovery part image
	err = e.CreateImgFromTreeNoMounts(recoveryRoot, recoveryImg, func() error { return cfg.Fs.RemoveAll(recoveryRoot) })
	if err != nil {
		return err
	}

	// Create state part image
	err = e.CreateImgFromTreeNoMounts(stateRoot, stateImg, func() error { return cfg.Fs.RemoveAll(stateRoot) })
	if err != nil {
		return err
	}

	// TODO set persistent partition
	/*if !spec.RecoveryOnly {
		// Create persistent part image
	}*/

	// Ensamble disk
	rawImg, err := CreateDiskImage(cfg, efiImg, oemImg, recoveryImg, stateImg)
	if err != nil {
		return err
	}

	// Write partition headers to disk
	err = CreateDiskPartitionTable(cfg, spec, rawImg)
	if err != nil {
		return err
	}

	// Convert image to desired format
	switch spec.Type {
	case constants.RawType:
		// Nothing to do here
		cfg.Logger.Infof("Done! Image created at %s", rawImg)
	case constants.AzureType:
		err = Raw2Azure(rawImg, cfg.Fs, cfg.Logger, false)
		if err != nil {
			return err
		}
		cfg.Logger.Infof("Done! Image created at %s", fmt.Sprintf("%s.vhd", rawImg))
	case constants.GCEType:
		err = Raw2Gce(rawImg, cfg.Fs, cfg.Logger, false)
		if err != nil {
			return err
		}
		cfg.Logger.Infof("Done! Image created at %s", fmt.Sprintf("%s.tar.gz", rawImg))
	}

	return eleError.NewFromError(err, eleError.Unknown)
}

// CreateDiskImage creates the final image by truncating the image with the proper size and
// concatenating the contents of the given partitions. No partition table is written
func CreateDiskImage(cfg *v1.BuildConfig, partImgs ...*v1.Image) (string, error) {
	var rawDiskFile, initDiskFile string
	var partFiles []string

	if cfg.Date {
		currTime := time.Now()
		rawDiskFile = fmt.Sprintf("%s.%s.raw", cfg.Name, currTime.Format("20060102"))
	} else {
		rawDiskFile = fmt.Sprintf("%s.raw", cfg.Name)
	}
	rawDiskFile = filepath.Join(cfg.OutDir, rawDiskFile)
	initDiskFile = filepath.Join(cfg.OutDir, constants.DiskWorkDir, "initdisk.img")

	// create 1MB of initial free space to disk for proper alignment and leave
	// room for GPT headers. This extra space will be appended at the end too.
	initImg, err := cfg.Fs.Create(initDiskFile)
	if err != nil {
		return "", eleError.NewFromError(err, eleError.CreateFile)
	}
	err = initImg.Truncate(1 * MB)
	if err != nil {
		initImg.Close()
		_ = cfg.Fs.RemoveAll(initDiskFile)
		return "", eleError.NewFromError(err, eleError.TruncateFile)
	}
	err = initImg.Close()
	if err != nil {
		_ = cfg.Fs.RemoveAll(initDiskFile)
		return "", eleError.NewFromError(err, eleError.CloseFile)
	}

	// List and concatenate all image files
	partFiles = append(partFiles, initDiskFile)
	for _, img := range partImgs {
		partFiles = append(partFiles, img.File)
	}
	partFiles = append(partFiles, initDiskFile)
	err = utils.ConcatFiles(cfg.Fs, partFiles, rawDiskFile)
	if err != nil {
		return "", eleError.NewFromError(err, eleError.CopyData)
	}

	return rawDiskFile, nil
}

// Raw2Gce transforms an image from RAW format into GCE format
// THIS REMOVES THE SOURCE IMAGE BY DEFAULT
func Raw2Gce(source string, fs v1.FS, logger v1.Logger, keepOldImage bool) error {
	// The RAW image file must have a size in an increment of 1 GB. For example, the file must be either 10 GB or 11 GB but not 10.5 GB.
	// The disk image filename must be disk.raw.
	// The compressed file must be a .tar.gz file that uses gzip compression and the --format=oldgnu option for the tar utility.
	logger.Info("Transforming raw image into gce format")
	actImg, err := fs.Open(source)
	if err != nil {
		return eleError.NewFromError(err, eleError.OpenFile)
	}
	info, err := actImg.Stat()
	if err != nil {
		return eleError.NewFromError(err, eleError.StatFile)
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
		return eleError.NewFromError(err, eleError.CreateFile)
	}
	defer file.Close()
	// Create gzip writer
	gzipWriter, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
	if err != nil {
		return eleError.NewFromError(err, eleError.GzipWriter)
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
		return eleError.NewFromError(err, eleError.TarHeader)
	}
	// copy the actual data
	_, err = io.Copy(tarWriter, sourceFile)
	if err != nil {
		return eleError.NewFromError(err, eleError.CopyData)
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
		return eleError.NewFromError(err, eleError.CopyFile)
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

func CreateDiskPartitionTable(cfg *v1.BuildConfig, spec *v1.Disk, disk string) error {
	var secSize, startS, sizeS uint

	gd := partitioner.NewGdiskCall(disk, cfg.Runner)
	dData, err := gd.Print()
	if err != nil {
		return err
	}
	secSize, err = gd.GetSectorSize(dData)
	if err != nil {
		return err
	}

	elParts := spec.Partitions.PartitionsByInstallOrder(v1.PartitionList{})
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
			Number: i + 1,
			StartS: startS,
			SizeS:  sizeS,
			PLabel: part.Name,
		}
		gd.CreatePartition(&gdPart)
	}
	out, err := gd.WriteChanges()
	cfg.Logger.Debugf("sgdisk output: %s", out)
	if err != nil {
		cfg.Logger.Errorf("Failed creating partition: %v", err)
		return err
	}
	return nil
}
