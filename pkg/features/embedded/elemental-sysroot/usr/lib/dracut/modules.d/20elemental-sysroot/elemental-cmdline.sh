type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

if getargbool 0 rd.cos.disable; then
    return 0
fi

if getargbool 0 elemental.disable; then
    return 0
fi

case "${root}" in
    LABEL=*) \
        root="${root//\//\\x2f}"
        root="/dev/disk/by-label/${root#LABEL=}"
        rootok=1 ;;
    UUID=*) \
        root="/dev/disk/by-uuid/${root#UUID=}"
        rootok=1 ;;
    /dev/*) \
        root="${root}"
        rootok=1 ;;
esac

[ "${rootok}" != "1" ] && return 0

info "root device set to root=${root}"

wait_for_dev -n "${root#block:}"

# Only run filesystem checks on force mode
fsck_mode=$(getarg fsck.mode=)
if [ "${fsck_mode}" == "force" ]; then
    /sbin/initqueue --finished --unique /sbin/elemental-fsck
fi

elemental_img=$(getarg elemental.image=)
mkdir -p /run/elemental
case "${elemental_img}" in
    *recovery*)
        echo -n 1 > /run/elemental/recovery_mode ;;
    *active*)
        echo -n 1 > /run/elemental/active_mode ;;
    *passive*)
        echo -n 1 > /run/elemental/passive_mode ;;
esac

cos_img=$(getarg cos-img/filename=)
[ -z "${cos_img}" ] && return 0

# set sentinel file for boot mode
mkdir -p /run/cos
case "${cos_img}" in
    *recovery*)
        echo -n 1 > /run/cos/recovery_mode ;;
    *active*)
        echo -n 1 > /run/cos/active_mode ;;
    *passive*)
        echo -n 1 > /run/cos/passive_mode ;;
esac

return 0
