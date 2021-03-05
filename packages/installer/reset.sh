#!/bin/bash
set -e

check_recovery() {
    SYSTEM=$(blkid -L COS_SYSTEM || true)
    if [ -z "$SYSTEM" ]; then
        echo "cos-reset can be run only from recovery"
        exit 1
    fi
    RECOVERY=$(blkid -L COS_RECOVERY || true)
    if [ -z "$RECOVERY" ]; then
        echo "Can't find COS_RECOVERY partition"
        exit 1
    fi
}

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
    STATEDIR=/tmp/state
    mkdir -p $STATEDIR || true
    RECOVERYDIR=/run/initramfs/isoscan
    #mount -o remount,rw ${STATE} ${STATEDIR}

    if [ -n "${BOOT}" ]; then
        mkdir -p /boot/efi || true
        mount ${BOOT} /boot/efi
    fi
    mkdir -p /boot/grub2 || true
    mount ${STATE} /boot/grub2
    mount ${STATE} $STATEDIR
}

cleanup2()
{  
    umount /boot/efi || true
    umount /boot/grub2 || true
}

cleanup()
{
    EXIT=$?
    cleanup2 2>/dev/null || true
    return $EXIT
}

install_grub()
{
    #mount -o remount,rw ${STATE} /boot/grub2
    grub2-install ${DEVICE}
}

reset() {
    rm -rf /oem/*
    rm -rf /usr/local/*
}

copy_active() {
    cp -rf ${RECOVERYDIR}/cOS/recovery.img ${STATEDIR}/cOS/passive.img
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
    cp -rf ${STATEDIR}/cOS/passive.img ${STATEDIR}/cOS/active.img
    tune2fs -L COS_ACTIVE ${STATEDIR}/cOS/active.img
}

trap cleanup exit

check_recovery

find_partitions

do_mount

if [ -n "$PERSISTENCE_RESET" ] && [ "$PERSISTENCE_RESET" == "true" ]; then
    reset
fi

copy_active

install_grub