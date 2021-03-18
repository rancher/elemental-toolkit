#!/bin/bash
# immutable root is specified with
# rd.cos.mount=LABEL=<vol_label>:<mountpoint>
# rd.cos.mount=UUID=<vol_uuid>:<mountpoint>
# rd.cos.overlay=tmpfs:<size>
# rd.cos.overlay=LABEL=<vol_label>
# rd.cos.overlay=UUID=<vol_uuid>
# rd.cos.oemtimeout=4
# rd.cos.debugrw

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

[ -z "${cos_overlay}" ] && cos_overlay=$(getarg rd.cos.overlay=)
[ -z "${root}" ] && root=$(getarg root=)
cos_oem_timeout=$(getarg rd.cos.oemtimeout=)
[ -z "$cos_oem_timeout" ] && cos_oem_timeout=4

cos_root_perm="ro"
if getargbool 0 rd.cos.debugrw; then
    cos_root_perm="rw"
fi

case "${root}" in
    LABEL=*) \
        root="${root//\//\\x2f}"
        root="cos:/dev/disk/by-label/${root#LABEL=}"
        rootok=1 ;;
    UUID=*) \
        root="cos:/dev/disk/by-uuid/${root#UUID=}"
        rootok=1 ;;
    /dev/*) \
        root="cos:${root}"
        rootok=1 ;;
esac

[ "${rootok}" != "1" ] && return 0

info "root device set to ${root}"

wait_for_dev -n "${root#cos:}"

case "${cos_overlay}" in
    UUID=*) \
        cos_overlay="block:/dev/disk/by-uuid/${cos_overlay#UUID=}"
    ;;
    LABEL=*) \
        cos_overlay="block:/dev/disk/by-label/${cos_overlay#LABEL=}"
    ;;
esac

info "overlay device set to ${cos_overlay}"

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

export cos_mounts cos_overlay cos_root_perm root cos_oem_timeout
