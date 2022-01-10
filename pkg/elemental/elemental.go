/*
Copyright © 2021 SUSE LLC

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

package elemental

import (
	"errors"
	"fmt"
	cnst "github.com/rancher-sandbox/elemental-cli/pkg/constants"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
	"github.com/spf13/afero"
	"os"
	"path/filepath"
	"strings"
)

// Elemental is the struct meant to self-contain most utils and actions related to Elemental, like installing or applying selinux
type Elemental struct {
	config *v1.RunConfig
}

func NewElemental(config *v1.RunConfig) *Elemental {
	return &Elemental{
		config: config,
	}
}

// PartitionAndFormatDevice creates a new empty partition table on target disk
// and applies the configured disk layout by creating and formatting all
// required partitions
func (c *Elemental) PartitionAndFormatDevice(disk *part.Disk) error {
	c.config.Logger.Infof("Partitioning device...")

	err := c.createPTableAndFirmwarePartitions(disk)
	if err != nil {
		return err
	}

	if c.config.PartTable == v1.GPT && c.config.PartLayout != "" {
		return c.config.CloudInitRunner.Run(cnst.PartStage, c.config.PartLayout)
	}

	return c.createDataPartitions(disk)
}

func (c *Elemental) createPTableAndFirmwarePartitions(disk *part.Disk) error {
	c.config.Logger.Debugf("Creating partition table...")
	out, err := disk.NewPartitionTable(c.config.PartTable)
	if err != nil {
		c.config.Logger.Errorf("Failed creating new partition table: %s", out)
		return err
	}

	if c.config.PartTable == v1.GPT {
		err = c.createAndFormatPartition(disk, c.config.Partitions[0])
		if err != nil {
			c.config.Logger.Errorf("Failed creating EFI or BIOS boot partition")
			return err
		}
	}
	return nil
}

func (c *Elemental) createAndFormatPartition(disk *part.Disk, part *v1.Partition) error {
	num, err := disk.AddPartition(part.Size, part.FS, part.PLabel, part.Flags...)
	if err != nil {
		c.config.Logger.Errorf("Failed creating %s partition", part.PLabel)
		return err
	}
	if part.FS != "" {
		out, err := disk.FormatPartition(num, part.FS, part.Label)
		if err != nil {
			c.config.Logger.Errorf("Failed formatting partition: %s", out)
			return err
		}
	} else {
		err = disk.WipeFsOnPartition(num)
		if err != nil {
			c.config.Logger.Errorf("Failed to wipe filesystem of partition %d", num)
			return err
		}
	}
	return nil
}

func (c *Elemental) createDataPartitions(disk *part.Disk) error {
	var dataParts []*v1.Partition
	// Skip the creation of EFI or BIOS partitions on GPT
	if c.config.PartTable == v1.GPT {
		dataParts = c.config.Partitions[1:]
	} else {
		dataParts = c.config.Partitions
	}
	for _, part := range dataParts {
		err := c.createAndFormatPartition(disk, part)
		if err != nil {
			return err
		}
	}
	return nil
}

// MountPartitions mounts state, recovery, oem, persistent and efi partitions.
// Note umounts must be handled by caller logic.
func (c Elemental) MountPartitions() error {
	c.config.Logger.Infof("Mounting disk partitions")
	var err error

	for _, part := range c.config.Partitions {
		if part.MountPoint != "" {
			err = c.MountPartition(part, "rw")
			if err != nil {
				c.UnmountPartitions()
				return err
			}
		}
	}

	return err
}

// UnmountPartitions unmounts recovery, state and oem partitions.
func (c Elemental) UnmountPartitions() error {
	var err error
	errMsg := ""
	failure := false

	// If there is an early error we still try to unmount other partitions
	for _, part := range c.config.Partitions {
		if part.MountPoint != "" {
			err = c.UnmountPartition(part)
			if err != nil {
				errMsg += fmt.Sprintf("Failed to unmount %s\n", part.MountPoint)
				failure = true
			}
		}
	}
	if failure {
		return errors.New(errMsg)
	}
	return nil
}

func (c Elemental) MountPartition(part *v1.Partition, opts ...string) error {
	err := c.config.Fs.MkdirAll(part.MountPoint, 0755)
	if err != nil {
		return err
	}
	device, err := utils.GetDeviceByLabel(c.config.Runner, part.Label)
	if err != nil {
		return err
	}
	err = c.config.Mounter.Mount(device, part.MountPoint, "auto", opts)
	if err != nil {
		return err
	}
	return nil
}

func (c Elemental) UnmountPartition(part *v1.Partition) error {
	// Using IsLikelyNotMountPoint seams to be safe as we are not checking
	// for bind mounts here
	if notMnt, _ := c.config.Mounter.IsLikelyNotMountPoint(part.MountPoint); notMnt == true {
		c.config.Logger.Debugf("Not unmounting partition, %s doesn't look like mountpoint", part.MountPoint)
		return nil
	}
	return c.config.Mounter.Unmount(part.MountPoint)
}

func (c Elemental) MountImage(img *v1.Image) error {
	err := c.config.Fs.MkdirAll(img.MountPoint, 0755)
	if err != nil {
		return err
	}
	out, err := c.config.Runner.Run("losetup", "--show", "-f", img.File)
	if err != nil {
		return err
	}
	loop := strings.TrimSpace(string(out))
	err = c.config.Mounter.Mount(loop, img.MountPoint, "auto", []string{"rw"})
	if err != nil {
		c.config.Runner.Run("losetup", "-d", loop)
		return err
	}
	img.LoopDevice = loop
	return nil
}

func (c Elemental) UnmountImage(img *v1.Image) error {
	// Using IsLikelyNotMountPoint seams to be safe as we are not checking
	// for bind mounts here
	if notMnt, _ := c.config.Mounter.IsLikelyNotMountPoint(img.MountPoint); notMnt == true {
		c.config.Logger.Debugf("Not unmounting image, %s doesn't look like mountpoint", img.MountPoint)
		return nil
	}

	err := c.config.Mounter.Unmount(img.MountPoint)
	if err != nil {
		return err
	}
	_, err = c.config.Runner.Run("losetup", "-d", img.LoopDevice)
	img.LoopDevice = ""
	return err
}

// CreateFileSystemImage creates the image file for config.target
func (c Elemental) CreateFileSystemImage(img v1.Image) error {
	c.config.Logger.Infof("Creating file system image %s", img.File)
	err := c.config.Fs.MkdirAll(filepath.Dir(img.File), 0755)
	if err != nil {
		return err
	}
	actImg, err := c.config.Fs.Create(img.File)
	if err != nil {
		return err
	}

	err = actImg.Truncate(int64(img.Size * 1024 * 1024))
	if err != nil {
		actImg.Close()
		c.config.Fs.RemoveAll(img.File)
		return err
	}
	err = actImg.Close()
	if err != nil {
		c.config.Fs.RemoveAll(img.File)
		return err
	}

	mkfs := part.NewMkfsCall(img.File, img.FS, img.Label, c.config.Runner)
	_, err = mkfs.Apply()
	if err != nil {
		c.config.Fs.RemoveAll(img.File)
		return err
	}
	return nil
}

// CopyCos will rsync from config.source to config.target
func (c *Elemental) CopyCos() error {
	c.config.Logger.Infof("Copying cOS..")
	excludes := []string{"mnt", "proc", "sys", "dev", "tmp"}
	err := utils.CreateDirStructure(c.config.Fs, c.config.ActiveImage.MountPoint)
	if err != nil {
		return err
	}
	err = utils.SyncData(c.config.ActiveImage.RootTree, c.config.ActiveImage.MountPoint, excludes...)
	if err != nil {
		return err
	}
	c.config.Logger.Infof("Finished copying cOS..")
	return nil
}

// CopyCloudConfig will check if there is a cloud init in the config and store it on the target
func (c *Elemental) CopyCloudConfig() error {
	if c.config.CloudInit != "" {
		customConfig := fmt.Sprintf("%s/99_custom.yaml", cnst.OEMDir)
		c.config.Logger.Infof("Trying to copy cloud config file %s to %s", c.config.CloudInit, customConfig)

		if err := c.GetUrl(c.config.CloudInit, customConfig); err != nil {
			return err
		}

		if err := c.config.Fs.Chmod(customConfig, os.ModePerm); err != nil {
			return err
		}
		c.config.Logger.Infof("Finished copying cloud config file %s to %s", c.config.CloudInit, customConfig)
	}
	return nil
}

// SelinuxRelabel will relabel the system if it finds the binary and the context
func (c *Elemental) SelinuxRelabel(raiseError bool) error {
	var err error

	contextFile := fmt.Sprintf("%s/etc/selinux/targeted/contexts/files/file_contexts", cnst.ActiveDir)

	_, err = c.config.Fs.Stat(contextFile)
	contextExists := err == nil

	if utils.CommandExists("setfiles") && contextExists {
		_, err = c.config.Runner.Run("setfiles", "-r", cnst.ActiveDir, contextFile, cnst.ActiveDir)
	}

	// In the original code this can error out and we dont really care
	// I guess that to maintain backwards compatibility we have to do the same, we dont care if it raises an error
	// but we still add the possibility to return an error if we want to change it in the future to be more strict?
	if raiseError && err != nil {
		return err
	} else {
		return nil
	}
}

// CheckNoFormat will make sure that if we set the no format option, the system doesnt already contain a cos system
// by checking the active/passive labels. If they are set then we check if we have the force flag, which means that we
// don't care and proceed to overwrite
func (c *Elemental) CheckNoFormat() error {
	c.config.Logger.Infof("Checking no-format condition")
	// User asked for no format, lets check if there are already those labeled file systems in the disk
	for _, label := range []string{c.config.ActiveImage.Label, c.config.PassiveLabel} {
		found, _ := utils.FindLabel(c.config.Runner, label)
		if found != "" {
			if c.config.Force {
				msg := fmt.Sprintf("Forcing overwrite of existing OS image due to `force` flag")
				c.config.Logger.Infof(msg)
				return nil
			} else {
				msg := fmt.Sprintf("There is already an active deployment in the system, use '--force' flag to overwrite it")
				c.config.Logger.Error(msg)
				return errors.New(msg)
			}
		}
	}
	return nil
}

// BootedFromSquash will check if we are booting from squashfs
func (c Elemental) BootedFromSquash() bool {
	if utils.BootedFrom(c.config.Runner, cnst.RecoveryLabel) {
		return true
	}
	return false
}

// GetIso will check if iso flag is set and if true will try to:
// download the iso to a temp file
// and mount the iso file as loop,
// and modify the IsoMnt var to point to the newly mounted dir
func (c *Elemental) GetIso() error {
	if c.config.Iso != "" {
		tmpDir, err := afero.TempDir(c.config.Fs, "", "elemental")
		if err != nil {
			return err
		}
		tmpFile := fmt.Sprintf("%s/cOs.iso", tmpDir)
		err = c.GetUrl(c.config.Iso, tmpFile)
		if err != nil {
			defer c.config.Fs.RemoveAll(tmpDir)
			return err
		}
		tmpIsoMount, err := afero.TempDir(c.config.Fs, "", "elemental-iso-mounted-")
		if err != nil {
			defer c.config.Fs.RemoveAll(tmpDir)
			return err
		}
		var mountOptions []string
		c.config.Logger.Infof("Mounting iso %s into %s", tmpFile, tmpIsoMount)
		err = c.config.Mounter.Mount(tmpFile, tmpIsoMount, "loop", mountOptions)
		if err != nil {
			defer c.config.Fs.RemoveAll(tmpDir)
			defer c.config.Fs.RemoveAll(tmpIsoMount)
			return err
		}
		// Store the new mounted dir into IsoMnt, so we can use it down the line
		c.config.IsoMnt = tmpIsoMount
		return nil
	}
	return nil
}

// GetUrl is a simple method that will try to get an url to a destination, no matter if its an http url, ftp, tftp or a file
func (c *Elemental) GetUrl(url string, destination string) error {
	var source []byte
	var err error

	switch {
	case strings.HasPrefix(url, "http"), strings.HasPrefix(url, "ftp"), strings.HasPrefix(url, "tftp"):
		c.config.Logger.Infof("Downloading from %s to %s", url, destination)
		resp, err := c.config.Client.Get(url)
		if err != nil {
			return err
		}
		_, err = resp.Body.Read(source)
		defer resp.Body.Close()
	default:
		c.config.Logger.Infof("Copying from %s to %s", url, destination)
		source, err = afero.ReadFile(c.config.Fs, url)
		if err != nil {
			return err
		}
	}

	err = afero.WriteFile(c.config.Fs, destination, source, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

// CopyRecovery will
// Check if we are booting from squash -> false? return
// true? -> :
// mkdir -p RECOVERYDIR
// mkdir -p  RECOVERYDIR/cOS
// if squash -> cp -a RECOVERYSQUASHFS to RECOVERYDIR/cOS/recovery.squashfs
// if not -> cp -a STATEDIR/cOS/active.img to RECOVERYDIR/cOS/recovery.img
// Where:
// RECOVERYDIR is cnst.RecoveryDir
// ISOMNT is /run/initramfs/live by default, can be set to a different dir if COS_INSTALL_ISO_URL is set
// RECOVERYSQUASHFS is $ISOMNT/recovery.squashfs
// RECOVERY is GetDeviceByLabel(cnst.RecoveryLabel)
// either is get from the system if NoFormat is enabled (searching for label COS_RECOVERY) or is a newly generated partition
func (c *Elemental) CopyRecovery() error {
	var err error
	if !c.BootedFromSquash() {
		return nil
	}
	recoveryDirCos := fmt.Sprintf("%s/cOS", cnst.RecoveryDir)
	recoveryDirCosSquashTarget := fmt.Sprintf("%s/cOS/%s", cnst.RecoveryDir, cnst.RecoverySquashFile)
	isoMntCosSquashSource := fmt.Sprintf("%s/%s", c.config.IsoMnt, cnst.RecoverySquashFile)
	imgCosSource := fmt.Sprintf("%s/cOS/%s", cnst.StateDir, c.config.ActiveImage.File)
	imgCosTarget := fmt.Sprintf("%s/cOS/%s", cnst.RecoveryDir, cnst.RecoveryImgFile)

	err = c.config.Fs.MkdirAll(recoveryDirCos, 0755)
	if err != nil {
		return err
	}
	if exists, _ := afero.Exists(c.config.Fs, isoMntCosSquashSource); exists {
		c.config.Logger.Infof("Copying squashfs..")
		err = utils.CopyFile(c.config.Fs, isoMntCosSquashSource, recoveryDirCosSquashTarget)
		if err != nil {
			return err
		}
	} else {
		c.config.Logger.Infof("Copying image file..")
		err = utils.CopyFile(c.config.Fs, imgCosSource, imgCosTarget)
		if err != nil {
			return err
		}
		_, err = c.config.Runner.Run("tune2fs", "-L", c.config.GetSystemLabel(), imgCosTarget)
		if err != nil {
			return err
		}
	}
	c.config.Logger.Infof("Recovery copied")
	return nil
}

func (c Elemental) CopyPassive() error {
	passImgFile := fmt.Sprintf("%s/cOS/%s", cnst.StateDir, cnst.PassiveImgFile)

	c.config.Logger.Infof("Copying image file..")
	err := utils.CopyFile(c.config.Fs, c.config.ActiveImage.File, passImgFile)
	if err != nil {
		return err
	}
	_, err = c.config.Runner.Run("tune2fs", "-L", c.config.PassiveLabel, passImgFile)
	if err != nil {
		c.config.Logger.Errorf("Failed to apply label %s to $s", c.config.PassiveLabel, passImgFile)
		c.config.Fs.Remove(passImgFile)
	}
	return err
}
