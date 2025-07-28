/*
Copyright © 2022 - 2025 SUSE LLC

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

	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
)

const (
	efiBootPath    = "/EFI/BOOT"
	efiImgX86      = "bootx64.efi"
	efiImgArm64    = "bootaa64.efi"
	grubCfg        = "grub.cfg"
	grubPrefixDir  = "/boot/grub2"
	isoBootCatalog = "/boot/boot.catalog"

	// TODO document any custom BIOS bootloader must match this setup as these are not configurable
	// and coupled with the xorriso call
	IsoLoaderPath = "/boot/x86_64/loader"
	isoHybridMBR  = IsoLoaderPath + "/boot_hybrid.img"
	isoBootFile   = IsoLoaderPath + "/eltorito.img"

	//TODO use some identifer known to be unique
	grubEfiCfg = "search --no-floppy --file --set=root " + constants.ISOKernelPath +
		"\nset prefix=($root)" + grubPrefixDir +
		"\nconfigfile $prefix/" + grubCfg

	// TODO not convinced having such a config here is the best idea
	grubCfgTemplate = "search --no-floppy --file --set=root " + constants.ISOKernelPath + "\n" +
		`set default=0
	set timeout=10
	set timeout_style=menu
	set linux=linux
	set initrd=initrd
	if [ "${grub_cpu}" = "x86_64" -o "${grub_cpu}" = "i386" -o "${grub_cpu}" = "arm64" ];then
		if [ "${grub_platform}" = "efi" ]; then
			if [ "${grub_cpu}" != "arm64" ]; then
				set linux=linuxefi
				set initrd=initrdefi
			fi
		fi
	fi
	if [ "${grub_platform}" = "efi" ]; then
		echo "Please press 't' to show the boot menu on this console"
	fi

	menuentry "%s" --class os --unrestricted {
		echo Loading kernel...
		$linux ($root)` + constants.ISOKernelPath + ` cdroot root=live:CDLABEL=%s rd.live.dir=/ rd.live.squashimg=rootfs.squashfs console=tty1 console=ttyS0 rd.cos.disable cos.setup=` + constants.ISOCloudInitPath + `
		echo Loading initrd...
		$initrd ($root)` + constants.ISOInitrdPath + `
	}                                                                               
																					
	if [ "${grub_platform}" = "efi" ]; then                                         
		hiddenentry "Text mode" --hotkey "t" {                                      
			set textmode=true                                                       
			terminal_output console                                                 
		}                                                                           
	fi`
)

func XorrisoBooloaderArgs(root, efiImg, firmware string) []string {
	switch firmware {
	case v1.EFI:
		args := []string{
			"-append_partition", "2", "0xef", efiImg,
			"-boot_image", "any", fmt.Sprintf("cat_path=%s", isoBootCatalog),
			"-boot_image", "any", "cat_hidden=on",
			"-boot_image", "any", "efi_path=--interval:appended_partition_2:all::",
			"-boot_image", "any", "platform_id=0xef",
			"-boot_image", "any", "appended_part_as=gpt",
			"-boot_image", "any", "partition_offset=16",
		}
		return args
	case v1.BIOS:
		args := []string{
			"-boot_image", "grub", fmt.Sprintf("bin_path=%s", isoBootFile),
			"-boot_image", "grub", fmt.Sprintf("grub2_mbr=%s/%s", root, isoHybridMBR),
			"-boot_image", "grub", "grub2_boot_info=on",
			"-boot_image", "any", "partition_offset=16",
			"-boot_image", "any", fmt.Sprintf("cat_path=%s", isoBootCatalog),
			"-boot_image", "any", "cat_hidden=on",
			"-boot_image", "any", "boot_info_table=on",
			"-boot_image", "any", "platform_id=0x00",
		}
		return args
	default:
		return []string{}
	}
}
