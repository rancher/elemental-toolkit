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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/partitioner"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
)

// BuildISORun will install the system from a given configuration
func BuildISORun(cfg *v1.BuildConfig) (err error) {
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	isoTmpDir, err := utils.TempDir(cfg.Fs, "", "elemental-iso")
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return cfg.Fs.RemoveAll(isoTmpDir) })

	rootDir := filepath.Join(isoTmpDir, "rootfs")
	err = utils.MkdirAll(cfg.Fs, rootDir, constants.DirPerm)
	if err != nil {
		return err
	}

	uefiDir := filepath.Join(isoTmpDir, "uefi")
	err = utils.MkdirAll(cfg.Fs, uefiDir, constants.DirPerm)
	if err != nil {
		return err
	}

	isoDir := filepath.Join(isoTmpDir, "iso")
	err = utils.MkdirAll(cfg.Fs, isoDir, constants.DirPerm)
	if err != nil {
		return err
	}

	if cfg.OutDir != "" {
		err = utils.MkdirAll(cfg.Fs, cfg.OutDir, constants.DirPerm)
		if err != nil {
			cfg.Logger.Errorf("Failed creating output folder: %s", cfg.OutDir)
			return err
		}
	}

	cfg.Logger.Infof("Preparing squashfs root...")
	err = applySources(cfg.Config, rootDir, cfg.ISO.RootFS...)
	if err != nil {
		cfg.Logger.Errorf("Failed installing OS packages: %v", err)
		return err
	}
	err = utils.CreateDirStructure(cfg.Fs, rootDir)
	if err != nil {
		cfg.Logger.Errorf("Failed creating root directory structure: %v", err)
		return err
	}

	cfg.Logger.Infof("Preparing EFI image...")
	err = applySources(cfg.Config, uefiDir, cfg.ISO.UEFI...)
	if err != nil {
		cfg.Logger.Errorf("Failed installing EFI packages: %v", err)
		return err
	}

	cfg.Logger.Infof("Preparing ISO image root tree...")
	err = applySources(cfg.Config, isoDir, cfg.ISO.Image...)
	if err != nil {
		cfg.Logger.Errorf("Failed installing ISO image packages: %v", err)
		return err
	}

	err = prepareISORoot(cfg.Config, isoDir, rootDir, uefiDir) // mksquashfs, mkfs.fat copy kernel and initrd
	if err != nil {
		cfg.Logger.Errorf("Failed preparing ISO's root tree: %v", err)
		return err
	}

	cfg.Logger.Infof("Creating ISO image...")
	err = burnISO(cfg, isoDir)
	if err != nil {
		cfg.Logger.Errorf("Failed preparing ISO's root tree: %v", err)
		return err
	}

	return err
}

func prepareISORoot(c v1.Config, isoDir string, rootDir string, uefiDir string) error {
	kernel, initrd, err := findKernelInitrd(c, rootDir)
	if err != nil {
		c.Logger.Error("Could not find kernel and/or initrd")
		return err
	}
	err = utils.MkdirAll(c.Fs, filepath.Join(isoDir, "boot"), constants.DirPerm)
	if err != nil {
		return err
	}
	//TODO document boot/kernel and boot/initrd expectation in bootloader config
	c.Logger.Debugf("Copying Kernel file %s to iso root tree", kernel)
	err = utils.CopyFile(c.Fs, kernel, filepath.Join(isoDir, constants.IsoKernelPath))
	if err != nil {
		return err
	}

	c.Logger.Debugf("Copying initrd file %s to iso root tree", initrd)
	err = utils.CopyFile(c.Fs, initrd, filepath.Join(isoDir, constants.IsoInitrdPath))
	if err != nil {
		return err
	}

	c.Logger.Info("Creating squashfs...")
	squashOptions := constants.GetDefaultSquashfsOptions()
	if len(c.SquashFsCompressionConfig) > 0 {
		squashOptions = append(squashOptions, c.SquashFsCompressionConfig...)
	} else {
		squashOptions = append(squashOptions, constants.GetDefaultSquashfsCompressionOptions()...)
	}
	err = utils.CreateSquashFS(c.Runner, c.Logger, rootDir, filepath.Join(isoDir, constants.IsoRootFile), squashOptions)
	if err != nil {
		return err
	}

	c.Logger.Info("Creating EFI image...")
	err = createEFI(c, uefiDir, filepath.Join(isoDir, constants.IsoEFIPath))
	if err != nil {
		return err
	}
	return nil
}

func findFileWithPrefix(fs v1.FS, path string, prefixes ...string) (string, error) {
	files, err := fs.ReadDir(path)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		for _, p := range prefixes {
			if !f.IsDir() && strings.HasPrefix(f.Name(), p) {
				if f.Mode()&os.ModeSymlink == os.ModeSymlink {
					found, err := fs.Readlink(filepath.Join(path, f.Name()))
					if err == nil {
						found = filepath.Join(path, found)
						if exists, _ := utils.Exists(fs, found); exists {
							return found, nil
						}
					}
				} else {
					return filepath.Join(path, f.Name()), nil
				}
			}
		}
	}
	return "", fmt.Errorf("No file found with prefixes: %v", prefixes)
}

func findKernelInitrd(c v1.Config, rootDir string) (kernel string, initrd string, err error) {
	kernelNames := []string{"uImage", "Image", "zImage", "vmlinuz", "image"}
	initrdNames := []string{"initrd", "initramfs"}
	kernel, err = findFileWithPrefix(c.Fs, filepath.Join(rootDir, "boot"), kernelNames...)
	if err != nil {
		c.Logger.Errorf("No Kernel file found")
		return "", "", err
	}
	initrd, err = findFileWithPrefix(c.Fs, filepath.Join(rootDir, "boot"), initrdNames...)
	if err != nil {
		c.Logger.Errorf("No initrd file found")
		return "", "", err
	}
	return kernel, initrd, nil
}

func createEFI(c v1.Config, root string, img string) error {
	efiSize, err := utils.DirSize(c.Fs, root)
	if err != nil {
		return err
	}

	// align efiSize to the next 4MB slot
	align := int64(4 * 1024 * 1024)
	efiSize = efiSize/align*align + align

	// TODO this is a copy of Elemental's CreateFilesystemImage, Elemental should not be tied to RunConfig
	err = utils.MkdirAll(c.Fs, filepath.Dir(img), constants.DirPerm)
	if err != nil {
		return err
	}
	actImg, err := c.Fs.Create(img)
	if err != nil {
		return err
	}

	err = actImg.Truncate(efiSize)
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

	mkfs := partitioner.NewMkfsCall(img, constants.EfiFs, constants.EfiLabel, c.Runner)
	_, err = mkfs.Apply()
	if err != nil {
		_ = c.Fs.RemoveAll(img)
		return err
	}
	// End of CreateFileSystemImage copy

	files, err := c.Fs.ReadDir(root)
	if err != nil {
		return err
	}

	for _, f := range files {
		_, err = c.Runner.Run("mcopy", "-s", "-i", img, filepath.Join(root, f.Name()), "::")
		if err != nil {
			return err
		}
	}

	return nil
}

func burnISO(c *v1.BuildConfig, root string) error {
	cmd := "xorriso"
	var outputFile string

	if c.Date {
		currTime := time.Now()
		outputFile = fmt.Sprintf("%s.%s.iso", c.Name, currTime.Format("20060102"))
	} else {
		outputFile = fmt.Sprintf("%s.iso", c.Name)
	}

	if c.OutDir != "" {
		outputFile = filepath.Join(c.OutDir, outputFile)
	}

	if exists, _ := utils.Exists(c.Fs, outputFile); exists {
		c.Logger.Warnf("Overwriting already existing %s", outputFile)
		err := c.Fs.Remove(outputFile)
		if err != nil {
			return err
		}
	}

	args := []string{
		"-volid", c.ISO.Label, "-joliet", "on", "-padding", "0",
		"-outdev", outputFile, "-map", root, "/", "-chmod", "0755", "--",
	}
	args = append(args, constants.GetDefaultXorrisoBooloaderArgs(root, c.ISO.BootFile, c.ISO.BootCatalog, c.ISO.HybridMBR)...)

	out, err := c.Runner.Run(cmd, args...)
	c.Logger.Debugf("Xorriso: %s", string(out))
	if err != nil {
		return err
	}
	return nil
}

func applySources(c v1.Config, target string, sources ...string) error {
	var err error
	for _, src := range sources {
		err = applySource(c, target, utils.NewSrcGuessingType(c, src))
		if err != nil {
			return err
		}
	}
	return nil
}

// TODO this method should be part of Elemental (almost the same code as in CopyImage) however Elemental
// is tied to RunConfig while it shouldn't, related to issue #33
func applySource(c v1.Config, target string, src v1.ImageSource) error {
	c.Logger.Debugf("Applying source %s to target %s", src.Value(), target)
	if src.IsDocker() {
		if c.Cosign {
			c.Logger.Infof("Running cosing verification for %s", src.Value())
			out, err := utils.CosignVerify(
				c.Fs, c.Runner, src.Value(),
				c.CosignPubKey, v1.IsDebugLevel(c.Logger),
			)
			if err != nil {
				c.Logger.Errorf("Cosign verification failed: %s", out)
				return err
			}
		}
		err := c.Luet.Unpack(target, src.Value(), c.LocalImage)
		if err != nil {
			return err
		}
	} else if src.IsDir() {
		excludes := []string{"/mnt", "/proc", "/sys", "/dev", "/tmp", "/host", "/run"}
		err := utils.SyncData(c.Fs, src.Value(), target, excludes...)
		if err != nil {
			return err
		}
	} else if src.IsChannel() {
		err := c.Luet.UnpackFromChannel(target, src.Value(), c.Repos...)
		if err != nil {
			return err
		}
	} else if src.IsFile() {
		err := utils.MkdirAll(c.Fs, filepath.Dir(target), constants.DirPerm)
		if err != nil {
			return err
		}
		err = utils.CopyFile(c.Fs, src.Value(), target)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Unknown image source type for %s", src.Value())
	}
	return nil
}
