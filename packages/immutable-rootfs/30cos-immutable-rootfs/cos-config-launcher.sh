#!/bin/bash

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

#======================================
# Functions
#--------------------------------------
function chroot_mounts {
    local mountpoint=$1
    mount -t proc /proc "${mountpoint}/proc/"
    mount -t sysfs /sys "${mountpoint}/sys/"
    mount --bind /dev "${mountpoint}/dev/"
}

function chroot_umounts {
    local mountpoint=$1
    umount "${mountpoint}/proc/"
    umount "${mountpoint}/sys/"
    umount "${mountpoint}/dev/"
}

#======================================
# Trigger pre-pivot config stage
#--------------------------------------

if ismounted /sysroot; then
    chroot_mounts /sysroot
    chroot /sysroot /usr/bin/cos-setup initramfs
    chroot_umounts /sysroot
fi
