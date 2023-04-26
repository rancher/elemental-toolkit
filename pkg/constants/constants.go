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

package constants

import (
	"os"
	"runtime"
)

const (
	GrubConf           = "/etc/cos/grub.cfg"
	GrubOEMEnv         = "grub_oem_env"
	GrubDefEntry       = "cOS"
	DefaultTty         = "tty1"
	BiosPartName       = "bios"
	EfiLabel           = "COS_GRUB"
	EfiPartName        = "efi"
	ActiveLabel        = "COS_ACTIVE"
	PassiveLabel       = "COS_PASSIVE"
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
	ActiveImgName      = "active"
	PassiveImgName     = "passive"
	RecoveryImgName    = "recovery"
	MountBinary        = "/usr/bin/mount"
	EfiDevice          = "/sys/firmware/efi"
	LinuxFs            = "ext4"
	LinuxImgFs         = "ext2"
	SquashFs           = "squashfs"
	EfiFs              = "vfat"
	BiosFs             = ""
	EfiSize            = uint(64)
	OEMSize            = uint(64)
	StateSize          = uint(8192)
	RecoverySize       = uint(4096)
	PersistentSize     = uint(0)
	BiosSize           = uint(1)
	ImgSize            = uint(0)
	ImgOverhead        = uint(256)
	HTTPTimeout        = 60
	CosSetup           = "/usr/bin/cos-setup"
	GPT                = "gpt"
	BuildImgName       = "elemental"
	UsrLocalPath       = "/usr/local"
	OEMPath            = "/oem"
	ConfigDir          = "/etc/elemental"

	// Mountpoints of images and partitions
	RecoveryDir     = "/run/cos/recovery"
	StateDir        = "/run/cos/state"
	OEMDir          = "/run/cos/oem"
	PersistentDir   = "/run/cos/persistent"
	ActiveDir       = "/run/cos/active"
	TransitionDir   = "/run/cos/transition"
	EfiDir          = "/run/cos/efi"
	ImgSrcDir       = "/run/cos/imgsrc"
	WorkingImgDir   = "/run/cos/workingtree"
	RunningStateDir = "/run/initramfs/cos-state" // TODO: converge this constant with StateDir/RecoveryDir in dracut module from cos-toolkit

	// Live image mountpoints
	ISOBaseTree = "/run/rootfsbase"
	LiveDir     = "/run/initramfs/live"

	// Image file names
	ActiveImgFile     = "active.img"
	PassiveImgFile    = "passive.img"
	RecoveryImgFile   = "recovery.img"
	TransitionImgFile = "transition.img"

	// Yip stages evaluated on reset/upgrade/install action
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

	// SELinux targeted policy paths
	SELinuxTargetedPath        = "/etc/selinux/targeted"
	SELinuxTargetedContextFile = SELinuxTargetedPath + "/contexts/files/file_contexts"
	SELinuxTargetedPolicyPath  = SELinuxTargetedPath + "/policy"

	// Kernel and initrd paths are arbitrary and coupled to grub.cfg
	ISOKernelPath    = "/boot/kernel"
	ISOInitrdPath    = "/boot/initrd"
	ISORootFile      = "rootfs.squashfs"
	ISOEFIImg        = "uefi.img"
	ISOLabel         = "COS_LIVE"
	ISOCloudInitPath = LiveDir + "/iso-config"

	// Default directory and file fileModes
	DirPerm        = os.ModeDir | os.ModePerm
	FilePerm       = 0666
	NoWriteDirPerm = 0555 | os.ModeDir
	TempDirPerm    = os.ModePerm | os.ModeSticky | os.ModeDir

	// Eject script
	EjectScript = "#!/bin/sh\n/usr/bin/eject -rmF"

	ArchAmd64 = "amd64"
	Archx86   = "x86_64"
	ArchArm64 = "arm64"

	Fedora = "fedora"
	Ubuntu = "ubuntu"
	Suse   = "suse"
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

// GetRunKeyEnvMap returns environment variable bindings to RunConfig data
func GetRunKeyEnvMap() map[string]string {
	return map[string]string{
		"poweroff": "POWEROFF",
		"reboot":   "REBOOT",
		"strict":   "STRICT",
		"eject-cd": "EJECT_CD",
	}
}

// GetInstallKeyEnvMap returns environment variable bindings to InstallSpec data
func GetInstallKeyEnvMap() map[string]string {
	return map[string]string{
		"target":              "TARGET",
		"system.uri":          "SYSTEM",
		"recovery-system.uri": "RECOVERY_SYSTEM",
		"cloud-init":          "CLOUD_INIT",
		"iso":                 "ISO",
		"firmware":            "FIRMWARE",
		"part-table":          "PART_TABLE",
		"no-format":           "NO_FORMAT",
		"tty":                 "TTY",
		"grub-entry-name":     "GRUB_ENTRY_NAME",
		"disable-boot-entry":  "DISABLE_BOOT_ENTRY",
	}
}

// GetResetKeyEnvMap returns environment variable bindings to ResetSpec data
func GetResetKeyEnvMap() map[string]string {
	return map[string]string{
		"target":          "TARGET",
		"system.uri":      "SYSTEM",
		"tty":             "TTY",
		"grub-entry-name": "GRUB_ENTRY_NAME",
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
