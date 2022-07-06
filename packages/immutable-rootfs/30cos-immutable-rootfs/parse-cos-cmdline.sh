#!/bin/bash
# immutable root is specified with
# rd.cos.mount=LABEL=<vol_label>:<mountpoint>
# rd.cos.mount=UUID=<vol_uuid>:<mountpoint>
# rd.cos.overlay=tmpfs:<size>
# rd.cos.overlay=LABEL=<vol_label>
# rd.cos.overlay=UUID=<vol_uuid>
# rd.cos.oemtimeout=<seconds>
# rd.cos.oemlabel=<vol_label>
# rd.cos.debugrw
# rd.cos.disable
# cos-img/filename=/cOS/active.img

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

cos_root="/run/cos/root"

if getargbool 0 rd.cos.disable; then
    return 0
fi

cos_img=$(getarg cos-img/filename=)
[ -z "${cos_img}" ] && return 0

mkdir -p "${cos_root}"

wait_for_mount "${cos_root}"
/sbin/initqueue --settled --unique /sbin/cos-root-mnt "${cos_img}"

# set sentinel file for boot mode
case "${cos_img}" in
    *recovery*)
        echo 1 > /run/cos/recovery_mode ;;
    *active*)
        echo 1 > /run/cos/active_mode ;;
    *passive*)
        echo 1 > /run/cos/passive_mode ;;
esac

return 0
