#!/bin/bash

function doLoopMount {
    local label

    for label in "${dev_labels[@]}"; do
        [ -e "/tmp/cosloop-${label}" ] && continue
        [ -e "/dev/disk/by-label/${label}" ] || continue
        > "/tmp/cosloop-${label}" 
        mount -t auto -o "${cos_root_perm}" "/dev/disk/by-label/${label}" "${cos_state}" || continue
        if [ -f "${cos_state}/${cos_img}" ]; then
            losetup -f "${cos_state}/${cos_img}"
            exit 0
        else
            umount "${cos_state}"
        fi
    done
}

type getarg > /dev/null 2>&1 || . /lib/dracut-lib.sh

PATH=/usr/sbin:/usr/bin:/sbin:/bin

declare cos_img=$1
declare cos_root_perm="ro"
declare cos_state="/run/initramfs/cos-state"
declare dev_labels=("COS_STATE" "COS_RECOVERY")

[ -z "${cos_img}" ] && exit 1

if getargbool 0 rd.cos.debugrw; then
    cos_root_perm="rw"
fi

ismounted "${cos_state}" && exit 0

mkdir -p "${cos_state}"

doLoopMount

rm -r "${cos_state}"
exit 1
