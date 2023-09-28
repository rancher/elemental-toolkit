type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

if getargbool 0 rd.cos.disable; then
    return 0
fi

if getargbool 0 elemental.disable; then
    return 0
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

