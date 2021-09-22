#!/bin/bash
set -e

source /usr/lib/cos/functions.sh

CHANNEL_UPGRADES="${CHANNEL_UPGRADES:-true}"
FORCE="${FORCE:-false}"

# 1. Identify active/passive partitions
# 2. Deploy active and passive to the same image

find_partitions() {
    STATE=$(blkid -L COS_STATE || true)
    if [ -z "$STATE" ]; then
        echo "State partition cannot be found"
        exit 1
    fi

    OEM=$(blkid -L COS_OEM || true)

    PERSISTENT=$(blkid -L COS_PERSISTENT || true)
    if [ -z "$PERSISTENT" ]; then
        echo "Persistent partition cannot be found"
        exit 1
    fi

    COS_ACTIVE=$(blkid -L COS_ACTIVE || true)
    COS_PASSIVE=$(blkid -L COS_PASSIVE || true)
    if [ -n "$COS_ACTIVE" ] || [ -n "$COS_PASSIVE" ]; then
        if [ "$FORCE" == "true" ]; then
            echo "Forcing overwrite current COS_ACTIVE and COS_PASSIVE partitions"
            return 0
        else
            echo "There is already an active deployment in the system, use '--force' flag to overwrite it"
            exit 1
        fi
   
    fi
}

# cos-upgrade-image: system/cos
find_upgrade_channel() {
    if [ -e "/etc/environment" ]; then
        source /etc/environment
    fi

    if [ -e "/etc/os-release" ]; then
        source /etc/os-release
    fi

    if [ -e "/etc/cos-upgrade-image" ]; then
        source /etc/cos-upgrade-image
    fi

    if [ -e "/etc/cos/config" ]; then
        source /etc/cos/config
    fi

    if [ -n "$NO_CHANNEL" ] && [ $NO_CHANNEL == true ]; then
        CHANNEL_UPGRADES=false
    fi

    if [ -n "$IMAGE" ]; then
        UPGRADE_IMAGE=$IMAGE
        echo "Upgrading to image $UPGRADE_IMAGE"
    fi

    if [ -z "$UPGRADE_IMAGE" ]; then
        UPGRADE_IMAGE="system/cos"
    fi
}

prepare_target() {
    if [ -f  ${STATEDIR}/cOS/active.img ] && [ "$FORCE" != "true" ]; then
        echo "There is already an active deployment in the system, use '--force' flag to overwrite it"
        exit 1
    fi

    mkdir -p ${STATEDIR}/cOS || true
    rm -rf ${STATEDIR}/cOS/active.img || true
    dd if=/dev/zero of=${STATEDIR}/cOS/active.img bs=1M count=3240
    mkfs.ext2 ${STATEDIR}/cOS/active.img
    mount -t ext2 -o loop ${STATEDIR}/cOS/active.img $TARGET
}

mount_state() {
    STATEDIR=/run/initramfs/state
    mkdir -p $STATEDIR
    mount ${STATE} ${STATEDIR}
}

is_mounted() {
    mountpoint -q "$1"
}

mount_image() {
    STATEDIR=/run/initramfs/cos-state
    TARGET=/tmp/upgrade

    mkdir -p $TARGET || true

    if [ -d "$STATEDIR" ]; then
        if [ -f /run/cos/recovery_mode ]; then
            mount_state
        else
            mount -o remount,rw ${STATE} ${STATEDIR}
        fi
    else
        mount_state
    fi

    is_mounted /usr/local || mount ${PERSISTENT} /usr/local

    prepare_target
}

upgrade() {
    ensure_dir_structure

    upgrade_state_dir="/usr/local/.cos-upgrade"
    temp_upgrade=$upgrade_state_dir/tmp/upgrade
    rm -rf $upgrade_state_dir || true
    mkdir -p $temp_upgrade

    if [ "$STRICT_MODE" = "true" ]; then
      cos-setup before-deploy
    else 
      cos-setup before-deploy || true
    fi

    # FIXME: XDG_RUNTIME_DIR is for containerd, by default that points to /run/user/<uid>
    # which might not be sufficient to unpack images. Use /usr/local/tmp until we get a separate partition
    # for the state
    # FIXME: Define default /var/tmp as tmpdir_base in default luet config file
    export XDG_RUNTIME_DIR=$temp_upgrade
    export TMPDIR=$temp_upgrade

    if [ -n "$CHANNEL_UPGRADES" ] && [ "$CHANNEL_UPGRADES" == true ]; then
        if [ -z "$VERIFY" ]; then
          args="--enable-logfile --logfile /tmp/luet.log --plugin luet-mtree"
        fi
        luet install $args --system-target $TARGET --system-engine memory -y $UPGRADE_IMAGE
        luet cleanup
    else
        args=""
        if [ -z "$VERIFY" ]; then
          args="--enable-logfile --logfile /tmp/luet.log --plugin luet-mtree"
        fi

        # unpack doesnt like when you try to unpack to a non existing dir
        mkdir -p $upgrade_state_dir/tmp/rootfs || true

        luet util unpack $args $UPGRADE_IMAGE $upgrade_state_dir/tmp/rootfs
        rsync -aqzAX --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' $upgrade_state_dir/tmp/rootfs/ $TARGET
        rm -rf $upgrade_state_dir/tmp/rootfs
    fi

    SELinux_relabel

    mount $PERSISTENT $TARGET/usr/local
    mount $OEM $TARGET/oem
    if [ "$STRICT_MODE" = "true" ]; then
        run_hook after-deploy-chroot $TARGET
    else 
        run_hook after-deploy-chroot $TARGET || true
    fi
    umount $TARGET/oem
    umount $TARGET/usr/local

    if [ "$STRICT_MODE" = "true" ]; then
      cos-setup after-deploy
    else 
      cos-setup after-deploy || true
    fi

    rm -rf $upgrade_state_dir
    umount $TARGET || true
}

SELinux_relabel()
{
    if which setfiles > /dev/null && [ -e ${TARGET}/etc/selinux/targeted/contexts/files/file_contexts ]; then
        setfiles -r ${TARGET} ${TARGET}/etc/selinux/targeted/contexts/files/file_contexts ${TARGET}
    fi
}

set_active_passive() {
    tune2fs -L COS_ACTIVE ${STATEDIR}/cOS/active.img

    cp -f ${STATEDIR}/cOS/active.img ${STATEDIR}/cOS/passive.img
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
}

ensure_dir_structure() {
    mkdir ${TARGET}/proc || true
    mkdir ${TARGET}/boot || true
    mkdir ${TARGET}/dev || true
    mkdir ${TARGET}/sys || true
    mkdir ${TARGET}/tmp || true
    mkdir ${TARGET}/usr/local || true
    mkdir ${TARGET}/oem || true
}

cleanup2()
{
    rm -rf /usr/local/tmp/upgrade || true
    mount -o remount,ro ${STATE} ${STATEDIR} || true
    if [ -n "${TARGET}" ]; then
        umount ${TARGET}/boot/efi || true
        umount ${TARGET}/ || true
        rm -rf ${TARGET}
    fi

    if [ "$STATEDIR" == "/run/initramfs/state" ]; then
        umount ${STATEDIR}
        rm -rf $STATEDIR
    fi
}

cleanup()
{
    EXIT=$?
    cleanup2 2>/dev/null || true
    return $EXIT
}

usage()
{
    echo "Usage: cos-deploy [--no-verify] [--docker-image] [--force] IMAGE"
    echo ""
    echo "Example: cos-deploy"
    echo ""
    echo "IMAGE is optional, and deployes the system to the given specified docker image."
    echo ""
    echo ""
    exit 1
}

while [ "$#" -gt 0 ]; do
    case $1 in
        --docker-image)
            NO_CHANNEL=true
            ;;
        --no-verify)
            VERIFY=false
            ;;
        --strict)
            STRICT_MODE=true
            ;;
        --force)
            FORCE=true
            ;;
        -h)
            usage
            ;;
        --help)
            usage
            ;;
        *)
            if [ "$#" -gt 2 ]; then
                usage
            fi
            IMAGE=$1
            break
            ;;
    esac
    shift 1
done

find_upgrade_channel

trap cleanup exit

echo "Deploying system.."

find_partitions

mount_image

upgrade

set_active_passive

cos-rebrand

echo "Flush changes to disk"
sync

echo "Deployment done, now you might want to reboot"
