#!/bin/bash

partprobe

kpartx -va $DRIVE

image=$1

if [ -z "$image" ]; then
    echo "No image specified"
    exit 1
fi

if [ ! -e "$WORKDIR/luet.yaml" ]; then
    ls -liah $WORKDIR
    echo "No valid config file"
    cat "$WORKDIR/luet.yaml"
    exit 1
fi
set -ax
TEMPDIR="$(mktemp -d)"
echo $TEMPDIR
mount "${device}p1" "${TEMPDIR}"
sudo luet install --config $WORKDIR/luet.yaml -y --system-target $TEMPDIR firmware/u-boot-rpi64
sudo luet install --config $WORKDIR/luet.yaml -y --system-target $TEMPDIR firmware/raspberrypi-firmware
sudo luet install --config $WORKDIR/luet.yaml -y --system-target $TEMPDIR firmware/raspberrypi-firmware-config
sudo luet install --config $WORKDIR/luet.yaml -y --system-target $TEMPDIR firmware/raspberrypi-firmware-dt
umount "${TEMPDIR}"
