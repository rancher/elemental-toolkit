#!/bin/bash

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

sudo luet install --config $WORKDIR/luet.yaml -y --system-target $WORKDIR firmware/odroid-c2
# conv=notrunc ?
dd if=$WORKDIR/bl1.bin.hardkernel of=$image conv=fsync bs=1 count=442
dd if=$WORKDIR/bl1.bin.hardkernel of=$image conv=fsync bs=512 skip=1 seek=1
dd if=$WORKDIR/u-boot.odroidc2 of=$image conv=fsync bs=512 seek=97
