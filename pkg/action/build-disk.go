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
	"io"
	"os"
	"path/filepath"

	"github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/partitioner"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
)

var MB = int64(1024 * 1024)

func BuildDiskRun(cfg *v1.BuildConfig, imgType string, arch string, oemLabel string, recoveryLabel string, output string) (err error) {
	cfg.Logger.Infof("Building disk image type %s for arch %s", imgType, arch)

	if oemLabel == "" {
		oemLabel = constants.OEMLabel
	}

	if recoveryLabel == "" {
		recoveryLabel = constants.RecoveryLabel
	}

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
	for _, pkg := range cfg.RawDisk[arch].Packages {
		err = os.MkdirAll(filepath.Join(baseDir, pkg.Target), constants.DirPerm)
		if err != nil {
			return err
		}
		err = applySources(cfg.Config, filepath.Join(baseDir, pkg.Target), pkg.Name)
		if err != nil {
			cfg.Logger.Error(err)
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

	return err
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
