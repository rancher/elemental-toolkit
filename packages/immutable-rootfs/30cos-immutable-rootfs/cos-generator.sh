#!/bin/bash

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

[ -z "${root}" ] && root=$(getarg root=)

root_perm="ro"
if getargbool 0 rd.cos.debug.rw; then
    root_perm="rw"
fi

oem_timeout=$(getarg rd.cos.oemtimeout=)
[ -z "${oem_timeout}" ] && oem_timeout="10"

GENERATOR_DIR="$2"
[ -z "$GENERATOR_DIR" ] && exit 1
[ -d "$GENERATOR_DIR" ] || mkdir "$GENERATOR_DIR"

dev=$(dev_unit_name /dev/disk/by-label/COS_OEM)

mkdir -p "$GENERATOR_DIR/$dev.device.d"
{
    echo "[Unit]"
    echo "JobRunningTimeoutSec=${oem_timeout}"
} > "$GENERATOR_DIR/$dev.device.d/timeout.conf"

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
{
    echo "[Unit]"
    echo "Before=initrd-root-fs.target"
    echo "DefaultDependencies=no"
    echo "[Mount]"
    echo "Where=/sysroot"
    echo "What=${root}"
    echo "Options=${root_perm},suid,dev,exec,auto,nouser,async"
} > "$GENERATOR_DIR"/sysroot.mount

if [ ! -e "$GENERATOR_DIR/initrd-root-fs.target.requires/sysroot.mount" ]; then
    mkdir -p "$GENERATOR_DIR"/initrd-root-fs.target.requires
    ln -s "$GENERATOR_DIR"/sysroot.mount \
        "$GENERATOR_DIR"/initrd-root-fs.target.requires/sysroot.mount
fi

mkdir -p "$GENERATOR_DIR/$dev.device.d"
{
    echo "[Unit]"
    echo "JobTimeoutSec=300"
    echo "JobRunningTimeoutSec=300"
} > "$GENERATOR_DIR/$dev.device.d/timeout.conf"
