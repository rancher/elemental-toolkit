#!/bin/bash 

# This is PoC for building images without requiring admin capabilities (CAP_SYS_ADMIN)

rm -rf ./*.part disk.raw grub_efi.cfg recovery root .luet.yaml

set -e

# Create a luet config for local repositories
cat << EOF > .luet.yaml
repositories:
  - name: cOS
    enable: true
    urls:
      - build
    type: disk
EOF

# Create root-tree for COS_RECOVERY
mkdir -p recovery
# `luet install` is the only step requiring root
luet install --system-target recovery -y recovery/cos
mkdir -p root/grub2/x86_64-efi
mkdir -p root/cOS
cp recovery/usr/share/grub2/x86_64-efi/*.mod root/grub2/x86_64-efi
cp recovery/etc/cos/grub.cfg root/grub2
sed -i 's/${saved_entry}/recovery/g' root/grub2/grub.cfg
luet install --system-target root/cOS -y recovery/cos-img

# Create a 2GB filesystem for COS_RECOVERY including the contents for root (grub config and squasfs container)
truncate -s $((2048*1024*1024)) rootfs.part
mkfs.ext2 -L COS_RECOVERY -d root rootfs.part

# Create the EFI partition FAT16 and include the EFI image and a basic grub.cfg
truncate -s $((20*1024*1024)) efi.part

mkfs.fat -F16 -n EFI efi.part
mmd -i efi.part ::EFI
mmd -i efi.part ::EFI/BOOT
mcopy -i efi.part recovery/usr/share/grub2/x86_64-efi/grub.efi ::EFI/BOOT/bootx64.efi

# Creat the EFI grub.cfg pointing the configs in COS_RECOVERY volume
cat << 'EOF' > grub_efi.cfg
search.fs_label COS_RECOVERY root
set root=($root)
set prefix=($root)/grub2
configfile ($root)/grub2/grub.cfg
EOF

# Copy grub.cfg in EFI partition
mcopy -i efi.part grub_efi.cfg ::EFI/BOOT/grub.cfg

# Create a 64MB filesystem for COS_OEM volume
truncate -s $((64*1024*1024)) oem.part
mkfs.ext2 -L COS_OEM oem.part

# Create disk image, add 3MB of initial free space to disk, 1MB is for proper alignement, 2MB are for the hybrid legacy boot.
truncate -s $((3*1024*1024)) disk.raw
{
    cat efi.part
    cat oem.part
    cat rootfs.part
} >> disk.raw

# Add an extra MB at the end of the disk for the gpt headers, in fact 34 sectors would be enough, but adding some more does not hurt.
truncate -s "+$((1024*1024))" disk.raw

# Create the partition table in disk.raw (assumes sectors of 512 bytes)
sgdisk -n 1:2048:+2M -c 1:legacy -t 1:EF02 disk.raw
sgdisk -n 2:0:+20M -c 2:UEFI -t 2:EF00 disk.raw
sgdisk -n 3:0:+64M -c 3:oem -t 3:8300 disk.raw
sgdisk -n 4:0:+2048M -c 4:root -t 4:8300 disk.raw

rm -rf ./*.part grub_efi.cfg recovery root .luet.yaml

# TODO hybrid boot. I did not fully figure out how to avoid grub2-install. Rough steps are:
#   1-> add MBR pointing to sector 2048 (first block of legacy partition) (this is the tricky part grub2-install patches the MBR binary)
#   2-> create core.img with the relevant modules and an embedded grub.cfg equivalent to grub_efi.cfg (grub2-mkimage tool should do the trick)
#   3-> dump grub core image for i386-pc into 2048 sector
