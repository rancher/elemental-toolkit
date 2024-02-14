#!/bin/bash

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

# Omit any immutable roofs module logic if disabled
if getargbool 0 elemental.disable; then
    exit 0
fi

oem_timeout=$(getargnum 120 1 1800 elemental.oemtimeout=)
oem_label=$(getarg elemental.oemlabel=)

GENERATOR_DIR="$2"
[ -z "$GENERATOR_DIR" ] && exit 1
[ -d "$GENERATOR_DIR" ] || mkdir "$GENERATOR_DIR"

oem_unit="oem.mount"

if [ -n "${oem_label}" ]; then
    dev=$(dev_unit_name /dev/disk/by-label/${oem_label})
    {
        echo "[Unit]"
        echo "DefaultDependencies=no"
        echo "Before=elemental-setup-rootfs.service"
        echo "After=dracut-initqueue.service"
        echo "Wants=dracut-initqueue.service"
        echo "PartOf=initrd.target"
        echo "[Mount]"
        echo "Where=/oem"
        echo "What=/dev/disk/by-label/${oem_label}"
        echo "Options=rw,suid,dev,exec,noauto,nouser,async"
    } > "$GENERATOR_DIR"/${oem_unit}

    mkdir -p "$GENERATOR_DIR/$dev.device.d"
    {
        echo "[Unit]"
        echo "Before=initrd-root-fs.target"
        echo "JobRunningTimeoutSec=${oem_timeout}"
    } > "$GENERATOR_DIR/$dev.device.d/timeout.conf"

    mkdir -p "$GENERATOR_DIR"/initrd-root-fs.target.wants
    ln -s "$GENERATOR_DIR"/"$dev".device \
        "$GENERATOR_DIR"/initrd-root-fs.target.wants/"$dev".device
fi

