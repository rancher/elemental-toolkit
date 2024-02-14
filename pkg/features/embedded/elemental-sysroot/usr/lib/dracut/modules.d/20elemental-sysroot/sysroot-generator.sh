#!/bin/bash

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

root_part_mnt="/run/initramfs/elemental-state"

# Omit any immutable roofs module logic if disabled
if getargbool 0 elemental.disable; then
    exit 0
fi

elemental_mode=$(getarg elemental.mode=)
elemental_img=$(getarg elemental.image=)
root=$(getarg root=)
rootok=0
snapshotter=$(getarg elemental.snapshotter=)

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

dev=$(dev_unit_name "${root}")

mkdir -p "$GENERATOR_DIR/$dev.device.d"
{
    echo "[Unit]"
    echo "JobTimeoutSec=300"
    echo "JobRunningTimeoutSec=300"
} > "$GENERATOR_DIR/$dev.device.d/timeout.conf"

if [ "${snapshotter}" == "btrfs" ]; then
    snapshots_unit=$(systemd-escape -p --suffix=mount /sysroot/.snapshots)
    rootvol_unit=$(systemd-escape -p --suffix=mount ${root_part_mnt})
    case "${elemental_mode}" in
        *active*)
            opts="ro,noatime,seclabel,compress=lzo,space_cache=v2" ;;
        *passive*)
            opts="ro,noatime,seclabel,compress=lzo,space_cache=v2,subvol=@/.snapshots/${elemental_img}/snapshot" ;;
        *)
            exit 1 ;;
    esac

    {
        echo "[Unit]"
        echo "Before=initrd-root-fs.target"
        echo "DefaultDependencies=no"
        echo "After=dracut-initqueue.service"
        echo "Wants=dracut-initqueue.service"
        echo "[Mount]"
        echo "Where=/sysroot"
        echo "What=${root}"
        echo "Options=${opts}"
    } > "$GENERATOR_DIR/sysroot.mount"

    {
        echo "[Unit]"
        echo "Before=initrd-root-fs.target"
        echo "DefaultDependencies=no"
        echo "PartOf=initrd.target"
        echo "[Mount]"
        echo "Where=${root_part_mnt}"
        echo "What=${root}"
        echo "Options=rw,noatime,seclabel,compress=lzo,space_cache=v2,subvol=@"
    } > "$GENERATOR_DIR/${rootvol_unit}"

    {
        echo "[Unit]"
        echo "Before=initrd-root-fs.target"
        echo "DefaultDependencies=no"
        echo "RequiresMountsFor=/sysroot"
        echo "PartOf=initrd.target"
        echo "[Mount]"
        echo "Where=/sysroot/.snapshots"
        echo "What=${root}"
        echo "Options=rw,noatime,seclabel,compress=lzo,space_cache=v2,subvol=@/.snapshots"
    } > "$GENERATOR_DIR/${snapshots_unit}"

    mkdir -p "$GENERATOR_DIR"/initrd-root-fs.target.wants
    ln -s "$GENERATOR_DIR/${snapshots_unit}" \
        "$GENERATOR_DIR/initrd-root-fs.target.wants/${snapshots_unit}"
    ln -s "$GENERATOR_DIR/${rootvol_unit}" \
        "$GENERATOR_DIR/initrd-root-fs.target.wants/${rootvol_unit}"
else
    state_unit=$(systemd-escape -p --suffix=mount ${root_part_mnt})
    case "${elemental_mode}" in
        *active*)
            image=".snapshots/active" ;;
        *passive*)
            image=".snapshots/${elemental_img}/snapshot" ;;
        *recovery*)
            image="recovery.img" ;;
        *)
            exit 1 ;;
    esac

    {
        echo "[Unit]"
        echo "Before=initrd-root-fs.target"
        echo "DefaultDependencies=no"
        echo "After=dracut-initqueue.service"
        echo "Wants=dracut-initqueue.service"
        echo "[Mount]"
        echo "Where=${root_part_mnt}"
        echo "What=${root}"
        echo "Options=rw,suid,dev,exec,auto,nouser,async"
    } > "$GENERATOR_DIR/${state_unit}"

    {
        echo "[Unit]"
        echo "Before=initrd-root-fs.target"
        echo "DefaultDependencies=no"
        echo "RequiresMountsFor=${root_part_mnt}"
        echo "[Mount]"
        echo "Where=/sysroot"
        echo "What=${root_part_mnt}/${image}"
        echo "Options=ro,suid,dev,exec,auto,nouser,async"
    } > "$GENERATOR_DIR"/sysroot.mount
fi

mkdir -p "$GENERATOR_DIR"/initrd-root-fs.target.requires
ln -s "$GENERATOR_DIR"/sysroot.mount \
    "$GENERATOR_DIR"/initrd-root-fs.target.requires/sysroot.mount
    