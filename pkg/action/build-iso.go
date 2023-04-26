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
	"fmt"
	"path/filepath"
	"time"

	"github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/elemental"
	elementalError "github.com/rancher/elemental-cli/pkg/error"
	"github.com/rancher/elemental-cli/pkg/live"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

type LiveBootloader interface {
	PrepareEFI(rootDir, uefiDir string) error
	PrepareISO(rootDir, isoDir string) error
}

type BuildISOAction struct {
	liveBoot LiveBootloader
	cfg      *v1.BuildConfig
	spec     *v1.LiveISO
	e        *elemental.Elemental
}

type BuildISOActionOption func(a *BuildISOAction)

func WithLiveBoot(l LiveBootloader) BuildISOActionOption {
	return func(a *BuildISOAction) {
		a.liveBoot = l
	}
}

func NewBuildISOAction(cfg *v1.BuildConfig, spec *v1.LiveISO, opts ...BuildISOActionOption) *BuildISOAction {
	b := &BuildISOAction{
		cfg:      cfg,
		e:        elemental.NewElemental(&cfg.Config),
		spec:     spec,
		liveBoot: live.NewGreenLiveBootLoader(cfg, spec),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// BuildISORun will install the system from a given configuration
func (b *BuildISOAction) ISORun() error {
	cleanup := utils.NewCleanStack()
	var err error
	defer func() { err = cleanup.Cleanup(err) }()

	isoTmpDir, err := utils.TempDir(b.cfg.Fs, "", "elemental-iso")
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateTempDir)
	}
	cleanup.Push(func() error { return b.cfg.Fs.RemoveAll(isoTmpDir) })

	rootDir := filepath.Join(isoTmpDir, "rootfs")
	err = utils.MkdirAll(b.cfg.Fs, rootDir, constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating rootfs dir: %s", rootDir)
		return elementalError.NewFromError(err, elementalError.CreateDir)
	}

	uefiDir := filepath.Join(isoTmpDir, "uefi")
	err = utils.MkdirAll(b.cfg.Fs, uefiDir, constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating uefi dir: %s", uefiDir)
		return elementalError.NewFromError(err, elementalError.CreateDir)
	}

	isoDir := filepath.Join(isoTmpDir, "iso")
	err = utils.MkdirAll(b.cfg.Fs, isoDir, constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating iso dir: %s", isoDir)
		return elementalError.NewFromError(err, elementalError.CreateDir)
	}

	if b.cfg.OutDir != "" {
		err = utils.MkdirAll(b.cfg.Fs, b.cfg.OutDir, constants.DirPerm)
		if err != nil {
			b.cfg.Logger.Errorf("Failed creating output dir: %s", b.cfg.OutDir)
			return elementalError.NewFromError(err, elementalError.CreateDir)
		}
	}

	b.cfg.Logger.Infof("Preparing squashfs root (%v source)...", len(b.spec.RootFS))
	err = b.applySources(rootDir, b.spec.RootFS...)
	if err != nil {
		b.cfg.Logger.Errorf("Failed installing OS packages: %v", err)
		return err
	}
	err = utils.CreateDirStructure(b.cfg.Fs, rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating root directory structure: %v", err)
		return elementalError.NewFromError(err, elementalError.CreateDir)
	}

	if b.spec.Firmware == v1.EFI {
		b.cfg.Logger.Infof("Preparing EFI image...")
		if b.spec.BootloaderInRootFs {
			err = b.liveBoot.PrepareEFI(rootDir, uefiDir)
			if err != nil {
				b.cfg.Logger.Errorf("Failed fetching EFI data: %v", err)
				return elementalError.NewFromError(err, elementalError.CopyData)
			}
		}
		err = b.applySources(uefiDir, b.spec.UEFI...)
		if err != nil {
			b.cfg.Logger.Errorf("Failed installing EFI packages: %v", err)
			return err
		}
	}

	b.cfg.Logger.Infof("Preparing ISO image root tree...")
	if b.spec.BootloaderInRootFs {
		err = b.liveBoot.PrepareISO(rootDir, isoDir)
		if err != nil {
			b.cfg.Logger.Errorf("Failed fetching bootloader binaries: %v", err)
			return elementalError.NewFromError(err, elementalError.CreateFile)
		}
	}
	err = b.applySources(isoDir, b.spec.Image...)
	if err != nil {
		b.cfg.Logger.Errorf("Failed installing ISO image packages: %v", err)
		return err
	}

	err = b.prepareISORoot(isoDir, rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("Failed preparing ISO's root tree: %v", err)
		return err
	}

	if b.spec.Firmware == v1.EFI {
		b.cfg.Logger.Info("Creating EFI image...")
		err = b.createEFI(uefiDir, filepath.Join(isoTmpDir, constants.ISOEFIImg))
		if err != nil {
			return err
		}
	}

	b.cfg.Logger.Infof("Creating ISO image...")
	err = b.burnISO(isoDir, filepath.Join(isoTmpDir, constants.ISOEFIImg))
	if err != nil {
		b.cfg.Logger.Errorf("Failed burning ISO file: %v", err)
		return err
	}

	return err
}

func (b BuildISOAction) prepareISORoot(isoDir string, rootDir string) error {
	kernel, initrd, err := b.e.FindKernelInitrd(rootDir)
	if err != nil {
		b.cfg.Logger.Error("Could not find kernel and/or initrd")
		return elementalError.NewFromError(err, elementalError.StatFile)
	}
	err = utils.MkdirAll(b.cfg.Fs, filepath.Join(isoDir, "boot"), constants.DirPerm)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateDir)
	}
	//TODO document boot/kernel and boot/initrd expectation in bootloader config
	b.cfg.Logger.Debugf("Copying Kernel file %s to iso root tree", kernel)
	err = utils.CopyFile(b.cfg.Fs, kernel, filepath.Join(isoDir, constants.ISOKernelPath))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}

	b.cfg.Logger.Debugf("Copying initrd file %s to iso root tree", initrd)
	err = utils.CopyFile(b.cfg.Fs, initrd, filepath.Join(isoDir, constants.ISOInitrdPath))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}

	b.cfg.Logger.Info("Creating squashfs...")
	squashOptions := append(constants.GetDefaultSquashfsOptions(), b.cfg.SquashFsCompressionConfig...)
	err = utils.CreateSquashFS(b.cfg.Runner, b.cfg.Logger, rootDir, filepath.Join(isoDir, constants.ISORootFile), squashOptions)
	return elementalError.NewFromError(err, elementalError.MKFSCall)
}

func (b BuildISOAction) createEFI(root string, img string) error {
	efiSize, err := utils.DirSize(b.cfg.Fs, root)
	if err != nil {
		return err
	}

	// align efiSize to the next 4MB slot
	align := int64(4 * 1024 * 1024)
	efiSizeMB := (efiSize/align*align + align) / (1024 * 1024)

	err = b.e.CreateFileSystemImage(&v1.Image{
		File:  img,
		Size:  uint(efiSizeMB),
		FS:    constants.EfiFs,
		Label: constants.EfiLabel,
	})
	if err != nil {
		return err
	}

	files, err := b.cfg.Fs.ReadDir(root)
	if err != nil {
		return err
	}

	for _, f := range files {
		_, err = b.cfg.Runner.Run("mcopy", "-s", "-i", img, filepath.Join(root, f.Name()), "::")
		if err != nil {
			return err
		}
	}

	return nil
}

func (b BuildISOAction) burnISO(root, efiImg string) error {
	cmd := "xorriso"
	var outputFile string
	var isoFileName string

	if b.cfg.Date {
		currTime := time.Now()
		isoFileName = fmt.Sprintf("%s.%s.iso", b.cfg.Name, currTime.Format("20060102"))
	} else {
		isoFileName = fmt.Sprintf("%s.iso", b.cfg.Name)
	}

	outputFile = isoFileName
	if b.cfg.OutDir != "" {
		outputFile = filepath.Join(b.cfg.OutDir, outputFile)
	}

	if exists, _ := utils.Exists(b.cfg.Fs, outputFile); exists {
		b.cfg.Logger.Warnf("Overwriting already existing %s", outputFile)
		err := b.cfg.Fs.Remove(outputFile)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.RemoveFile)
		}
	}

	args := []string{
		"-volid", b.spec.Label /*"-joliet", "on"*/, "-padding", "0",
		"-outdev", outputFile, "-map", root, "/", "-chmod", "0755", "--",
	}
	args = append(args, live.XorrisoBooloaderArgs(root, efiImg, b.spec.Firmware)...)

	out, err := b.cfg.Runner.Run(cmd, args...)
	b.cfg.Logger.Debugf("Xorriso: %s", string(out))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CommandRun)
	}

	checksum, err := utils.CalcFileChecksum(b.cfg.Fs, outputFile)
	if err != nil {
		b.cfg.Logger.Errorf("checksum computation failed: %v", err)
		return elementalError.NewFromError(err, elementalError.CalculateChecksum)
	}
	err = b.cfg.Fs.WriteFile(fmt.Sprintf("%s.sha256", outputFile), []byte(fmt.Sprintf("%s %s\n", checksum, isoFileName)), 0644)
	if err != nil {
		b.cfg.Logger.Errorf("cannot write checksum file: %v", err)
		return elementalError.NewFromError(err, elementalError.CreateFile)
	}

	return nil
}

func (b BuildISOAction) applySources(target string, sources ...*v1.ImageSource) error {
	for _, src := range sources {
		_, err := b.e.DumpSource(target, src)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.DumpSource)
		}
	}
	return nil
}
