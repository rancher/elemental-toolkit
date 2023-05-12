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

package elemental

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	cnst "github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/partitioner"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

// Elemental is the struct meant to self-contain most utils and actions related to Elemental, like installing or applying selinux
type Elemental struct {
	config *v1.Config
}

func NewElemental(config *v1.Config) *Elemental {
	return &Elemental{
		config: config,
	}
}

// FormatPartition will format an already existing partition
func (e *Elemental) FormatPartition(part *v1.Partition, opts ...string) error {
	e.config.Logger.Infof("Formatting '%s' partition", part.Name)
	return partitioner.FormatDevice(e.config.Runner, part.Path, part.FS, part.FilesystemLabel, opts...)
}

// PartitionAndFormatDevice creates a new empty partition table on target disk
// and applies the configured disk layout by creating and formatting all
// required partitions
func (e *Elemental) PartitionAndFormatDevice(i *v1.InstallSpec) error {
	disk := partitioner.NewDisk(
		i.Target,
		partitioner.WithRunner(e.config.Runner),
		partitioner.WithFS(e.config.Fs),
		partitioner.WithLogger(e.config.Logger),
	)

	if !disk.Exists() {
		e.config.Logger.Errorf("Disk %s does not exist", i.Target)
		return fmt.Errorf("disk %s does not exist", i.Target)
	}

	e.config.Logger.Infof("Partitioning device...")
	out, err := disk.NewPartitionTable(i.PartTable)
	if err != nil {
		e.config.Logger.Errorf("Failed creating new partition table: %s", out)
		return err
	}

	parts := i.Partitions.PartitionsByInstallOrder(i.ExtraPartitions)
	return e.createPartitions(disk, parts)
}

func (e *Elemental) createAndFormatPartition(disk *partitioner.Disk, part *v1.Partition) error {
	e.config.Logger.Debugf("Adding partition %s", part.Name)
	num, err := disk.AddPartition(part.Size, part.FS, part.Name, part.Flags...)
	if err != nil {
		e.config.Logger.Errorf("Failed creating %s partition", part.Name)
		return err
	}
	partDev, err := disk.FindPartitionDevice(num)
	if err != nil {
		return err
	}
	if part.FS != "" {
		e.config.Logger.Debugf("Formatting partition with label %s", part.FilesystemLabel)
		err = partitioner.FormatDevice(e.config.Runner, partDev, part.FS, part.FilesystemLabel)
		if err != nil {
			e.config.Logger.Errorf("Failed formatting partition %s", part.Name)
			return err
		}
	} else {
		e.config.Logger.Debugf("Wipe file system on %s", part.Name)
		err = disk.WipeFsOnPartition(partDev)
		if err != nil {
			e.config.Logger.Errorf("Failed to wipe filesystem of partition %s", partDev)
			return err
		}
	}
	part.Path = partDev
	return nil
}

func (e *Elemental) createPartitions(disk *partitioner.Disk, parts v1.PartitionList) error {
	for _, part := range parts {
		err := e.createAndFormatPartition(disk, part)
		if err != nil {
			return err
		}
	}
	return nil
}

// MountPartitions mounts configured partitions. Partitions with an unset mountpoint are not mounted.
// Note umounts must be handled by caller logic.
func (e Elemental) MountPartitions(parts v1.PartitionList) error {
	e.config.Logger.Infof("Mounting disk partitions")
	var err error

	for _, part := range parts {
		if part.MountPoint != "" {
			err = e.MountPartition(part, "rw")
			if err != nil {
				_ = e.UnmountPartitions(parts)
				return err
			}
		}
	}

	return err
}

// UnmountPartitions unmounts configured partitiosn. Partitions with an unset mountpoint are not unmounted.
func (e Elemental) UnmountPartitions(parts v1.PartitionList) error {
	e.config.Logger.Infof("Unmounting disk partitions")
	var err error
	errMsg := ""
	failure := false

	// If there is an early error we still try to unmount other partitions
	for _, part := range parts {
		if part.MountPoint != "" {
			err = e.UnmountPartition(part)
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

// MountRWPartition mounts, or remounts if needed, a partition with RW permissions
func (e Elemental) MountRWPartition(part *v1.Partition) (umount func() error, err error) {
	if mnt, _ := utils.IsMounted(e.config, part); mnt {
		err = e.MountPartition(part, "remount", "rw")
		if err != nil {
			e.config.Logger.Errorf("failed mounting %s partition: %v", part.Name, err)
			return nil, err
		}
		umount = func() error { return e.MountPartition(part, "remount", "ro") }
	} else {
		err = e.MountPartition(part, "rw")
		if err != nil {
			e.config.Logger.Error("failed mounting %s partition: %v", part.Name, err)
			return nil, err
		}
		umount = func() error { return e.UnmountPartition(part) }
	}
	return umount, nil
}

// MountPartition mounts a partition with the given mount options
func (e Elemental) MountPartition(part *v1.Partition, opts ...string) error {
	e.config.Logger.Debugf("Mounting partition %s", part.FilesystemLabel)
	err := utils.MkdirAll(e.config.Fs, part.MountPoint, cnst.DirPerm)
	if err != nil {
		return err
	}
	if part.Path == "" {
		// Lets error out only after 10 attempts to find the device
		device, err := utils.GetDeviceByLabel(e.config.Runner, part.FilesystemLabel, 10)
		if err != nil {
			e.config.Logger.Errorf("Could not find a device with label %s", part.FilesystemLabel)
			return err
		}
		part.Path = device
	}
	err = e.config.Mounter.Mount(part.Path, part.MountPoint, "auto", opts)
	if err != nil {
		e.config.Logger.Errorf("Failed mounting device %s with label %s", part.Path, part.FilesystemLabel)
		return err
	}
	return nil
}

// UnmountPartition unmounts the given partition or does nothing if not mounted
func (e Elemental) UnmountPartition(part *v1.Partition) error {
	if mnt, _ := utils.IsMounted(e.config, part); !mnt {
		e.config.Logger.Debugf("Not unmounting partition, %s doesn't look like mountpoint", part.MountPoint)
		return nil
	}
	e.config.Logger.Debugf("Unmounting partition %s", part.FilesystemLabel)
	return e.config.Mounter.Unmount(part.MountPoint)
}

// MountImage mounts an image with the given mount options
func (e Elemental) MountImage(img *v1.Image, opts ...string) error {
	e.config.Logger.Debugf("Mounting image %s", img.Label)
	err := utils.MkdirAll(e.config.Fs, img.MountPoint, cnst.DirPerm)
	if err != nil {
		return err
	}
	out, err := e.config.Runner.Run("losetup", "--show", "-f", img.File)
	if err != nil {
		return err
	}
	loop := strings.TrimSpace(string(out))
	err = e.config.Mounter.Mount(loop, img.MountPoint, "auto", opts)
	if err != nil {
		_, _ = e.config.Runner.Run("losetup", "-d", loop)
		return err
	}
	img.LoopDevice = loop
	return nil
}

// UnmountImage unmounts the given image or does nothing if not mounted
func (e Elemental) UnmountImage(img *v1.Image) error {
	// Using IsLikelyNotMountPoint seams to be safe as we are not checking
	// for bind mounts here
	if notMnt, _ := e.config.Mounter.IsLikelyNotMountPoint(img.MountPoint); notMnt {
		e.config.Logger.Debugf("Not unmounting image, %s doesn't look like mountpoint", img.MountPoint)
		return nil
	}

	e.config.Logger.Debugf("Unmounting image %s", img.Label)
	err := e.config.Mounter.Unmount(img.MountPoint)
	if err != nil {
		return err
	}
	_, err = e.config.Runner.Run("losetup", "-d", img.LoopDevice)
	img.LoopDevice = ""
	return err
}

// CreateFileSystemImage creates the image file for the given image
func (e Elemental) CreateFileSystemImage(img *v1.Image) error {
	e.config.Logger.Infof("Creating file system image %s", img.File)
	err := utils.MkdirAll(e.config.Fs, filepath.Dir(img.File), cnst.DirPerm)
	if err != nil {
		return err
	}
	actImg, err := e.config.Fs.Create(img.File)
	if err != nil {
		return err
	}

	err = actImg.Truncate(int64(img.Size * 1024 * 1024))
	if err != nil {
		actImg.Close()
		_ = e.config.Fs.RemoveAll(img.File)
		return err
	}
	err = actImg.Close()
	if err != nil {
		_ = e.config.Fs.RemoveAll(img.File)
		return err
	}

	mkfs := partitioner.NewMkfsCall(img.File, img.FS, img.Label, e.config.Runner)
	_, err = mkfs.Apply()
	if err != nil {
		_ = e.config.Fs.RemoveAll(img.File)
		return err
	}
	return nil
}

// DeployImgTree will deploy the given image into the given root tree. Returns source metadata in info,
// a tree cleaner function and error. The given root will be a bind mount of a temporary directory into the same
// filesystem of img.File, this is helpful to make the deployment easily accessible in after-* hooks.
func (e *Elemental) DeployImgTree(img *v1.Image, root string) (info interface{}, cleaner func() error, err error) {
	// We prepare the rootTree next to the target image file, in the same base path
	e.config.Logger.Infof("Preparing root tree for image: %s", img.File)
	tmp := strings.TrimSuffix(img.File, filepath.Ext(img.File))
	tmp += ".imgTree"
	err = utils.MkdirAll(e.config.Fs, tmp, cnst.DirPerm)
	if err != nil {
		return nil, nil, err
	}

	err = utils.MkdirAll(e.config.Fs, root, cnst.DirPerm)
	if err != nil {
		_ = e.config.Fs.RemoveAll(tmp)
		return nil, nil, err
	}
	err = e.config.Mounter.Mount(tmp, root, "bind", []string{"bind"})
	if err != nil {
		_ = e.config.Fs.RemoveAll(tmp)
		_ = e.config.Fs.RemoveAll(root)
		return nil, nil, err
	}

	cleaner = func() error {
		_ = e.config.Mounter.Unmount(root)
		err := e.config.Fs.RemoveAll(root)
		if err != nil {
			return err
		}
		return e.config.Fs.RemoveAll(tmp)
	}

	info, err = e.DumpSource(root, img.Source)
	if err != nil {
		_ = cleaner()
		return nil, nil, err
	}
	err = utils.CreateDirStructure(e.config.Fs, root)
	if err != nil {
		_ = cleaner()
		return nil, nil, err
	}

	return info, cleaner, err
}

// CreateImgFromTree creates the given image from with the contents of the tree for the given root.
func (e *Elemental) CreateImgFromTree(root string, img *v1.Image, cleaner func() error) (err error) {
	if cleaner != nil {
		defer func() {
			cErr := cleaner()
			if cErr != nil && err == nil {
				err = cErr
			}
		}()
	}

	if img.FS == cnst.SquashFs {
		e.config.Logger.Infof("Creating squashed image: %s", img.File)
		squashOptions := append(cnst.GetDefaultSquashfsOptions(), e.config.SquashFsCompressionConfig...)
		err = utils.CreateSquashFS(e.config.Runner, e.config.Logger, root, img.File, squashOptions)
		if err != nil {
			return err
		}
	} else {
		e.config.Logger.Infof("Creating filesystem image: %s", img.File)
		if img.Size == 0 {
			size, err := utils.DirSizeMB(e.config.Fs, root)
			if err != nil {
				return err
			}
			img.Size = size + cnst.ImgOverhead
		}
		err = e.CreateFileSystemImage(img)
		if err != nil {
			return err
		}
		err = e.MountImage(img, "rw")
		if err != nil {
			return err
		}
		defer func() {
			mErr := e.UnmountImage(img)
			if err == nil && mErr != nil {
				err = mErr
			}
		}()
		err = utils.SyncData(e.config.Logger, e.config.Fs, root, img.MountPoint)
		if err != nil {
			return err
		}
	}
	return err
}

// CopyFileImg copies the files target as the source of this image. It also applies the img label over the copied image.
func (e *Elemental) CopyFileImg(img *v1.Image) error {
	if !img.Source.IsFile() {
		return fmt.Errorf("Copying a file image requires an image source of file type")
	}

	err := utils.MkdirAll(e.config.Fs, filepath.Dir(img.File), cnst.DirPerm)
	if err != nil {
		return err
	}

	e.config.Logger.Infof("Copying image %s to %s", img.Source.Value(), img.File)
	err = utils.CopyFile(e.config.Fs, img.Source.Value(), img.File)
	if err != nil {
		return err
	}

	if img.FS != cnst.SquashFs && img.Label != "" {
		e.config.Logger.Infof("Setting label: %s ", img.Label)
		_, err = e.config.Runner.Run("tune2fs", "-L", img.Label, img.File)
	}
	return err
}

// DeployImage will deploy the given image into the target. This method
// creates the filesystem image file and fills it with the correspondant data
func (e *Elemental) DeployImage(img *v1.Image) (interface{}, error) {
	e.config.Logger.Infof("Deploying image: %s", img.File)
	info, cleaner, err := e.DeployImgTree(img, cnst.WorkingImgDir)
	if err != nil {
		return nil, err
	}

	err = e.CreateImgFromTree(cnst.WorkingImgDir, img, cleaner)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// DumpSource sets the image data according to the image source type
func (e *Elemental) DumpSource(target string, imgSrc *v1.ImageSource) (info interface{}, err error) { // nolint:gocyclo
	e.config.Logger.Infof("Copying %s source...", imgSrc.Value())

	if imgSrc.IsImage() {
		if e.config.Cosign {
			e.config.Logger.Infof("Running cosing verification for %s", imgSrc.Value())
			out, err := utils.CosignVerify(
				e.config.Fs, e.config.Runner, imgSrc.Value(),
				e.config.CosignPubKey, v1.IsDebugLevel(e.config.Logger),
			)
			if err != nil {
				e.config.Logger.Errorf("Cosign verification failed: %s", out)
				return nil, err
			}
		}

		err = e.config.ImageExtractor.ExtractImage(imgSrc.Value(), target, e.config.Platform.String(), e.config.LocalImage)
		if err != nil {
			return nil, err
		}
	} else if imgSrc.IsDir() {
		excludes := []string{"/mnt", "/proc", "/sys", "/dev", "/tmp", "/host", "/run"}
		err = utils.SyncData(e.config.Logger, e.config.Fs, imgSrc.Value(), target, excludes...)
		if err != nil {
			return nil, err
		}
	} else if imgSrc.IsFile() {
		err = utils.MkdirAll(e.config.Fs, cnst.ImgSrcDir, cnst.DirPerm)
		if err != nil {
			return nil, err
		}
		img := &v1.Image{File: imgSrc.Value(), MountPoint: cnst.ImgSrcDir}
		err = e.MountImage(img, "auto", "ro")
		if err != nil {
			return nil, err
		}
		defer e.UnmountImage(img) // nolint:errcheck
		excludes := []string{"/mnt", "/proc", "/sys", "/dev", "/tmp", "/host", "/run"}
		err = utils.SyncData(e.config.Logger, e.config.Fs, cnst.ImgSrcDir, target, excludes...)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unknown image source type")
	}
	e.config.Logger.Infof("Finished copying %s into %s", imgSrc.Value(), target)
	return info, nil
}

// CopyCloudConfig will check if there is a cloud init in the config and store it on the target
func (e *Elemental) CopyCloudConfig(cloudInit []string) (err error) {
	for i, ci := range cloudInit {
		customConfig := filepath.Join(cnst.OEMDir, fmt.Sprintf("9%d_custom.yaml", i))
		err = utils.GetSource(e.config, ci, customConfig)
		if err != nil {
			return err
		}
		if err = e.config.Fs.Chmod(customConfig, cnst.FilePerm); err != nil {
			return err
		}
		e.config.Logger.Infof("Finished copying cloud config file %s to %s", cloudInit, customConfig)
	}
	return nil
}

// SelinuxRelabel will relabel the system if it finds the binary and the context
func (e *Elemental) SelinuxRelabel(rootDir string, raiseError bool) error {
	policyFile, err := utils.FindFileWithPrefix(e.config.Fs, filepath.Join(rootDir, cnst.SELinuxTargetedPolicyPath), "policy.")
	contextFile := filepath.Join(rootDir, cnst.SELinuxTargetedContextFile)
	contextExists, _ := utils.Exists(e.config.Fs, contextFile)

	if err == nil && contextExists && e.config.Runner.CommandExists("setfiles") {
		var out []byte
		var err error
		if rootDir == "/" || rootDir == "" {
			out, err = e.config.Runner.Run("setfiles", "-c", policyFile, "-e", "/dev", "-e", "/proc", "-e", "/sys", "-F", contextFile, "/")
		} else {
			out, err = e.config.Runner.Run("setfiles", "-c", policyFile, "-F", "-r", rootDir, contextFile, rootDir)
		}
		e.config.Logger.Debugf("SELinux setfiles output: %s", string(out))
		if err != nil && raiseError {
			return err
		}
	} else {
		e.config.Logger.Debugf("No files relabelling as SELinux utilities are not found")
	}

	return nil
}

// CheckActiveDeployment returns true if at least one of the provided filesystem labels is found within the system
func (e *Elemental) CheckActiveDeployment(labels []string) bool {
	e.config.Logger.Infof("Checking for active deployment")

	for _, label := range labels {
		found, _ := utils.GetDeviceByLabel(e.config.Runner, label, 1)
		if found != "" {
			e.config.Logger.Debug("there is already an active deployment in the system")
			return true
		}
	}
	return false
}

// UpdateSourceISO downloads an ISO in a temporary folder, mounts it and updates active image to use the ISO squashfs image as
// source. Returns a cleaner method to unmount and remove the temporary folder afterwards.
func (e Elemental) UpdateSourceFormISO(iso string, activeImg *v1.Image) (func() error, error) {
	nilErr := func() error { return nil }

	tmpDir, err := utils.TempDir(e.config.Fs, "", "elemental")
	if err != nil {
		return nilErr, err
	}

	cleanTmpDir := func() error { return e.config.Fs.RemoveAll(tmpDir) }

	tmpFile := filepath.Join(tmpDir, "elemental.iso")
	err = utils.GetSource(e.config, iso, tmpFile)
	if err != nil {
		return cleanTmpDir, err
	}

	isoMnt := filepath.Join(tmpDir, "iso")
	err = utils.MkdirAll(e.config.Fs, isoMnt, cnst.DirPerm)
	if err != nil {
		return cleanTmpDir, err
	}

	e.config.Logger.Infof("Mounting iso %s into %s", tmpFile, isoMnt)
	err = e.config.Mounter.Mount(tmpFile, isoMnt, "auto", []string{"loop"})
	if err != nil {
		return cleanTmpDir, err
	}

	cleanAll := func() error {
		cErr := e.config.Mounter.Unmount(isoMnt)
		if cErr != nil {
			return cErr
		}
		return cleanTmpDir()
	}

	squashfsImg := filepath.Join(isoMnt, cnst.ISORootFile)
	ok, _ := utils.Exists(e.config.Fs, squashfsImg)
	if !ok {
		return cleanAll, fmt.Errorf("squashfs image not found in ISO: %s", squashfsImg)
	}
	activeImg.Source = v1.NewFileSrc(squashfsImg)

	return cleanAll, nil
}

// SetDefaultGrubEntry Sets the default_meny_entry value in RunConfig.GrubOEMEnv file at in
// State partition mountpoint. If there is not a custom value in the os-release file, we do nothing
// As the grub config already has a sane default
func (e Elemental) SetDefaultGrubEntry(partMountPoint string, imgMountPoint string, defaultEntry string) error {
	var configEntry string
	osRelease, err := utils.LoadEnvFile(e.config.Fs, filepath.Join(imgMountPoint, "etc", "os-release"))
	e.config.Logger.Debugf("Looking for GRUB_ENTRY_NAME name in %s", filepath.Join(imgMountPoint, "etc", "os-release"))
	if err != nil {
		e.config.Logger.Warnf("Could not load os-release file: %v", err)
	} else {
		configEntry = osRelease["GRUB_ENTRY_NAME"]
		// If its not empty override the defaultEntry and set the one set on the os-release file
		if configEntry != "" {
			defaultEntry = configEntry
		}
	}

	if defaultEntry == "" {
		e.config.Logger.Warn("No default entry name for grub, not setting a name")
		return nil
	}

	e.config.Logger.Infof("Setting default grub entry to %s", defaultEntry)
	grub := utils.NewGrub(e.config)
	return grub.SetPersistentVariables(
		filepath.Join(partMountPoint, cnst.GrubOEMEnv),
		map[string]string{"default_menu_entry": defaultEntry},
	)
}

// FindKernelInitrd finds for kernel and intird files inside the /boot directory of a given
// root tree path. It assumes kernel and initrd files match certain file name prefixes.
func (e Elemental) FindKernelInitrd(rootDir string) (kernel string, initrd string, err error) {
	kernelNames := []string{"uImage", "Image", "zImage", "vmlinuz", "image"}
	initrdNames := []string{"initrd", "initramfs"}
	kernel, err = utils.FindFileWithPrefix(e.config.Fs, filepath.Join(rootDir, "boot"), kernelNames...)
	if err != nil {
		e.config.Logger.Errorf("No Kernel file found")
		return "", "", err
	}
	initrd, err = utils.FindFileWithPrefix(e.config.Fs, filepath.Join(rootDir, "boot"), initrdNames...)
	if err != nil {
		e.config.Logger.Errorf("No initrd file found")
		return "", "", err
	}
	return kernel, initrd, nil
}

// DeactivateDevice deactivates unmounted the block devices present within the system.
// Useful to deactivate LVM volumes, if any, related to the target device.
func (e Elemental) DeactivateDevices() error {
	out, err := e.config.Runner.Run(
		"blkdeactivate", "--lvmoptions", "retry,wholevg",
		"--dmoptions", "force,retry", "--errors",
	)
	e.config.Logger.Debugf("blkdeactivate command output: %s", string(out))
	return err
}
