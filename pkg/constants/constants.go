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

package constants

import (
	"runtime"
)

const (
	GrubConf               = "/etc/cos/grub.cfg"
	GrubOEMEnv             = "grub_oem_env"
	GrubDefEntry           = "cOs"
	BiosPartName           = "p.bios"
	EfiLabel               = "COS_GRUB"
	EfiPartName            = "p.grub"
	ActiveLabel            = "COS_ACTIVE"
	PassiveLabel           = "COS_PASSIVE"
	SystemLabel            = "COS_SYSTEM"
	RecoveryLabel          = "COS_RECOVERY"
	RecoveryPartName       = "p.recovery"
	StateLabel             = "COS_STATE"
	StatePartName          = "p.state"
	PersistentLabel        = "COS_PERSISTENT"
	PersistentPartName     = "p.persistent"
	OEMLabel               = "COS_OEM"
	OEMPartName            = "p.oem"
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
	PartStage              = "partitioning"
	IsoMnt                 = "/run/initramfs/live"
	RecoveryDir            = "/run/cos/recovery"
	StateDir               = "/run/cos/state"
	OEMDir                 = "/run/cos/oem"
	PersistentDir          = "/run/cos/persistent"
	ActiveDir              = "/run/cos/active"
	EfiDir                 = "/run/cos/efi"
	DownloadedIsoMnt       = "/run/cos/iso"
	RecoverySquashFile     = "recovery.squashfs"
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
	UpgradeActive          = "active"
	UpgradeRecovery        = "recovery"
	UpgradeSource          = "system/cos"
	UpgradeRecoveryDir     = "/run/initramfs/live"
	TransitionImgFile      = "transition.img"
	TransitionSquashFile   = "transition.squashfs"
	// TODO converge this constant with StateDir/RecoveryDir in dracut module from cos-toolkit
	RunningStateDir = "/run/initramfs/cos-state"
	ActiveImgName   = "active"
	PassiveImgName  = "passive"
	RecoveryImgName = "recovery"
)

func GetCloudInitPaths() []string {
	return []string{"/system/oem", "/oem/", "/usr/local/cloud-config/"}
}

// GetDefaultSquashfsOptions returns the default options to use when creating a squashfs
func GetDefaultSquashfsOptions() []string {
	options := []string{"-b", "1024k", "-comp", "xz", "-Xbcj"}
	// Set the filter based on arch for best compression results
	if runtime.GOARCH == "arm64" {
		options = append(options, "arm")
	} else {
		options = append(options, "x86")
	}
	return options
}
