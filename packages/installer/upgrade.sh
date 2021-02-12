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
    mount $PASSIVE ${TARGET}
    mkdir -p ${TARGET}/oem || true
    mount ${OEM} ${TARGET}/oem
    mkdir -p ${TARGET}/usr/local || true
    mount ${PERSISTENT} ${TARGET}/usr/local
}

upgrade() {
    mount -t auto $PASSIVE /tmp/upgrade
    luet install -y $UPGRADE_IMAGE
    luet cleanup
}

switch_active() {
    tune2fs -L COS_ACTIVE $PASSIVE
    tune2fs -L COS_PASSIVE $ACTIVE
}

find_partitions

find_upgrade_channel

mount_image

upgrade

switch_active