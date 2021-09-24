#!/bin/bash

prepare_chroot() {
    local dir=$1

    for mnt in /dev /dev/pts /proc /sys
    do
        mount -o bind $mnt $dir/$mnt
    done
}

cleanup_chroot() {
    local dir=$1

    for mnt in /sys /proc /dev/pts /dev
    do
        umount $dir/$mnt
    done
}

run_hook() {
    local hook=$1
    local dir=$2

    prepare_chroot $dir
    chroot $dir /usr/bin/cos-setup $hook
    cleanup_chroot $dir
}