#!/bin/bash
# immutable root is specified with
# rd.cos.mount=LABEL=<vol_label>:<mountpoint>
# rd.cos.mount=UUID=<vol_uuid>:<mountpoint>
# rd.cos.overlay=tmpfs:<size>
# rd.cos.overlay=LABEL=<vol_label>
# rd.cos.overlay=UUID=<vol_uuid>
# rd.cos.oemtimeout=<seconds>
# rd.cos.debugrw
# cos-img/filename=/cOS/active.img

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

cos_unit="cos-immutable-rootfs.service"
cos_layout="/run/cos/cos-layout.env"

# Disable the service unless we override it
mkdir -p "/run/systemd/system/${cos_unit}.d"

cos_img=$(getarg cos-img/filename=)
[ -z "${cos_img}" ] && return 0
cos_overlay=$(getarg rd.cos.overlay=)
[ -z "${cos_overlay}" ] && cos_overlay="tmpfs:20%"
[ -z "${root}" ] && root=$(getarg root=)

cos_root_perm="ro"
if getargbool 0 rd.cos.debugrw; then
    cos_root_perm="rw"
fi

case "${root}" in
    LABEL=*) \
        root="${root//\//\\x2f}"
        root="/dev/disk/by-label/${root#LABEL=}"
        rootok=1 ;;
    UUID=*) \
        root="cos:/dev/disk/by-uuid/${root#UUID=}"
        rootok=1 ;;
    /dev/*) \
        root="${root}"
        rootok=1 ;;
esac

[ "${rootok}" != "1" ] && return 0

info "root device set to root=${root}"

wait_for_dev -n "${root}"
/sbin/initqueue --settled --unique /sbin/cos-loop-img "${cos_img}"

case "${cos_overlay}" in
    UUID=*) \
        cos_overlay="block:/dev/disk/by-uuid/${cos_overlay#UUID=}"
    ;;
    LABEL=*) \
        cos_overlay="block:/dev/disk/by-label/${cos_overlay#LABEL=}"
    ;;
esac

cos_mounts=()
for mount in $(getargs rd.cos.mount=); do
    case "${mount}" in
        UUID=*) \
            mount="/dev/disk/by-uuid/${mount#UUID=}"
        ;;
        LABEL=*) \
            mount="/dev/disk/by-label/${mount#LABEL=}"
        ;;
    esac
    cos_mounts+=("${mount}")
done

mkdir -p "${cos_layout%/*}"
> "${cos_layout}"

{
    echo "[Service]"
    echo "Environment=\"cos_mounts=${cos_mounts[@]}\""
    echo "Environment=\"cos_overlay=${cos_overlay}\""
    echo "Environment=\"cos_root_perm=${cos_root_perm}\""
    echo "Environment=\"root=${root}\""
    echo "EnvironmentFile=/run/cos/cos-layout.env"
} > "/run/systemd/system/${cos_unit}.d/override.conf"

return 0
