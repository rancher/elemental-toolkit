#!/bin/bash

# called by dracut
check() {
    require_binaries "$systemdutildir"/systemd || return 1
    return 255
}

# called by dracut 
depends() {
    echo systemd rootfs-block dm fs-lib
    return 0
}

# called by dracut
install() {
    declare moddir=${moddir}
    declare systemdutildir=${systemdutildir}
    declare systemdsystemunitdir=${systemdsystemunitdir}

    inst_multiple \
        ln mkdir mount umount elemental

    inst_multiple -o \
        systemd-analyze partprobe sync udevadm parted mkfs.ext2 mkfs.ext3 mkfs.ext4 mkfs.vfat \
	mkfs.fat mkfs.xfs blkid resize2fs mount xfs_growfs umount sgdisk partx mkfs.btrfs btrfs

    inst_script "${moddir}/oem-generator.sh" \
        "${systemdutildir}/system-generators/dracut-elemental-oem-generator"

    inst_simple "/etc/systemd/system/elemental-setup-pre-rootfs.service" \
        "${systemdsystemunitdir}/elemental-setup-pre-rootfs.service"
    mkdir -p "${initdir}/${systemdsystemunitdir}/initrd-root-device.target.wants"
    ln_r "../elemental-setup-pre-rootfs.service" \
        "${systemdsystemunitdir}/initrd-root-device.target.wants/elemental-setup-pre-rootfs.service"

    inst_simple "/etc/systemd/system/elemental-setup-rootfs.service" \
        "${systemdsystemunitdir}/elemental-setup-rootfs.service"
    mkdir -p "${initdir}/${systemdsystemunitdir}/initrd-root-fs.target.wants"
    ln_r "../elemental-setup-rootfs.service" \
        "${systemdsystemunitdir}/initrd-root-fs.target.wants/elemental-setup-rootfs.service"

    inst_simple "/etc/systemd/system/elemental-setup-initramfs.service" \
        "${systemdsystemunitdir}/elemental-setup-initramfs.service"
    mkdir -p "${initdir}/${systemdsystemunitdir}/initrd.target.wants"
    ln_r "../elemental-setup-initramfs.service" \
        "${systemdsystemunitdir}/initrd.target.wants/elemental-setup-initramfs.service"

    dracut_need_initqueue
}

