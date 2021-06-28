#!/bin/bash
# cos_root_perm, cos_mounts and cos_overlay variables already processsed

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

function readCOSLayoutConfig {
    local mounts=()
    : "${MERGE:=true}"

    if [ "${DEBUG_RW}" = "true" ]; then
        cos_root_perm="rw"
    fi

    if [ -n "${VOLUMES}" ]; then
        for volume in ${VOLUMES}; do
            mounts+=("$(parseCOSMount ${volume})")
        done
    fi

    if [ "${MERGE}" = "true" ]; then
        if [ -n "${cos_mounts}" ]; then
            for mount in ${cos_mounts}; do
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

    if [ -e "${mount#*:}" ] && ! findmnt -rno SOURCE "${mount#*:}" > /dev/null; then
        mount -t auto "${mount#*:}" "/sysroot${mount%%:*}"
    else
        echo "Warning: ${mount#*:} already mounted or device not found" >&2
    fi
    echo "${mount#*:} ${mount%%:*} auto defaults 0 0\n"
}

#======================================
# Mount the rootfs layout
#--------------------------------------

PATH=/usr/sbin:/usr/bin:/sbin:/bin

declare cos_mounts=${cos_mounts}
declare cos_overlay=${cos_overlay}
declare cos_root_perm=${cos_root_perm}
declare overlay_base="/run/overlay"
declare rw_paths=("/etc" "/root" "/home" "/opt" "/srv" "/usr/local" "/var")
declare etc_conf="/sysroot/etc/systemd/system/etc.mount.d"
declare cos_layout="/run/cos/cos-layout.env"
declare root_fstype=$(findmnt -rno FSTYPE /sysroot)
declare root=$(findmnt -rno SOURCE /sysroot)
declare fstab
declare state_label

readCOSLayoutConfig

[ -z "${cos_overlay}" ] && exit 0

# If sysroot is already an overlay do not prepare the rw overlay
if [ "${root_fstype}" != "overlay" ]; then
    state_label=$(ls /tmp/cosloop-*)
    state_label="${state_label##/tmp/cosloop-}"
    if [ -f "/dev/disk/by-label/${state_label}" ]; then
        fstab="/dev/disk/by-label/${state_label} /run/initramfs/cos-state auto ${cos_root_perm} 0 0\n"
    fi
    fstab+="${root} / auto ${cos_root_perm} 0 0\n"
    fstab+=$(mountOverlayBase)
fi

mountpoints=($(getCOSMounts))

for mount in "${mountpoints[@]}"; do
    if [ "${mount#*:}" = "overlay" ]; then
        if [ "${root_fstype}" != "overlay" ]; then
            fstab+=$(mountOverlay "${mount%%:*}")
        fi
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

exit 0
