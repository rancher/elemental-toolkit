#!/bin/bash

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

root_part_mnt="/run/initramfs/elemental-state"

# Omit any immutable roofs module logic if disabled
if getargbool 0 elemental.disable; then
    exit 0
fi

# Omit any immutable rootfs module logic if no image path provided
elemental_image=$(getarg elemental.image=)
[ -z "${elemental_image}" ] && exit 0

[ -z "${root}" ] && root=$(getarg root=)

img=/cOS/${elemental_image}.img

root_perm="ro"

GENERATOR_DIR="$2"
[ -z "$GENERATOR_DIR" ] && exit 1
[ -d "$GENERATOR_DIR" ] || mkdir "$GENERATOR_DIR"

case "${root}" in
    LABEL=*) \
        root="${root//\//\\x2f}"
        root="/dev/disk/by-label/${root#LABEL=}"
        rootok=1 ;;
    UUID=*) \
        root="/dev/disk/by-uuid/${root#UUID=}"
        rootok=1 ;;
    /dev/*) \
        rootok=1 ;;
esac

[ "${rootok}" != "1" ] && exit 0

root_part_unit="${root_part_mnt#/}"
root_part_unit="${root_part_unit//-/\\x2d}"
root_part_unit="${root_part_unit//\//-}.mount"

state_unit=$(systemd-escape -p --suffix=mount ${root_part_mnt})

{
    echo "[Unit]"
    echo "Before=initrd-root-fs.target"
    echo "DefaultDependencies=no"
    echo "After=dracut-initqueue.service"
    echo "Wants=dracut-initqueue.service"
    echo "[Mount]"
    echo "Where=${root_part_mnt}"
    echo "What=${root}"
    echo "Options=${root_perm},suid,dev,exec,auto,nouser,async"
} > "$GENERATOR_DIR/${state_unit}"

dev=$(dev_unit_name "${root}")

mkdir -p "$GENERATOR_DIR/$dev.device.d"
{
    echo "[Unit]"
    echo "JobTimeoutSec=300"
    echo "JobRunningTimeoutSec=300"
} > "$GENERATOR_DIR/$dev.device.d/timeout.conf"

{
    echo "[Unit]"
    echo "Before=initrd-root-fs.target"
    echo "DefaultDependencies=no"
    echo "RequiresMountsFor=${root_part_mnt}"
    echo "[Mount]"
    echo "Where=/sysroot"
    echo "What=${root_part_mnt}/${img#/}"
    echo "Options=${root_perm},suid,dev,exec,auto,nouser,async"
} > "$GENERATOR_DIR"/sysroot.mount

if [ ! -e "$GENERATOR_DIR/initrd-root-fs.target.requires/sysroot.mount" ]; then
    mkdir -p "$GENERATOR_DIR"/initrd-root-fs.target.requires
    ln -s "$GENERATOR_DIR"/sysroot.mount \
        "$GENERATOR_DIR"/initrd-root-fs.target.requires/sysroot.mount
fi
