#!/bin/bash
set -e

is_booting_from_squashfs() {
    if cat /proc/cmdline | grep -q "COS_RECOVERY"; then
        return 0
    else
        return 1
    fi
}

check_recovery() {
    if ! is_booting_from_squashfs; then
        system=$(blkid -L COS_SYSTEM || true)
        if [ -z "$system" ]; then
            echo "cos-reset can be run only from recovery"
            exit 1
        fi
        recovery=$(blkid -L COS_RECOVERY || true)
        if [ -z "$recovery" ]; then
            echo "Can't find COS_RECOVERY partition"
            exit 1
        fi
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

    if is_booting_from_squashfs; then
        RECOVERYDIR=/run/initramfs/live
    else
        RECOVERYDIR=/run/initramfs/isoscan
    fi

    #mount -o remount,rw ${STATE} ${STATEDIR}

    mount ${STATE} $STATEDIR

    if [ -n "${BOOT}" ]; then
        mkdir -p $STATEDIR/boot/efi || true
        mount ${BOOT} $STATEDIR/boot/efi
    fi
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
    if [ "$COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        GRUB_TARGET="--target=x86_64-efi --efi-directory=${STATEDIR}/boot/efi"
    fi
    #mount -o remount,rw ${STATE} /boot/grub2
    grub2-install ${GRUB_TARGET} --root-directory=${STATEDIR} --boot-directory=${STATEDIR} --removable ${DEVICE}

    GRUBDIR=
    if [ -d "${STATEDIR}/grub" ]; then
        GRUBDIR="${STATEDIR}/grub"
    elif [ -d "${STATEDIR}/grub2" ]; then
        GRUBDIR="${STATEDIR}/grub2"
    fi

    cp -rfv /etc/cos/grub.cfg $GRUBDIR/grub.cfg
}

reset() {
    rm -rf /oem/*
    rm -rf /usr/local/*
}

copy_passive() {
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
    cp -rf ${STATEDIR}/cOS/passive.img ${STATEDIR}/cOS/active.img
    tune2fs -L COS_ACTIVE ${STATEDIR}/cOS/active.img
}

SELinux_relabel()
{
    if which setfiles > /dev/null && [ -e ${TARGET}/etc/selinux/targeted/contexts/files/file_contexts ]; then
        setfiles -r ${TARGET} ${TARGET}/etc/selinux/targeted/contexts/files/file_contexts ${TARGET}
    fi
}

ensure_dir_structure() {
    mkdir ${TARGET}/proc || true
    mkdir ${TARGET}/boot || true
    mkdir ${TARGET}/dev || true
    mkdir ${TARGET}/sys || true
    mkdir ${TARGET}/tmp || true
}

copy_active() {
    if is_booting_from_squashfs; then
        tmp_dir=$(mktemp -d -t squashfs-XXXXXXXXXX)
        loop_dir=$(mktemp -d -t loop-XXXXXXXXXX)

        # Squashfs is at ${RECOVERYDIR}/cOS/recovery.squashfs. 
        mount -t squashfs -o loop ${RECOVERYDIR}/cOS/recovery.squashfs $tmp_dir
        
        TARGET=$loop_dir
        # TODO: Size should be tweakable
        dd if=/dev/zero of=${STATEDIR}/cOS/transition.img bs=1M count=3240
        mkfs.ext2 ${STATEDIR}/cOS/transition.img -L COS_PASSIVE
        sync
        LOOP=$(losetup --show -f ${STATEDIR}/cOS/transition.img)
        mount -t ext2 $LOOP $TARGET
        rsync -aqzAX --exclude='mnt' \
        --exclude='proc' --exclude='sys' \
        --exclude='dev' --exclude='tmp' \
        $tmp_dir/ $TARGET
        ensure_dir_structure

        SELinux_relabel

        # Targets are ${STATEDIR}/cOS/active.img and ${STATEDIR}/cOS/passive.img
        umount $tmp_dir
        rm -rf $tmp_dir
        umount $TARGET
        rm -rf $TARGET

        mv -f ${STATEDIR}/cOS/transition.img ${STATEDIR}/cOS/passive.img
        sync
        copy_passive
    else
        cp -rf ${RECOVERYDIR}/cOS/recovery.img ${STATEDIR}/cOS/passive.img
        copy_passive
    fi
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