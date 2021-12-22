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

const (
	GrubConf           = "/etc/cos/grub.cfg"
	BiosPLabel         = "p.bios"
	EfiLabel           = "COS_GRUB"
	EfiPLabel          = "p.grub"
	ActiveLabel        = "COS_ACTIVE"
	PassiveLabel       = "COS_PASSIVE"
	SystemLabel        = "COS_SYSTEM"
	RecoveryLabel      = "COS_RECOVERY"
	RecoveryPLabel     = "p.recovery"
	StateLabel         = "COS_STATE"
	StatePLabel        = "p.state"
	PersistentLabel    = "COS_PERSISTENT"
	PersistentPLabel   = "p.persistent"
	OEMLabel           = "COS_OEM"
	OEMPLabel          = "p.oem"
	ActivePLabel       = "p.active"
	MountBinary        = "/usr/bin/mount"
	EfiDevice          = "/sys/firmware/efi"
	LinuxFs            = "ext4"
	EfiFs              = "vfat"
	BiosFs             = ""
	EfiSize            = uint(64)
	OEMSize            = uint(64)
	StateSize          = uint(15360)
	RecoverySize       = uint(8192)
	PersistentSize     = uint(0)
	BiosSize           = uint(1)
	ImgSize            = uint(3072)
	PartStage          = "partitioning"
	RecoveryDirSquash  = "/run/initramfs/live"
	IsoMnt             = "/run/initramfs/live"
	RecoveryDir        = "/run/cos/recovery"
	StateDir           = "/run/cos/state"
	OEMDir             = "/run/cos/oem"
	ActiveDir          = "/run/cos/active"
	RecoverySquashFile = "recovery.squashfs"
	ActiveImgFile      = "active.img"
	PassiveImgFile     = "passive.img"
	RecoveryImgFile    = "recovery.img"
)
