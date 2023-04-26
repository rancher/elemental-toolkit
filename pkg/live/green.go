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

package live

import (
	"fmt"
	"path/filepath"

	"strings"

	"github.com/rancher/elemental-cli/pkg/constants"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

type GreenLiveBootLoader struct {
	buildCfg *v1.BuildConfig
	spec     *v1.LiveISO
}

func NewGreenLiveBootLoader(cfg *v1.BuildConfig, spec *v1.LiveISO) *GreenLiveBootLoader {
	return &GreenLiveBootLoader{buildCfg: cfg, spec: spec}
}

func (g *GreenLiveBootLoader) PrepareEFI(rootDir, uefiDir string) error {
	const (
		grubEfiImageX86   = "/usr/share/grub2/x86_64-efi/grub.efi"
		grubEfiImageArm64 = "/usr/share/grub2/arm64-efi/grub.efi"
		shimBasePathX86   = "/usr/share/efi/x86_64"
		shimBasePathArm64 = "/usr/share/efi/aarch64"
		shimImg           = "shim.efi"
		mokManager        = "MokManager.efi"
	)

	err := utils.MkdirAll(g.buildCfg.Fs, filepath.Join(uefiDir, efiBootPath), constants.DirPerm)
	if err != nil {
		return err
	}

	// _, arch, _, err := v1.ParsePlatform(g.buildCfg.Platform)
	// if err != nil {
	// 	return err
	// }

	switch g.buildCfg.Platform.Arch {
	case constants.ArchAmd64, constants.Archx86:
		err = g.copyEfiFiles(
			uefiDir,
			filepath.Join(rootDir, shimBasePathX86, shimImg),
			filepath.Join(rootDir, shimBasePathX86, mokManager),
			filepath.Join(rootDir, grubEfiImageX86),
			efiImgX86,
		)
	case constants.ArchArm64:
		err = g.copyEfiFiles(
			uefiDir,
			filepath.Join(rootDir, shimBasePathArm64, shimImg),
			filepath.Join(rootDir, shimBasePathArm64, mokManager),
			filepath.Join(rootDir, grubEfiImageArm64),
			efiImgArm64,
		)
	default:
		err = fmt.Errorf("Not supported architecture: %v", g.buildCfg.Platform.Arch)
	}
	if err != nil {
		return err
	}

	return g.buildCfg.Fs.WriteFile(filepath.Join(uefiDir, efiBootPath, grubCfg), []byte(grubEfiCfg), constants.FilePerm)
}

func (g *GreenLiveBootLoader) copyEfiFiles(uefiDir, shimImg, mokManager, grubImg, efiImg string) error {
	err := utils.CopyFile(g.buildCfg.Fs, shimImg, filepath.Join(uefiDir, efiBootPath, efiImg))
	if err != nil {
		return err
	}
	err = utils.CopyFile(g.buildCfg.Fs, grubImg, filepath.Join(uefiDir, efiBootPath))
	if err != nil {
		return err
	}
	return utils.CopyFile(g.buildCfg.Fs, mokManager, filepath.Join(uefiDir, efiBootPath))
}

func (g *GreenLiveBootLoader) PrepareISO(rootDir, imageDir string) error {
	const (
		grubBootHybridImg = "/usr/share/grub2/i386-pc/boot_hybrid.img"
		syslinuxFiles     = "/usr/share/syslinux/isolinux.bin " +
			"/usr/share/syslinux/menu.c32 " +
			"/usr/share/syslinux/chain.c32 " +
			"/usr/share/syslinux/mboot.c32"
	)

	err := utils.MkdirAll(g.buildCfg.Fs, filepath.Join(imageDir, grubPrefixDir), constants.DirPerm)
	if err != nil {
		return err
	}

	if g.spec.Firmware == v1.BIOS {
		// Create eltorito image
		eltorito, err := g.BuildEltoritoImg(rootDir)
		if err != nil {
			return err
		}

		// Create loaders folder
		loaderDir := filepath.Join(imageDir, isoLoaderPath)
		err = utils.MkdirAll(g.buildCfg.Fs, loaderDir, constants.DirPerm)
		if err != nil {
			return err
		}
		// Inlude loaders in expected paths
		loaderFiles := []string{eltorito, grubBootHybridImg}
		loaderFiles = append(loaderFiles, strings.Split(syslinuxFiles, " ")...)
		for _, f := range loaderFiles {
			err = utils.CopyFile(
				g.buildCfg.Fs,
				filepath.Join(rootDir, f),
				filepath.Join(imageDir, isoLoaderPath),
			)
			if err != nil {
				return err
			}
		}
	}

	// Write grub.cfg file
	err = g.buildCfg.Fs.WriteFile(
		filepath.Join(imageDir, grubPrefixDir, grubCfg),
		[]byte(fmt.Sprintf(grubCfgTemplate, g.spec.GrubEntry, g.spec.Label)),
		constants.FilePerm,
	)
	if err != nil {
		return err
	}

	if g.spec.Firmware == v1.EFI {
		// Include EFI contents in iso root too
		return g.PrepareEFI(rootDir, imageDir)
	}

	return nil
}

func (g *GreenLiveBootLoader) BuildEltoritoImg(rootDir string) (string, error) {
	const (
		grubBiosTarget  = "i386-pc"
		grubI386BinDir  = "/usr/share/grub2/i386-pc"
		grubBiosImg     = grubI386BinDir + "/core.img"
		grubBiosCDBoot  = grubI386BinDir + "/cdboot.img"
		grubEltoritoImg = grubI386BinDir + "/eltorito.img"
		//TODO this list could be optimized
		grubModules = "ext2 iso9660 linux echo configfile search_label search_fs_file search search_fs_uuid " +
			"ls normal gzio png fat gettext font minicmd gfxterm gfxmenu all_video xfs btrfs lvm luks " +
			"gcry_rijndael gcry_sha256 gcry_sha512 crypto cryptodisk test true loadenv part_gpt " +
			"part_msdos biosdisk vga vbe chain boot"
	)
	var args []string
	args = append(args, "-O", grubBiosTarget)
	args = append(args, "-o", grubBiosImg)
	args = append(args, "-p", grubPrefixDir)
	args = append(args, "-d", grubI386BinDir)
	args = append(args, strings.Split(grubModules, " ")...)

	chRoot := utils.NewChroot(rootDir, &g.buildCfg.Config)
	out, err := chRoot.Run("grub2-mkimage", args...)
	if err != nil {
		g.buildCfg.Logger.Errorf("grub2-mkimage failed: %s", string(out))
		g.buildCfg.Logger.Errorf("Error: %v", err)
		return "", err
	}

	concatFiles := func() error {
		return utils.ConcatFiles(
			g.buildCfg.Fs, []string{grubBiosCDBoot, grubBiosImg},
			grubEltoritoImg,
		)
	}
	err = chRoot.RunCallback(concatFiles)
	if err != nil {
		return "", err
	}
	return grubEltoritoImg, nil
}
