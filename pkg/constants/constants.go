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

package constants

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	GrubConf               = "/etc/cos/grub.cfg"
	GrubOEMEnv             = "grub_oem_env"
	GrubDefEntry           = "cOS"
	DefaultTty             = "tty1"
	BiosPartName           = "bios"
	EfiLabel               = "COS_GRUB"
	EfiPartName            = "efi"
	ActiveLabel            = "COS_ACTIVE"
	PassiveLabel           = "COS_PASSIVE"
	SystemLabel            = "COS_SYSTEM"
	RecoveryLabel          = "COS_RECOVERY"
	RecoveryPartName       = "recovery"
	StateLabel             = "COS_STATE"
	StatePartName          = "state"
	PersistentLabel        = "COS_PERSISTENT"
	PersistentPartName     = "persistent"
	OEMLabel               = "COS_OEM"
	OEMPartName            = "oem"
	ISOLabel               = "COS_LIVE"
	MountBinary            = "/usr/bin/mount"
	EfiDevice              = "/sys/firmware/efi"
	LinuxFs                = "ext4"
	LinuxImgFs             = "ext2"
	SquashFs               = "squashfs"
	EfiFs                  = "vfat"
	BiosFs                 = ""
	EfiSize                = uint(64)
	OEMSize                = uint(64)
	StateSize              = uint(15360)
	RecoverySize           = uint(8192)
	PersistentSize         = uint(0)
	BiosSize               = uint(1)
	ImgSize                = uint(3072)
	HTTPTimeout            = 60
	PartStage              = "partitioning"
	LiveDir                = "/run/initramfs/live"
	RecoveryDir            = "/run/cos/recovery"
	StateDir               = "/run/cos/state"
	OEMDir                 = "/run/cos/oem"
	PersistentDir          = "/run/cos/persistent"
	ActiveDir              = "/run/cos/active"
	TransitionDir          = "/run/cos/transition"
	EfiDir                 = "/run/cos/efi"
	RecoverySquashFile     = "recovery.squashfs"
	IsoRootFile            = "rootfs.squashfs"
	IsoEFIPath             = "/boot/uefi.img"
	ActiveImgFile          = "active.img"
	PassiveImgFile         = "passive.img"
	RecoveryImgFile        = "recovery.img"
	IsoBaseTree            = "/run/rootfsbase"
	CosSetup               = "/usr/bin/cos-setup"
	AfterInstallChrootHook = "after-install-chroot"
	AfterInstallHook       = "after-install"
	BeforeInstallHook      = "before-install"
	AfterResetChrootHook   = "after-reset-chroot"
	AfterResetHook         = "after-reset"
	BeforeResetHook        = "before-reset"
	LuetCosignPlugin       = "luet-cosign"
	LuetMtreePlugin        = "luet-mtree"
	LuetDefaultRepoURI     = "quay.io/costoolkit/releases-green"
	LuetRepoMaxPrio        = 1
	LuetDefaultRepoPrio    = 90
	UpgradeActive          = "active"
	UpgradeRecovery        = "recovery"
	ChannelSource          = "system/cos"
	TransitionImgFile      = "transition.img"
	TransitionSquashFile   = "transition.squashfs"
	RunningStateDir        = "/run/initramfs/cos-state" // TODO: converge this constant with StateDir/RecoveryDir in dracut module from cos-toolkit
	ActiveImgName          = "active"
	PassiveImgName         = "passive"
	RecoveryImgName        = "recovery"
	GPT                    = "gpt"
	BuildImgName           = "elemental"
	UsrLocalPath           = "/usr/local"
	OEMPath                = "/oem"

	//TODO these paths are abitrary, coupled to package live/grub2 and assuming xz
	// I'd suggest using `/boot/kernel` and `/boot/initrd`
	IsoKernelPath = "/boot/kernel.xz"
	IsoInitrdPath = "/boot/rootfs.xz"

	// TODO would be nice to discover these ISO loader values instead of hardcoding them
	// These values are coupled with package live/grub2
	IsoHybridMBR   = "/boot/x86_64/loader/boot_hybrid.img"
	IsoBootCatalog = "/boot/x86_64/boot.catalog"
	IsoBootFile    = "/boot/x86_64/loader/eltorito.img"

	// Default directory and file fileModes
	DirPerm        = os.ModeDir | os.ModePerm
	FilePerm       = 0666
	NoWriteDirPerm = 0555 | os.ModeDir
	TempDirPerm    = os.ModePerm | os.ModeSticky | os.ModeDir

	// Eject script
	EjectScript = "#!/bin/sh\n/usr/bin/eject -rmF"
)

func GetCloudInitPaths() []string {
	return []string{"/system/oem", "/oem/", "/usr/local/cloud-config/"}
}

// GetDefaultSquashfsOptions returns the default options to use when creating a squashfs
func GetDefaultSquashfsOptions() []string {
	return []string{"-b", "1024k"}
}

func GetDefaultSquashfsCompressionOptions() []string {
	options := []string{"-comp", "xz", "-Xbcj"}
	// Set the filter based on arch for best compression results
	if runtime.GOARCH == "arm64" {
		options = append(options, "arm")
	} else {
		options = append(options, "x86")
	}
	return options
}

func GetDefaultXorrisoBooloaderArgs(root, bootFile, bootCatalog, hybridMBR string) []string {
	args := []string{}
	// TODO: make this detection more robust or explicit
	// Assume ISOLINUX bootloader is used if boot file is includes 'isolinux'
	// in its name, otherwise assume an eltorito based grub2 setup
	if strings.Contains(bootFile, "isolinux") {
		args = append(args, []string{
			"-boot_image", "isolinux", fmt.Sprintf("bin_path=%s", bootFile),
			"-boot_image", "isolinux", fmt.Sprintf("system_area=%s/%s", root, hybridMBR),
			"-boot_image", "isolinux", "partition_table=on",
		}...)
	} else {
		args = append(args, []string{
			"-boot_image", "grub", fmt.Sprintf("bin_path=%s", bootFile),
			"-boot_image", "grub", fmt.Sprintf("grub2_mbr=%s/%s", root, hybridMBR),
			"-boot_image", "grub", "grub2_boot_info=on",
		}...)
	}

	args = append(args, []string{
		"-boot_image", "any", "partition_offset=16",
		"-boot_image", "any", fmt.Sprintf("cat_path=%s", bootCatalog),
		"-boot_image", "any", "cat_hidden=on",
		"-boot_image", "any", "boot_info_table=on",
		"-boot_image", "any", "platform_id=0x00",
		"-boot_image", "any", "emul_type=no_emulation",
		"-boot_image", "any", "load_size=2048",
		"-append_partition", "2", "0xef", filepath.Join(root, IsoEFIPath),
		"-boot_image", "any", "next",
		"-boot_image", "any", "efi_path=--interval:appended_partition_2:all::",
		"-boot_image", "any", "platform_id=0xef",
		"-boot_image", "any", "emul_type=no_emulation",
	}...)
	return args
}

func GetDefaultISOImage() []string {
	return []string{"live/grub2", "live/grub2-efi-image"}
}

func GetDefaultISOUEFI() []string {
	return []string{"live/grub2-efi-image"}
}

func GetBuildDiskDefaultPackages() map[string]string {
	return map[string]string{
		"system/grub2-efi-image": "efi",
		"system/grub2-config":    "root",
		"system/grub2-artifacts": "root/grub2",
		"recovery/cos-img":       "root/cOS",
	}
}

// GetRunKeyEnvMap returns environment variable bindings to RunConfig data
func GetRunKeyEnvMap() map[string]string {
	// None for the time being
	return map[string]string{}
}

// GetInstallKeyEnvMap returns environment variable bindings to InstallSpec data
func GetInstallKeyEnvMap() map[string]string {
	return map[string]string{
		"target":              "TARGET",
		"system.uri":          "SYSTEM",
		"recovery-system.uri": "RECOVERY_SYSTEM",
	}
}

// GetResetKeyEnvMap returns environment variable bindings to ResetSpec data
func GetResetKeyEnvMap() map[string]string {
	return map[string]string{
		"target":     "TARGET",
		"system.uri": "SYSTEM",
	}
}

// GetUpgradeKeyEnvMap returns environment variable bindings to UpgradeSpec data
func GetUpgradeKeyEnvMap() map[string]string {
	return map[string]string{
		"recovery":            "RECOVERY",
		"system.uri":          "SYSTEM",
		"recovery-system.uri": "RECOVERY_SYSTEM",
	}
}

// GetBuildKeyEnvMap returns environment variable bindings to BuildConfig data
func GetBuildKeyEnvMap() map[string]string {
	return map[string]string{
		"name": "NAME",
	}
}

// GetISOKeyEnvMap returns environment variable bindings to LiveISO data
func GetISOKeyEnvMap() map[string]string {
	// None for the time being
	return map[string]string{}
}

// GetDiskKeyEnvMap returns environment variable bindings to RawDisk data
func GetDiskKeyEnvMap() map[string]string {
	// None for the time being
	return map[string]string{}
}
