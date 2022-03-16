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

package elemental

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	cnst "github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/partitioner"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
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

// FormatPartition will format an already existing partition
func (c *Elemental) FormatPartition(part *v1.Partition, opts ...string) error {
	c.config.Logger.Infof("Formatting '%s' partition", part.Name)
	return partitioner.FormatDevice(c.config.Runner, part.Path, part.FS, part.Label, opts...)
}

// PartitionAndFormatDevice creates a new empty partition table on target disk
// and applies the configured disk layout by creating and formatting all
// required partitions
func (c *Elemental) PartitionAndFormatDevice(disk *partitioner.Disk) error {
	c.config.Logger.Infof("Partitioning device...")

	err := c.createPTableAndFirmwarePartitions(disk)
	if err != nil {
		return err
	}

	if c.config.PartLayout != "" {
		c.config.Logger.Infof("Setting custom partitions from %s...", c.config.PartLayout)
		return c.config.CloudInitRunner.Run(cnst.PartStage, c.config.PartLayout)
	}

	return c.createDataPartitions(disk)
}

func (c *Elemental) createPTableAndFirmwarePartitions(disk *partitioner.Disk) error {
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

func (c *Elemental) createAndFormatPartition(disk *partitioner.Disk, part *v1.Partition) error {
	c.config.Logger.Debugf("Adding partition %s", part.Name)
	num, err := disk.AddPartition(part.Size, part.FS, part.Name, part.Flags...)
	if err != nil {
		c.config.Logger.Errorf("Failed creating %s partition", part.Name)
		return err
	}
	partDev, err := disk.FindPartitionDevice(num)
	if err != nil {
		return err
	}
	if part.FS != "" {
		c.config.Logger.Debugf("Formatting partition with label %s", part.Label)
		err = partitioner.FormatDevice(c.config.Runner, partDev, part.FS, part.Label)
		if err != nil {
			c.config.Logger.Errorf("Failed formatting partition %s", part.Name)
			return err
		}
	} else {
		c.config.Logger.Debugf("Wipe file system on %s", part.Name)
		err = disk.WipeFsOnPartition(partDev)
		if err != nil {
			c.config.Logger.Errorf("Failed to wipe filesystem of partition %s", partDev)
			return err
		}
	}
	part.Path = partDev
	return nil
}

func (c *Elemental) createDataPartitions(disk *partitioner.Disk) error {
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

// MountPartitions mounts configured partitions. Partitions with an unset mountpoint are not mounted.
// Note umounts must be handled by caller logic.
func (c Elemental) MountPartitions() error {
	c.config.Logger.Infof("Mounting disk partitions")
	var err error

	for _, part := range c.config.Partitions {
		if part.MountPoint != "" {
			err = c.MountPartition(part, "rw")
			if err != nil {
				_ = c.UnmountPartitions()
				return err
			}
		}
	}

	return err
}

// UnmountPartitions unmounts configured partitiosn. Partitions with an unset mountpoint are not unmounted.
func (c Elemental) UnmountPartitions() error {
	c.config.Logger.Infof("Unmounting disk partitions")
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

// MountPartition mounts a partition with the given mount options
func (c Elemental) MountPartition(part *v1.Partition, opts ...string) error {
	c.config.Logger.Debugf("Mounting partition %s", part.Label)
	err := utils.MkdirAll(c.config.Fs, part.MountPoint, cnst.DirPerm)
	if err != nil {
		return err
	}
	if part.Path == "" {
		// Lets error out only after 10 attempts to find the device
		device, err := utils.GetDeviceByLabel(c.config.Runner, part.Label, 10)
		if err != nil {
			c.config.Logger.Errorf("Could not find a device with label %s", part.Label)
			return err
		}
		part.Path = device
	}
	err = c.config.Mounter.Mount(part.Path, part.MountPoint, "auto", opts)
	if err != nil {
		c.config.Logger.Errorf("Failed mounting device %s with label %s", part.Path, part.Label)
		return err
	}
	return nil
}

// UnmountPartition unmounts the given partition or does nothing if not mounted
func (c Elemental) UnmountPartition(part *v1.Partition) error {
	// Using IsLikelyNotMountPoint seams to be safe as we are not checking
	// for bind mounts here
	if notMnt, _ := c.config.Mounter.IsLikelyNotMountPoint(part.MountPoint); notMnt {
		c.config.Logger.Debugf("Not unmounting partition, %s doesn't look like mountpoint", part.MountPoint)
		return nil
	}
	c.config.Logger.Debugf("Unmounting partition %s", part.Label)
	return c.config.Mounter.Unmount(part.MountPoint)
}

// MountImage mounts an image with the given mount options
func (c Elemental) MountImage(img *v1.Image, opts ...string) error {
	c.config.Logger.Debugf("Mounting image %s", img.Label)
	err := utils.MkdirAll(c.config.Fs, img.MountPoint, cnst.DirPerm)
	if err != nil {
		return err
	}
	out, err := c.config.Runner.Run("losetup", "--show", "-f", img.File)
	if err != nil {
		return err
	}
	loop := strings.TrimSpace(string(out))
	err = c.config.Mounter.Mount(loop, img.MountPoint, "auto", opts)
	if err != nil {
		_, _ = c.config.Runner.Run("losetup", "-d", loop)
		return err
	}
	img.LoopDevice = loop
	return nil
}

// UnmountImage unmounts the given image or does nothing if not mounted
func (c Elemental) UnmountImage(img *v1.Image) error {
	// Using IsLikelyNotMountPoint seams to be safe as we are not checking
	// for bind mounts here
	if notMnt, _ := c.config.Mounter.IsLikelyNotMountPoint(img.MountPoint); notMnt {
		c.config.Logger.Debugf("Not unmounting image, %s doesn't look like mountpoint", img.MountPoint)
		return nil
	}

	c.config.Logger.Debugf("Unmounting image %s", img.Label)
	err := c.config.Mounter.Unmount(img.MountPoint)
	if err != nil {
		return err
	}
	_, err = c.config.Runner.Run("losetup", "-d", img.LoopDevice)
	img.LoopDevice = ""
	return err
}

// CreateFileSystemImage creates the image file for config.target
func (c Elemental) CreateFileSystemImage(img *v1.Image) error {
	c.config.Logger.Infof("Creating file system image %s", img.File)
	err := utils.MkdirAll(c.config.Fs, filepath.Dir(img.File), cnst.DirPerm)
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
		_ = c.config.Fs.RemoveAll(img.File)
		return err
	}
	err = actImg.Close()
	if err != nil {
		_ = c.config.Fs.RemoveAll(img.File)
		return err
	}

	mkfs := partitioner.NewMkfsCall(img.File, img.FS, img.Label, c.config.Runner)
	_, err = mkfs.Apply()
	if err != nil {
		_ = c.config.Fs.RemoveAll(img.File)
		return err
	}
	return nil
}

// DeployImage will deploay the given image into the target. This method
// creates the filesystem image file, mounts it and unmounts it as needed.
func (c *Elemental) DeployImage(img *v1.Image, leaveMounted bool) error {
	var err error

	if !img.Source.IsFile() {
		//TODO add support for squashfs images
		err = c.CreateFileSystemImage(img)
		if err != nil {
			return err
		}

		err = c.MountImage(img, "rw")
		if err != nil {
			return err
		}
	}
	err = c.CopyImage(img)
	if err != nil {
		_ = c.UnmountImage(img)
		return err
	}
	if leaveMounted && img.Source.IsFile() {
		err = c.MountImage(img, "rw")
		if err != nil {
			return err
		}
	}
	if !leaveMounted {
		err = c.UnmountImage(img)
		if err != nil {
			return err
		}
	}
	return nil
}

// CopyImage sets the image data according to the image source type
func (c *Elemental) CopyImage(img *v1.Image) error { // nolint:gocyclo
	c.config.Logger.Infof("Copying %s image...", img.Label)
	var err error

	if img.Source.IsDocker() {
		if c.config.Cosign {
			c.config.Logger.Infof("Running cosing verification for %s", img.Source.Value())
			out, err := utils.CosignVerify(
				c.config.Fs, c.config.Runner, img.Source.Value(),
				c.config.CosignPubKey, v1.IsDebugLevel(c.config.Logger),
			)
			if err != nil {
				c.config.Logger.Errorf("Cosign verification failed: %s", out)
				return err
			}
		}
		err = c.config.Luet.Unpack(img.MountPoint, img.Source.Value(), false)
		if err != nil {
			return err
		}
	} else if img.Source.IsDir() {
		excludes := []string{"mnt", "proc", "sys", "dev", "tmp", "host", "run"}
		err = utils.SyncData(c.config.Fs, img.Source.Value(), img.MountPoint, excludes...)
		if err != nil {
			return err
		}
	} else if img.Source.IsChannel() {
		err = c.config.Luet.UnpackFromChannel(img.MountPoint, img.Source.Value())
		if err != nil {
			return err
		}
	}

	if img.Source.IsFile() {
		err := utils.MkdirAll(c.config.Fs, filepath.Dir(img.File), cnst.DirPerm)
		if err != nil {
			return err
		}
		err = utils.CopyFile(c.config.Fs, img.Source.Value(), img.File)
		if err != nil {
			return err
		}
		if img.Label != "" && img.FS != cnst.SquashFs {
			_, err = c.config.Runner.Run("tune2fs", "-L", img.Label, img.File)
			if err != nil {
				c.config.Logger.Errorf("Failed to apply label %s to $s", img.Label, img.File)
				_ = c.config.Fs.Remove(img.File)
				return err
			}
		}
	} else {
		err = utils.CreateDirStructure(c.config.Fs, img.MountPoint)
		if err != nil {
			fmt.Println("failed creating dir structure")
			return err
		}
	}
	c.config.Logger.Infof("Finished copying %s...", img.Label)
	return nil
}

// CopyCloudConfig will check if there is a cloud init in the config and store it on the target
func (c *Elemental) CopyCloudConfig() (err error) {
	if c.config.CloudInit != "" {
		customConfig := filepath.Join(cnst.OEMDir, "99_custom.yaml")
		err = utils.GetSource(c.config, c.config.CloudInit, customConfig)
		if err != nil {
			return err
		}
		if err = c.config.Fs.Chmod(customConfig, cnst.FilePerm); err != nil {
			return err
		}
		c.config.Logger.Infof("Finished copying cloud config file %s to %s", c.config.CloudInit, customConfig)
	}
	return nil
}

// SelinuxRelabel will relabel the system if it finds the binary and the context
func (c *Elemental) SelinuxRelabel(rootDir string, raiseError bool) error {
	var err error

	contextFile := filepath.Join(rootDir, "/etc/selinux/targeted/contexts/files/file_contexts")

	_, err = c.config.Fs.Stat(contextFile)
	contextExists := err == nil

	if utils.CommandExists("setfiles") && contextExists {
		_, err = c.config.Runner.Run("setfiles", "-r", rootDir, contextFile, rootDir)
	}

	// In the original code this can error out and we dont really care
	// I guess that to maintain backwards compatibility we have to do the same, we dont care if it raises an error
	// but we still add the possibility to return an error if we want to change it in the future to be more strict?
	if raiseError && err != nil {
		return err
	}
	return nil
}

// CheckNoFormat will make sure that if we set the no format option, the system doesnt already contain a cos system
// by checking the active/passive labels. If they are set then we check if we have the force flag, which means that we
// don't care and proceed to overwrite
func (c *Elemental) CheckNoFormat() error {
	c.config.Logger.Infof("Checking no-format condition")
	// User asked for no format, lets check if there are already those labeled file systems in the disk
	for _, label := range []string{c.config.ActiveLabel, c.config.PassiveLabel} {
		found, _ := utils.GetDeviceByLabel(c.config.Runner, label, 1)
		if found != "" {
			if c.config.Force {
				msg := "Forcing overwrite of existing OS image due to `force` flag"
				c.config.Logger.Infof(msg)
				return nil
			}
			msg := "there is already an active deployment in the system, use '--force' flag to overwrite it"
			c.config.Logger.Error(msg)
			return errors.New(msg)
		}
	}
	return nil
}

// GetIso will try to:
// download the iso into a temporary folder, mount the iso file as loop
// in cnst.DownloadedIsoMnt and update recovery and active image sources if
// they are already configured.
func (c *Elemental) GetIso() (tmpDir string, err error) {
	//TODO support ISO download in persistent storage?
	tmpDir, err = utils.TempDir(c.config.Fs, "", "elemental")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = c.config.Fs.RemoveAll(tmpDir)
		}
	}()

	isoMnt := filepath.Join(tmpDir, "iso")
	rootfsMnt := filepath.Join(tmpDir, "rootfs")

	tmpFile := filepath.Join(tmpDir, "cOs.iso")
	err = utils.GetSource(c.config, c.config.Iso, tmpFile)
	if err != nil {
		return "", err
	}
	err = utils.MkdirAll(c.config.Fs, isoMnt, cnst.DirPerm)
	if err != nil {
		return "", err
	}
	c.config.Logger.Infof("Mounting iso %s into %s", tmpFile, isoMnt)
	err = c.config.Mounter.Mount(tmpFile, isoMnt, "auto", []string{"loop"})
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = c.config.Mounter.Unmount(isoMnt)
		}
	}()

	c.config.Logger.Infof("Mounting squashfs image from iso into %s", rootfsMnt)
	err = utils.MkdirAll(c.config.Fs, rootfsMnt, cnst.DirPerm)
	if err != nil {
		return "", err
	}
	err = c.config.Mounter.Mount(filepath.Join(isoMnt, cnst.IsoRootFile), rootfsMnt, "auto", []string{})
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = c.config.Mounter.Unmount(rootfsMnt)
		}
	}()

	activeImg := c.config.Images.GetActive()
	if activeImg != nil {
		activeImg.Source = v1.NewDirSrc(rootfsMnt)
	}
	recoveryImg := c.config.Images.GetRecovery()
	if recoveryImg != nil {
		squashedImgSource := filepath.Join(isoMnt, cnst.RecoverySquashFile)
		if exists, _ := utils.Exists(c.config.Fs, squashedImgSource); exists {
			recoveryImg.Source = v1.NewFileSrc(squashedImgSource)
			recoveryImg.FS = cnst.SquashFs
		} else if activeImg != nil {
			recoveryImg.Source = v1.NewFileSrc(activeImg.File)
			recoveryImg.FS = cnst.LinuxImgFs
			recoveryImg.Label = c.config.SystemLabel
		} else {
			return "", errors.New("Can't set recovery image from ISO, source image is missing")
		}
	}
	return tmpDir, nil
}

// Sets the default_meny_entry value in RunConfig.GrubOEMEnv file at in
// State partition mountpoint.
func (c Elemental) SetDefaultGrubEntry() error {
	var part *v1.Partition

	part = c.config.Partitions.GetByName(cnst.StatePartName)
	if part == nil {
		// Try to fall back to get it via StateLabel
		p, err := utils.GetFullDeviceByLabel(c.config.Runner, c.config.StateLabel, 5)
		if err != nil {
			return errors.New("state partition not found. Cannot set grub env file")
		} else if p.MountPoint == "" {
			return errors.New("state partition not mounted. Cannot set grub env file")
		} else {
			part = p
		}
	}
	grub := utils.NewGrub(c.config)
	return grub.SetPersistentVariables(
		filepath.Join(part.MountPoint, cnst.GrubOEMEnv),
		map[string]string{"default_menu_entry": c.config.GrubDefEntry},
	)
}

// Runs rebranding procedure. Note this assumes all required partitions and
// images to be mounted in advance.
func (c Elemental) Rebrand() error {
	return c.SetDefaultGrubEntry()
}
