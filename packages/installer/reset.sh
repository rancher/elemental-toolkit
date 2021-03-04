#!/bin/bash
set -e

# FIXME: should have two ways for resetting
# - easiest/reliable: copy 1:1 RECOVERY partition. 
#   cOS just detecting if it's running by recovery partition should be able to behave differently
# - less repro, but keeps things up-to-date: rerun installation steps into COS_STATE. 
#   But what upgrades the recovery mechanism itself remains an open point. It should be run as part of cos-upgrade, maybe with a flag.
# At the moment cos-reset is implementing the latter as we don't have a recovery partition (yet).

find_partitions() {
    STATE=$(blkid -L COS_STATE || true)
    if [ -z "$STATE" ]; then
        echo "State partition cannot be found"
        exit 1
    fi
    DEVICE=/dev/$(lsblk -no pkname $STATE)

    BOOT=$(blkid -L COS_GRUB || true)
}

do_mount()
{
    STATEDIR=/run/initramfs/isoscan
    mount -o remount,rw ${STATE} ${STATEDIR}

    if [ -n "${BOOT}" ]; then
        mkdir -p /boot/efi || true
        mount ${BOOT} /boot/efi
    fi
    mkdir -p /boot/grub2 || true
    mount ${STATE} /boot/grub2
}

cleanup2()
{  
    umount /boot/efi || true
    umount /boot/grub2 || true
    mount -o remount,ro ${STATE} ${STATEDIR} || true
}

cleanup()
{
    EXIT=$?
    cleanup2 2>/dev/null || true
    return $EXIT
}

install_grub()
{
    mount -o remount,rw ${STATE} /boot/grub2
    grub2-install ${DEVICE}
}

reset() {
    rm -rf /oem/*
    rm -rf /usr/local/*
}

copy_active() {
    mount -o remount,rw ${STATE} ${STATEDIR}
    cp -rf ${STATEDIR}/cOS/active.img ${STATEDIR}/cOS/passive.img
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
}

trap cleanup exit

find_partitions
do_mount

if [ -n "$PERSISTENCE_RESET" ] && [ "$PERSISTENCE_RESET" == "true" ]; then
    reset
fi

CURRENT=passive.img cos-upgrade

install_grub

copy_active