#!/bin/bash
set -e
# 1. Identify active/passive partition
# 2. Install upgrade in passive partition
# 3. Invert partition labels
# 4. Update grub (?)
# 5. Reboot if requested by user (?)

find_partitions() {
    ACTIVE=$(blkid -L COS_ACTIVE || true)
    if [ -z "$ACTIVE" ]; then
        echo "Active partition cannot be found"
        exit 1
    fi
    PASSIVE=$(blkid -L COS_PASSIVE || true)
    if [ -z "$ACTIVE" ]; then
        echo "Active partition cannot be found"
        exit 1
    fi
    PERSISTENT=$(blkid -L COS_PERSISTENT || true)
    if [ -z "$PERSISTENT" ]; then
        echo "Persistent partition cannot be found"
        exit 1
    fi
    OEM=$(blkid -L COS_OEM || true)
    if [ -z "$OEM" ]; then
        echo "OEM partition cannot be found"
        exit 1
    fi

    CURRENT=$(df $0 | tail -1 | gawk '{print $1}')
    if [ -z "$CURRENT" ]; then
        echo "Could not determine current partition"
        exit 1
    fi
    if [ -z "$ACTIVE" ]; then
        echo "Could not determine active partition"
        exit 1
    fi
    if [ -z "$PASSIVE" ]; then
        echo "Could not determine passive partition"
        exit 1
    fi

    if [[ $CURRENT == $ACTIVE ]]; then
        TARGET_PARTITION=$PASSIVE
        NEW_ACTIVE=$PASSIVE
        NEW_PASSIVE=$ACTIVE
    elif [[ $CURRENT == $PASSIVE ]]; then
        # We booted from the fallback, and we are attempting to fixup the active one
        TARGET_PARTITION=$ACTIVE
        NEW_ACTIVE=$ACTIVE
        NEW_PASSIVE=$PASSIVE
    elif [ -z "$TARGET_PARTITION" ]; then
        # We booted from an ISO or some else medium. We assume we want to fixup the current label
        read -p "Could not determine current partition. Set TARGET_PARTITION, NEW_ACTIVE and NEW_PASSIVE. Otherwise assuming you want to overwrite COS_ACTIVE? [y/N] : " -n 1 -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]
        then
            [[ "$0" = "$BASH_SOURCE" ]] && exit 1 || return 1 # handle exits from shell or function but don't exit interactive shell
        fi
        TARGET_PARTITION=$ACTIVE
        NEW_ACTIVE=$ACTIVE
        NEW_PASSIVE=$PASSIVE
    fi

    if [ -z "$TARGET_PARTITION" ]; then
        echo "Could not determine target partition. Set TARGET_PARTITION, NEW_ACTIVE and NEW_PASSIVE"
        exit 1
    fi

    echo "-> Partition labeled COS_ACTIVE: $ACTIVE"
    echo "-> Partition labeled COS_PASSIVE: $PASSIVE"
    echo "-> Booting from: $CURRENT"
    echo "-> Target upgrade partition: $TARGET_PARTITION"
}

# cos-upgrade-image: system/cos
find_upgrade_channel() {
    UPGRADE_IMAGE=$(cat /etc/cos-upgrade-image)
    if [ -z "$UPGRADE_IMAGE" ]; then
        UPGRADE_IMAGE="system/cos"
        echo "Upgrade image not found in /etc/cos-upgrade-image, using $UPGRADE_IMAGE"
    fi
}

mount_image() {
    TARGET=/tmp/upgrade
    mkdir ${TARGET} || true
    mount $TARGET_PARTITION ${TARGET}
}

mount_persistent() {
    mkdir -p ${TARGET}/oem || true
    mount ${OEM} ${TARGET}/oem
    mkdir -p ${TARGET}/usr/local || true
    mount ${PERSISTENT} ${TARGET}/usr/local
    GRUB=$(blkid -L COS_GRUB || true)
    if [ -n "$GRUB" ]; then
        mkdir -p ${TARGET}/boot/efi || true
        mount ${GRUB} ${TARGET}/boot/efi
    fi
}

upgrade() {
    mount_image

    # XXX: Wipe old, needed until we have a persistent luet state.
    # TODO: at least cache downloads before wiping and we are sure we can perform the new install
    if [ -d "/tmp/empty" ]; then
        rm -rf /tmp/empty
    fi
    mkdir /tmp/empty
    rsync -a --delete /tmp/empty/ /tmp/upgrade/

    mount_persistent
    ensure_dir_structure
    # FIXME: XDG_RUNTIME_DIR is for containerd, by default that points to /run/user/<uid>
    # which might not be sufficient to unpack images. Use /usr/local/tmp until we get a separate partition
    # for the state
    # FIXME: Define default /var/tmp as tmpdir_base in default luet config file
    XDG_RUNTIME_DIR=/var/tmp TMPDIR=/var/tmp luet install -y $UPGRADE_IMAGE
    luet cleanup
    rm -rf /tmp/upgrade/var/tmp/*
}

switch_active() {
    echo "-> Flagging $NEW_ACTIVE as COS_ACTIVE"
    tune2fs -L COS_ACTIVE $NEW_ACTIVE
    echo "-> Flagging $NEW_PASSIVE as COS_PASSIVE"
    tune2fs -L COS_PASSIVE $NEW_PASSIVE
}

ensure_dir_structure() {
    mkdir ${TARGET}/proc || true
    mkdir ${TARGET}/boot || true
    mkdir ${TARGET}/dev || true
    mkdir ${TARGET}/sys || true
    mkdir ${TARGET}/tmp || true
}

update_grub() {
    if [ -e "/sys/firmware/efi" ]; then
        GRUB_TARGET="--target=x86_64-efi --efi-dir=${TARGET}/boot/efi"
    fi
    DEVICE=/dev/$(lsblk -no pkname ${TARGET_PARTITION})
    grub2-install ${GRUB_TARGET} --removable ${DEVICE}
}

cleanup2()
{
    if [ -n "${TARGET}" ]; then
        umount ${TARGET}/boot/efi || true
        umount ${TARGET}/oem || true
        umount ${TARGET}/usr/local || true
        umount ${TARGET}/ || true
    fi
}

cleanup()
{
    EXIT=$?
    cleanup2 2>/dev/null || true
    return $EXIT
}

find_partitions

find_upgrade_channel

trap cleanup exit

upgrade

switch_active

# FIXME: We have to regenerate grub since now we don't have a partition for /boot/grub2. upgrades,
# since don't have any state and wipe everything, will remove generated grub files during install
# NOTE: If we have a persistent luetdb state, this wouldnt be required, because handled by upgrades
# (no wipe needed).
update_grub

echo "Flush changes to disk"
sync
sync

if [ -n "$INTERACTIVE" ] && [ $INTERACTIVE == false ]; then
    if grep -q 'cos.upgrade.power_off=true' /proc/cmdline; then
        poweroff -f
    else
        echo " * Rebooting system in 5 seconds (CTRL+C to cancel)"
        sleep 5
        reboot -f
    fi
else
    echo "Upgrade done, now you might want to reboot"
fi