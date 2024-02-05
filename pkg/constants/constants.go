/*
Copyright © 2022 - 2024 SUSE LLC

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
	"os"
	"path/filepath"
	"runtime"
)

const (
	BiosPartName       = "bios"
	EfiLabel           = "COS_GRUB"
	EfiPartName        = "efi"
	SystemLabel        = "COS_SYSTEM"
	RecoveryLabel      = "COS_RECOVERY"
	RecoveryPartName   = "recovery"
	StateLabel         = "COS_STATE"
	StatePartName      = "state"
	InstallStateFile   = "state.yaml"
	PersistentLabel    = "COS_PERSISTENT"
	PersistentPartName = "persistent"
	OEMLabel           = "COS_OEM"
	OEMPartName        = "oem"
	MountBinary        = "/usr/bin/mount"
	EfiDevice          = "/sys/firmware/efi"
	LinuxFs            = "ext4"
	LinuxImgFs         = "ext2"
	SquashFs           = "squashfs"
	EfiFs              = "vfat"
	Btrfs              = "btrfs"
	BiosFs             = ""
	MinPartSize        = uint(64)
	EfiSize            = MinPartSize
	OEMSize            = MinPartSize
	StateSize          = uint(8192)
	RecoverySize       = uint(4096)
	PersistentSize     = uint(0)
	BiosSize           = uint(1)
	ImgSize            = uint(0)
	ImgOverhead        = uint(256)
	HTTPTimeout        = 60
	GPT                = "gpt"
	BuildImgName       = "elemental"
	OEMPath            = "/oem"
	PersistentPath     = PersistentDir
	ConfigDir          = "/etc/elemental"
	OverlayMode        = "overlay"
	BindMode           = "bind"
	Tmpfs              = "tmpfs"
	Autofs             = "auto"
	Block              = "block"

	// Maxium number of nested symlinks to resolve
	MaxLinkDepth = 4

	// Kernel and initrd paths
	KernelModulesDir = "/lib/modules"
	KernelPath       = "/boot/vmlinuz"
	InitrdPath       = "/boot/initrd"
	ElementalInitrd  = "/boot/elemental.initrd"

	// Bootloader constants
	EntryEFIPath           = "/EFI/ELEMENTAL"
	FallbackEFIPath        = "/EFI/BOOT"
	BootEntryName          = "elemental-shim"
	EfiImgX86              = "bootx64.efi"
	EfiImgArm64            = "bootaa64.efi"
	EfiImgRiscv64          = "bootriscv64.efi"
	GrubCfg                = "grub.cfg"
	GrubCfgPath            = "/etc/elemental"
	GrubOEMEnv             = "grub_oem_env"
	GrubEnv                = "grubenv"
	GrubDefEntry           = "Elemental"
	GrubFallback           = "default_fallback"
	GrubPassiveSnapshots   = "passive_snaps"
	ElementalBootloaderBin = "/usr/lib/elemental/bootloader"

	// Mountpoints of images and partitions
	RunElementalDir    = "/run/elemental"
	RecoveryDir        = "/run/elemental/recovery"
	StateDir           = "/run/elemental/state"
	OEMDir             = "/run/elemental/oem"
	PersistentDir      = "/run/elemental/persistent"
	TransitionDir      = "/run/elemental/transition"
	EfiDir             = "/run/elemental/efi"
	ImgSrcDir          = "/run/elemental/imgsrc"
	WorkingImgDir      = "/run/elemental/workingtree"
	OverlayDir         = "/run/elemental/overlay"
	PersistentStateDir = ".state"
	RunningStateDir    = "/run/initramfs/elemental-state" // TODO: converge this constant with StateDir/RecoveryDir when moving to elemental-rootfs as default rootfs feature.

	// Running mode sentinel files
	ActiveMode   = "/run/elemental/active_mode"
	PassiveMode  = "/run/elemental/passive_mode"
	RecoveryMode = "/run/elemental/recovery_mode"

	// Live image mountpoints
	ISOBaseTree = "/run/rootfsbase"
	LiveDir     = "/run/initramfs/live"

	// Image constants
	ActiveImgName     = "active"
	PassiveImgName    = "passive"
	RecoveryImgName   = "recovery"
	RecoveryImgFile   = "recovery.img"
	TransitionImgFile = "transition.img"

	// Yip stages evaluated on reset/upgrade/install/build-disk actions
	AfterInstallChrootHook = "after-install-chroot"
	AfterInstallHook       = "after-install"
	PostInstallHook        = "post-install"
	BeforeInstallHook      = "before-install"
	AfterResetChrootHook   = "after-reset-chroot"
	AfterResetHook         = "after-reset"
	PostResetHook          = "post-reset"
	BeforeResetHook        = "before-reset"
	AfterUpgradeChrootHook = "after-upgrade-chroot"
	AfterUpgradeHook       = "after-upgrade"
	PostUpgradeHook        = "post-upgrade"
	BeforeUpgradeHook      = "before-upgrade"
	AfterDiskChrootHook    = "after-disk-chroot"
	AfterDiskHook          = "after-disk"
	PostDiskHook           = "post-disk"
	BeforeDiskHook         = "before-disk"

	// SELinux targeted policy paths
	SELinuxTargetedPath        = "/etc/selinux/targeted"
	SELinuxTargetedContextFile = SELinuxTargetedPath + "/contexts/files/file_contexts"
	SELinuxTargetedPolicyPath  = SELinuxTargetedPath + "/policy"

	ISORootFile      = "rootfs.squashfs"
	ISOEFIImg        = "uefi.img"
	ISOLabel         = "COS_LIVE"
	ISOCloudInitPath = LiveDir + "/iso-config"

	// Constants related to disk builds
	DiskWorkDir = "build"
	RawType     = "raw"
	AzureType   = "azure"
	GCEType     = "gce"

	// Default directory and file fileModes
	DirPerm        = os.ModeDir | os.ModePerm
	FilePerm       = 0666
	NoWriteDirPerm = 0555 | os.ModeDir
	TempDirPerm    = os.ModePerm | os.ModeSticky | os.ModeDir

	// Eject script
	EjectScript = "#!/bin/sh\n/usr/bin/eject -rmF"

	ArchAmd64   = "amd64"
	Archx86     = "x86_64"
	ArchArm64   = "arm64"
	ArchAarch64 = "aarch64"
	ArchRiscV64 = "riscv64"

	Rsync = "rsync"

	// Snapshotters
	MaxSnaps                  = 2
	LoopDeviceSnapshotterType = "loopdevice"
	BtrfsSnapshotterType      = "btrfs"
	ActiveSnapshot            = "active"
	PassiveSnapshot           = "passive_%d"

	// Legacy paths
	LegacyImagesPath  = "cOS"
	LegacyPassivePath = LegacyImagesPath + "/passive.img"
	LegacyActivePath  = LegacyImagesPath + "/active.img"
	LegacyStateDir    = "/run/initramfs/cos-state"
)

// GetDefaultSystemEcludes returns a list of transient paths
// that are commonly present in an Elemental based running system.
// Those paths are not needed or wanted in order to replicate the
// root-tree as they are generated at runtime.
func GetDefaultSystemExcludes() []string {
	return []string{
		"/mnt",
		"/proc",
		"/sys",
		"/dev",
		"/tmp",
		"/run",
		"/host",
		"/etc/resolv.conf",
	}
}

func GetKernelPatterns() []string {
	return []string{
		"/boot/uImage*",
		"/boot/Image*",
		"/boot/zImage*",
		"/boot/vmlinuz*",
		"/boot/image*",
	}
}

func GetInitrdPatterns() []string {
	return []string{
		"/boot/elemental.initrd*",
		"/boot/initrd*",
		"/boot/initramfs*",
	}
}

func GetShimFilePatterns() []string {
	return []string{
		filepath.Join(ElementalBootloaderBin, "shim*"),
		"/usr/share/efi/*/shim.efi",
		"/boot/efi/EFI/*/shim*.efi",
	}
}

func GetGrubEFIFilePatterns() []string {
	return []string{
		filepath.Join(ElementalBootloaderBin, "grub*"),
		"/usr/share/grub2/*-efi/grub.efi",
		"/boot/efi/EFI/*/grub*.efi",
	}
}

func GetMokMngrFilePatterns() []string {
	return []string{
		filepath.Join(ElementalBootloaderBin, "mm*"),
		"/boot/efi/EFI/*/mm*.efi",
		"/usr/share/efi/*/MokManager.efi",
	}
}

func GetDefaultGrubModules() []string {
	return []string{"loopback.mod", "squash4.mod", "xzio.mod"}
}

func GetDefaultGrubModulesPatterns() []string {
	return []string{
		"/boot/grub2/*-efi",
		"/usr/share/grub*/*-efi",
		"/usr/lib/grub*/*-efi",
	}
}

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

// GetRunKeyEnvMap returns environment variable bindings to RunConfig data
func GetRunKeyEnvMap() map[string]string {
	return map[string]string{
		"poweroff": "POWEROFF",
		"reboot":   "REBOOT",
		"strict":   "STRICT",
		"eject-cd": "EJECT_CD",
	}
}

// GetInitKeyEnvMap returns environment variable bindings to InitSpec data
func GetInitKeyEnvMap() map[string]string {
	return map[string]string{
		"mkinitrd": "MKINITRD",
		"force":    "FORCE",
	}
}

// GetMountKeyEnvMap returns environment variable bindings to MountSpec data
func GetMountKeyEnvMap() map[string]string {
	return map[string]string{
		"write-fstab": "WRITE_FSTAB",
		"sysroot":     "SYSROOT",
	}
}

// GetInstallKeyEnvMap returns environment variable bindings to InstallSpec data
func GetInstallKeyEnvMap() map[string]string {
	return map[string]string{
		"target":             "TARGET",
		"system":             "SYSTEM",
		"recovery-system":    "RECOVERY_SYSTEM",
		"cloud-init":         "CLOUD_INIT",
		"iso":                "ISO",
		"firmware":           "FIRMWARE",
		"part-table":         "PART_TABLE",
		"no-format":          "NO_FORMAT",
		"grub-entry-name":    "GRUB_ENTRY_NAME",
		"disable-boot-entry": "DISABLE_BOOT_ENTRY",
	}
}

// GetResetKeyEnvMap returns environment variable bindings to ResetSpec data
func GetResetKeyEnvMap() map[string]string {
	return map[string]string{
		"system":             "SYSTEM",
		"grub-entry-name":    "GRUB_ENTRY_NAME",
		"cloud-init":         "CLOUD_INIT",
		"reset-persistent":   "PERSISTENT",
		"reset-oem":          "OEM",
		"disable-boot-entry": "DISABLE_BOOT_ENTRY",
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

// GetBootPath returns path use to store the boot files
func ISOLoaderPath(arch string) string {
	return filepath.Join("/boot", arch, "loader")
}

// ISOKernelPath returns path use to store the kernel
func ISOKernelPath(arch string) string {
	return filepath.Join(ISOLoaderPath(arch), "linux")
}

// ISOInitrdPath returns path use to store the initramfs
func ISOInitrdPath(arch string) string {
	return filepath.Join(ISOLoaderPath(arch), "initrd")
}
