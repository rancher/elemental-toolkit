/*
Copyright © 2022 SUSE LLC

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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/elemental"
	"github.com/rancher/elemental-cli/pkg/partitioner"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

var MB = int64(1024 * 1024)
var GB = 1024 * MB

func BuildDiskRun(cfg *v1.BuildConfig, spec *v1.RawDisk, imgType string, oemLabel string, recoveryLabel string, output string) (err error) {
	cfg.Logger.Infof("Building disk image type %s for arch %s", imgType, cfg.Arch)
	if (*spec)[cfg.Arch] == nil {
		msg := fmt.Sprintf("no values in the config for arch %s", cfg.Arch)
		cfg.Logger.Error(msg)
		return errors.New(msg)
	}

	if len((*spec)[cfg.Arch].Packages) == 0 {
		msg := fmt.Sprintf("no packages in the config for arch %s", cfg.Arch)
		cfg.Logger.Error(msg)
		return errors.New(msg)
	}

	if len(cfg.Config.Repos) == 0 {
		msg := fmt.Sprintf("no repositories configured for arch %s", cfg.Arch)
		cfg.Logger.Error(msg)
		return errors.New(msg)
	}

	if oemLabel == "" {
		oemLabel = constants.OEMLabel
	}

	if recoveryLabel == "" {
		recoveryLabel = constants.RecoveryLabel
	}

	e := elemental.NewElemental(&cfg.Config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	// baseDir is where we are going install all packages
	baseDir, err := utils.TempDir(cfg.Fs, "", "elemental-build-disk-files")
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return cfg.Fs.RemoveAll(baseDir) })

	// diskTempDir is where we are going to create all the disk parts
	diskTempDir, err := utils.TempDir(cfg.Fs, "", "elemental-build-disk-parts")
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return cfg.Fs.RemoveAll(diskTempDir) })

	rootfsPart := filepath.Join(diskTempDir, "rootfs.part")
	oemPart := filepath.Join(diskTempDir, "oem.part")
	efiPart := filepath.Join(diskTempDir, "efi.part")

	// Extract required packages to basedir
	for _, pkg := range (*spec)[cfg.Arch].Packages {
		err = os.MkdirAll(filepath.Join(baseDir, pkg.Target), constants.DirPerm)
		if err != nil {
			return err
		}
		imgSource, err := v1.NewSrcFromURI(pkg.Name)
		if err != nil {
			return err
		}
		err = e.DumpSource(
			filepath.Join(baseDir, pkg.Target),
			imgSource,
		)
		if err != nil {
			cfg.Logger.Error(err)
			return err
		}
	}

	// Create rootfs.part
	err = CreatePart(
		cfg,
		rootfsPart,
		filepath.Join(baseDir, "root"),
		recoveryLabel,
		constants.LinuxImgFs,
		2048*MB,
	)
	if err != nil {
		cfg.Logger.Error(err)
		return err
	}

	// create EFI part
	err = CreatePart(
		cfg,
		efiPart,
		"",
		constants.EfiLabel,
		constants.EfiFs,
		20*MB,
	)
	if err != nil {
		cfg.Logger.Error(err)
		return err
	}
	// copy files to efi with mcopy
	_, err = cfg.Runner.Run("mcopy", "-s", "-i", efiPart, filepath.Join(baseDir, "efi", "EFI"), "::EFI")
	if err != nil {
		return err
	}

	// Create the oem part
	// Create the grubenv forcing first boot to be on recovery system
	_ = cfg.Fs.Mkdir(filepath.Join(baseDir, "oem"), constants.DirPerm)
	err = utils.CopyFile(cfg.Fs, filepath.Join(baseDir, "root", "etc", "cos", "grubenv_firstboot"), filepath.Join(baseDir, "oem", "grubenv"))
	if err != nil {
		return err
	}
	err = CreatePart(
		cfg,
		oemPart,
		filepath.Join(baseDir, "oem"),
		oemLabel,
		constants.LinuxImgFs,
		64*MB,
	)
	if err != nil {
		cfg.Logger.Error(err)
		return err
	}

	// Create final image
	err = CreateFinalImage(cfg, output, efiPart, oemPart, rootfsPart)
	if err != nil {
		cfg.Logger.Error(err)
		return err
	}

	switch imgType {
	case "raw":
		// Nothing to do here
		cfg.Logger.Infof("Done! Image created at %s", output)
	case "azure":
		err = Raw2Azure(output, cfg.Fs, cfg.Logger, false)
		if err != nil {
			return err
		}
		cfg.Logger.Infof("Done! Image created at %s", fmt.Sprintf("%s.tar.gz", output))
	case "gce":
		err = Raw2Gce(output, cfg.Fs, cfg.Logger, false)
		if err != nil {
			return err
		}
		cfg.Logger.Infof("Done! Image created at %s", fmt.Sprintf("%s.vhd", output))
	}

	return err
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
		return err
	}
	info, err := actImg.Stat()
	if err != nil {
		return err
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
		return err
	}
	defer file.Close()
	// Create gzip writer
	gzipWriter := gzip.NewWriter(file)
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
		return err
	}
	// copy the actual data
	_, err = io.Copy(tarWriter, sourceFile)
	if err != nil {
		return err
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
		return err
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

// CreateFinalImage creates the final image by truncating the image with the proper sizes, concatenating the contents of the
// given parts and creating the partition table on the image
func CreateFinalImage(c *v1.BuildConfig, img string, parts ...string) error {
	err := utils.MkdirAll(c.Fs, filepath.Dir(img), constants.DirPerm)
	if err != nil {
		return err
	}
	actImg, err := c.Fs.Create(img)
	if err != nil {
		return err
	}

	// add 3MB of initial free space to disk, 1MB is for proper alignment, 2MB are for the hybrid legacy boot.
	err = actImg.Truncate(3 * MB)
	if err != nil {
		actImg.Close()
		_ = c.Fs.RemoveAll(img)
		return err
	}
	// Seek to the end of the file, so we start copying the files at the end of those 3Mb that we truncated before
	_, _ = actImg.Seek(0, io.SeekEnd)
	for _, p := range parts {
		c.Logger.Debugf("Copying %s", p)
		toRead, _ := c.Fs.Open(p)
		_, err = io.Copy(actImg, toRead)
		if err != nil {
			return err
		}
	}

	info, _ := actImg.Stat()
	finalSize := info.Size() + (1 * MB)
	err = actImg.Truncate(finalSize)
	if err != nil {
		actImg.Close()
		_ = c.Fs.RemoveAll(img)
		return err
	}

	err = actImg.Close()
	if err != nil {
		_ = c.Fs.RemoveAll(img)
		return err
	}

	// Partition table
	/*
		Where:
		  -c indicates change the name of the partition in partnum:name format
		  -n new partition in partnum:start:end format
		  -t type of the partition (EF02 bios, EF00 efi and 8300 linux)
	*/
	out, err := c.Runner.Run("sgdisk", "-n", "1:2048:+2M", "-c", "1:legacy", "-t", "1:EF02", img)
	if err != nil {
		c.Logger.Errorf("Error from sgdisk: %s", out)
		return err
	}
	_, err = c.Runner.Run("sgdisk", "-n", "2:0:+20M", "-c", "2:UEFI", "-t", "2:EF00", img)
	if err != nil {
		c.Logger.Errorf("Error from sgdisk: %s", out)
		return err
	}
	_, err = c.Runner.Run("sgdisk", "-n", "3:0:+64M", "-c", "3:oem", "-t", "3:8300", img)
	if err != nil {
		c.Logger.Errorf("Error from sgdisk: %s", out)
		return err
	}
	_, err = c.Runner.Run("sgdisk", "-n", "4:0:+2048M", "-c", "4:root", "-t", "4:8300", img)
	if err != nil {
		c.Logger.Errorf("Error from sgdisk: %s", out)
		return err
	}

	return err
}

// CreatePart creates, truncates, and formats an img.part file. if rootDir is passed it will use that as the rootdir for
// the part creation, thus copying the contents into the newly created part file
func CreatePart(c *v1.BuildConfig, img string, rootDir string, label string, fs string, size int64) error {
	err := utils.MkdirAll(c.Fs, filepath.Dir(img), constants.DirPerm)
	if err != nil {
		return err
	}
	actImg, err := c.Fs.Create(img)
	if err != nil {
		return err
	}

	err = actImg.Truncate(size)
	if err != nil {
		actImg.Close()
		_ = c.Fs.RemoveAll(img)
		return err
	}
	err = actImg.Close()
	if err != nil {
		_ = c.Fs.RemoveAll(img)
		return err
	}

	var extraOpts []string

	// Only add the rootDir if it's not empty
	if rootDir != "" {
		extraOpts = []string{"-d", rootDir}
	}

	mkfs := partitioner.NewMkfsCall(img, fs, label, c.Runner, extraOpts...)
	out, err := mkfs.Apply()
	if err != nil {
		_ = c.Fs.RemoveAll(img)
		c.Logger.Errorf("Error applying mkfs call: %s", out)
		return err
	}
	return err
}
