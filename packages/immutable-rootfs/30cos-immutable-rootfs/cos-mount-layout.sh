#!/bin/bash
# cos_root_perm, cos_mounts and cos_overlay variables are already processed

#======================================
# Functions
#--------------------------------------

function getOverlayMountpoints {
    local mountpoints

    for path in "${rw_paths[@]}"; do
        if ! hasMountpoint "${path}" "${cos_mounts[@]}"; then
            mountpoints+="${path}:overlay "
        fi
    done
    echo "${mountpoints}"
}

function hasMountpoint {
    local path=$1
    shift
    local mounts=("$@")
    
    for mount in "${mounts[@]}"; do
        if [ "${path}" = "${mount#*:}" ]; then
            return 0
        fi
    done
    return 1
}

function parseOverlay {
    local overlay=$1

    case "${overlay}" in
        UUID=*) \
            overlay="block:/dev/disk/by-uuid/${overlay#UUID=}"
        ;;
        LABEL=*) \
            overlay="block:/dev/disk/by-label/${overlay#LABEL=}"
        ;;
    esac
    echo "${overlay}"
}

function parseCOSMount {
    local mount=$1

    case "${mount}" in
        UUID=*) \
            mount="/dev/disk/by-uuid/${mount#UUID=}"
        ;;
        LABEL=*) \
            mount="/dev/disk/by-label/${mount#LABEL=}"
        ;;
    esac
    echo "${mount}"
}

function setupLayout {
    local o_mnt=0
    local so_mnt=0

    if [ -e "/dev/disk/by-label/${oem_label}" ]; then
        info "Mounting ${oem_mount}"
        mkdir -p "${oem_mount}"
        mount -t auto  "/dev/disk/by-label/${oem_label}" "${oem_mount}"
        o_mnt=1
    fi

    if [ -d "/sysroot/system/oem" ]; then
        ln -s /sysroot/system /system
    fi

    mkdir -p "${cos_layout%/*}"
    cos-setup rootfs

    [ "${o_mnt}" = 1 ] && umount "${oem_mount}"
}

function readCOSLayoutConfig {
    local mounts=()
    local MERGE="true"
    local VOLUMES
    local OVERLAY
    local DEBUG_RW

    [ ! -f "${cos_layout}" ] && return

    info "Loading ${cos_layout}"
    . "${cos_layout}"

    if [ "${DEBUG_RW}" = "true" ]; then
        cos_root_perm="rw"
    fi

    if [ -n "${VOLUMES}" ]; then
        for volume in ${VOLUMES}; do
            mounts+=("$(parseCOSMount ${volume})")
        done
    fi

    if [ "${MERGE}" = "true" ]; then
        if [ ${#mounts[@]} -gt 0 ]; then
            for mount in "${cos_mounts[@]}"; do
                if ! hasMountpoint "${mount#*:}" "${mounts[@]}"; then
                    mounts+=("${mount}")
                fi
            done
        fi
    fi

    if [ -n "${OVERLAY}" ]; then
        cos_overlay=$(parseOverlay "${OVERLAY}")
    fi
    if [ ${#mounts[@]} -gt 0 ]; then
        cos_mounts=("${mounts[@]}")
    else
        cos_mounts=()
    fi
}

function getCOSMounts {
    local mounts

    for mount in "${cos_mounts[@]}"; do
        mounts+="${mount#*:}:${mount%%:*} "
    done
    mounts+="$(getOverlayMountpoints)"
    echo -e "${mounts// /\\n}" | sort -
}

function mountOverlayBase {
    local fstab_line

    mkdir -p "${overlay_base}"
    if [ "${cos_overlay%%:*}" = "tmpfs" ]; then
        overlay_size="${cos_overlay#*:}"
        mount -t tmpfs -o "defaults,size=${overlay_size}" tmpfs "${overlay_base}"
        fstab_line="tmpfs ${overlay_base} tmpfs defaults,size=${overlay_size} 0 0\n"
    elif [ "${cos_overlay%%:*}" = "block" ]; then
        overlay_block="${cos_overlay#*:}"
        mount -t auto "${overlay_block}" "${overlay_base}"
        fstab_line="${overlay_block} ${overlay_base} auto defaults 0 0\n"
    fi
    echo "${fstab_line}"
}

function mountOverlay {
    local mount=$1
    local merged
    local upperdir
    local workdir
    local fstab_line

    mount="${mount#/}"
    merged="/sysroot/${mount}"
    if [ -d "${merged}" ] && ! mountpoint -q "${merged}"; then
        upperdir="${overlay_base}/${mount//\//-}.overlay/upper"
        workdir="${overlay_base}/${mount//\//-}.overlay/work"
        mkdir -p "${upperdir}" "${workdir}"
        mount -t overlay overlay -o "defaults,lowerdir=${merged},upperdir=${upperdir},workdir=${workdir}" "${merged}"
        fstab_line="overlay /${mount} overlay defaults,lowerdir=/${mount},upperdir=${upperdir},"
        fstab_line+="workdir=${workdir},x-systemd.requires-mounts-for=${overlay_base}\n"
    fi
    echo "${fstab_line}"
}

function mountPersistent {
    local mount=$1

    if [ -e "${mount#*:}" ]; then
        mount -t auto "${mount#*:}" "/sysroot${mount%%:*}"
    else
        warn "${mount#*:} not mounted, device not found"
    fi
    echo "${mount#*:} ${mount%%:*} auto defaults 0 0\n"
}

#======================================
# Mount the rootfs layout
#--------------------------------------

type info >/dev/null 2>&1 || . /lib/dracut-lib.sh
PATH=/usr/sbin:/usr/bin:/sbin:/bin

declare root=${root}
declare cos_mounts=("${cos_mounts[@]}")
declare cos_overlay=${cos_overlay}
declare oem_label="COS_OEM"
declare oem_mount="/oem"
declare overlay_base="/run/overlay"
declare rw_paths=("/etc" "/root" "/home" "/opt" "/srv" "/usr/local" "/var")
declare etc_conf="/sysroot/etc/systemd/system/etc.mount.d"
declare cos_layout="/run/cos/cos-layout.env"
declare fstab

[ ! "${root%%:*}" = "cos" ] && return 0

setupLayout

readCOSLayoutConfig

[ -z "${cos_overlay}" ] && return 0

fstab="${root#cos:} / auto ${cos_root_perm},suid,dev,exec,auto,nouser,async 0 0\n"

fstab+=$(mountOverlayBase)

mountpoints=($(getCOSMounts))

for mount in "${mountpoints[@]}"; do
    info "Mounting ${mount%%:*}"
    if [ "${mount#*:}" = "overlay" ]; then
        fstab+=$(mountOverlay "${mount%%:*}")
    else
        fstab+=$(mountPersistent "${mount}")
    fi
done

echo -e "${fstab}" > /sysroot/etc/fstab

if [ ! -f "${etc_conf}/override.conf" ]; then
    mkdir -p "${etc_conf}"
    {
        echo "[Mount]"
        echo "LazyUnmount=true"
    } > "${etc_conf}/override.conf"
fi

return 0
