#!/bin/bash
set -e

PROG=$0

## Installer
PROGS="dd curl mkfs.ext4 mkfs.vfat fatlabel parted partprobe grub2-install grub2-editenv"
DISTRO=/run/rootfsbase
ISOMNT=/run/initramfs/live
ISOBOOT=${ISOMNT}/boot
TARGET=/run/cos/target
RECOVERYDIR=/run/cos/recovery
RECOVERYSQUASHFS=${ISOMNT}/recovery.squashfs
GRUBCONF=/etc/cos/grub.cfg

# Default size (in MB) of disk image files (.img) created during upgrades
DEFAULT_IMAGE_SIZE=3240

## cosign signatures
COSIGN_REPOSITORY="${COSIGN_REPOSITORY:-raccos/releases-:FLAVOR:}"
COSIGN_EXPERIMENTAL="${COSIGN_EXPERIMENTAL:-1}"
COSIGN_PUBLIC_KEY_LOCATION="${COSIGN_PUBLIC_KEY_LOCATION:-}"

## Upgrades
CHANNEL_UPGRADES="${CHANNEL_UPGRADES:-true}"
ARCH=$(uname -p)
if [ "${ARCH}" == "aarch64" ]; then
  ARCH="arm64"
fi

ARCH=$(uname -p)

if [ "${ARCH}" == "aarch64" ]; then
  ARCH="arm64"
fi

if [ "$COS_DEBUG" = true ]; then
    set -x
fi

## COMMON

load_config() {
    if [ -e /etc/environment ]; then
        source /etc/environment
    fi

    if [ -e /etc/os-release ]; then
        source /etc/os-release
    fi

    if [ -e /etc/cos/config ]; then
        source /etc/cos/config
    fi
}

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


is_mounted() {
    mountpoint -q "$1"
}

is_booting_from_squashfs() {
    if cat /proc/cmdline | grep -q "COS_RECOVERY"; then
        return 0
    else
        return 1
    fi
}

prepare_deploy_target() {
    if [ -f  ${STATEDIR}/cOS/active.img ] && [ "$FORCE" != "true" ]; then
        echo "There is already an active deployment in the system, use '--force' flag to overwrite it"
        exit 1
    fi

    mkdir -p ${STATEDIR}/cOS || true
    rm -rf ${STATEDIR}/cOS/active.img || true
    dd if=/dev/zero of=${STATEDIR}/cOS/active.img bs=1M count=$DEFAULT_IMAGE_SIZE
    mkfs.ext2 ${STATEDIR}/cOS/active.img
    mount -t ext2 -o loop ${STATEDIR}/cOS/active.img $TARGET
}

prepare_target() {
    mkdir -p ${STATEDIR}/cOS || true
    rm -rf ${STATEDIR}/cOS/transition.img || true
    dd if=/dev/zero of=${STATEDIR}/cOS/transition.img bs=1M count=$DEFAULT_IMAGE_SIZE
    mkfs.ext2 ${STATEDIR}/cOS/transition.img
    mount -t ext2 -o loop ${STATEDIR}/cOS/transition.img $TARGET
}

usage()
{
    echo "Usage: $PROG install|deploy|upgrade|reset|rebrand [options]"
    echo ""
    echo "Example: $PROG-install /dev/vda"
    echo "  install:"
    echo "  [--partition-layout /path/to/config/file.yaml ] [--force-efi] [--force-gpt] [--iso https://.../OS.iso] [--debug] [--tty TTY] [--poweroff] [--no-format] [--config https://.../config.yaml] DEVICE"
    echo ""
    echo "  upgrade:"
    echo "  [--strict] [--recovery] [--no-verify] [--no-cosign] [--directory] [--docker-image] (IMAGE/DIRECTORY)"
    echo ""
    echo "  deploy:"
    echo "  [--strict] [--no-verify] [--no-cosign] [--force] [--docker-image] (IMAGE)"
    echo ""
    echo "   DEVICE must be the disk that will be partitioned (/dev/vda). If you are using --no-format it should be the device of the COS_STATE partition (/dev/vda2)"
    echo "   IMAGE must be a container image if --docker-image is specified"
    echo "   DIRECTORY must be passed if --directory is specified"
    echo ""
    echo "The parameters names refer to the same names used in the cmdline, refer to README.md for"
    echo "more info."
    echo ""
    exit 1
}

## END COMMON

## INSTALLER
umount_target() {
    sync
    umount ${TARGET}/oem
    umount ${TARGET}/usr/local
    umount ${TARGET}/boot/efi || true
    umount ${TARGET}
    if [ -n "$LOOP" ]; then
        losetup -d $LOOP
    fi
}

installer_cleanup2()
{
    sync
    umount_target || true
    umount ${STATEDIR}
    umount ${RECOVERYDIR}
    [ -n "$COS_INSTALL_ISO_URL" ] && umount ${ISOMNT} || true
}

installer_cleanup()
{
    EXIT=$?
    installer_cleanup2 2>/dev/null || true
    return $EXIT
}

prepare_recovery() {
    echo "Preparing recovery.."
    mkdir -p $RECOVERYDIR
    mount $RECOVERY $RECOVERYDIR
    mkdir -p $RECOVERYDIR/cOS

    if [ -e "$RECOVERYSQUASHFS" ]; then
        echo "Copying squashfs.."
        cp -a $RECOVERYSQUASHFS $RECOVERYDIR/cOS/recovery.squashfs
    else
        echo "Copying image file.."
        cp -a $STATEDIR/cOS/active.img $RECOVERYDIR/cOS/recovery.img
        sync
        tune2fs -L COS_SYSTEM $RECOVERYDIR/cOS/recovery.img
    fi

    sync
}

prepare_passive() {
    echo "Preparing passive boot.."
    cp -a ${STATEDIR}/cOS/active.img ${STATEDIR}/cOS/passive.img
    sync
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
    sync
}

part_probe() {
    local dev=$1
    partprobe ${dev} 2>/dev/null || true

    sync
    sleep 2

    dmsetup remove_all 2>/dev/null || true
}

blkid_probe() {
    OEM=$(blkid -L COS_OEM || true)
    STATE=$(blkid -L COS_STATE || true)
    RECOVERY=$(blkid -L COS_RECOVERY || true)
    BOOT=$(blkid -L COS_GRUB || true)
    PERSISTENT=$(blkid -L COS_PERSISTENT || true)
}

do_format()
{
    if [ "$COS_INSTALL_NO_FORMAT" = "true" ]; then
        STATE=$(blkid -L COS_STATE || true)
        if [ -z "$STATE" ] && [ -n "$DEVICE" ]; then
            tune2fs -L COS_STATE $DEVICE
        fi
        blkid_probe
        return 0
    fi

    echo "Formatting drives.."

    if [ -n "$COS_PARTITION_LAYOUT" ] && [ "$PARTTABLE" != "gpt" ]; then
        echo "Custom layout only available with GPT based installations"
        exit 1
    fi

    dd if=/dev/zero of=${DEVICE} bs=1M count=1
    parted -s ${DEVICE} mklabel ${PARTTABLE}

    # Partitioning via cloud-init config file
    if [ -n "$COS_PARTITION_LAYOUT" ] && [ "$PARTTABLE" = "gpt" ]; then
        if [ "$BOOTFLAG" == "esp" ]; then
            parted -s ${DEVICE} mkpart primary fat32 0% 50MB # efi
            parted -s ${DEVICE} set 1 ${BOOTFLAG} on
            PREFIX=${DEVICE}
            if [ ! -e ${PREFIX}${STATE_NUM} ]; then
                PREFIX=${DEVICE}p
            fi
            BOOT=${PREFIX}1
            mkfs.vfat -F 32 ${BOOT}
            fatlabel ${BOOT} COS_GRUB
        elif [ "$BOOTFLAG" == "bios_grub" ]; then
            parted -s ${DEVICE} mkpart primary 0% 1MB # BIOS boot partition for GRUB
            parted -s ${DEVICE} set 1 ${BOOTFLAG} on
        fi

        yip -s partitioning $COS_PARTITION_LAYOUT

        part_probe $DEVICE

        blkid_probe

        return 0
    fi

    # Standard partitioning
    if [ "$PARTTABLE" = "gpt" ] && [ "$BOOTFLAG" == "esp" ]; then
        BOOT_NUM=1
        OEM_NUM=2
        STATE_NUM=3
        RECOVERY_NUM=4
        PERSISTENT_NUM=5
        parted -s ${DEVICE} mkpart primary fat32 0% 50MB # efi
        parted -s ${DEVICE} mkpart primary ext4 50MB 100MB # oem
        parted -s ${DEVICE} mkpart primary ext4 100MB 15100MB # state
        parted -s ${DEVICE} mkpart primary ext4 15100MB 23100MB # recovery
        parted -s ${DEVICE} mkpart primary ext4 23100MB 100% # persistent
        parted -s ${DEVICE} set 1 ${BOOTFLAG} on
    elif [ "$PARTTABLE" = "gpt" ] && [ "$BOOTFLAG" == "bios_grub" ]; then
        BOOT_NUM=
        OEM_NUM=2
        STATE_NUM=3
        RECOVERY_NUM=4
        PERSISTENT_NUM=5
        parted -s ${DEVICE} mkpart primary 0% 1MB # BIOS boot partition for GRUB
        parted -s ${DEVICE} mkpart primary ext4 1MB 51MB # oem
        parted -s ${DEVICE} mkpart primary ext4 51MB 15051MB # state
        parted -s ${DEVICE} mkpart primary ext4 15051MB 23051MB # recovery
        parted -s ${DEVICE} mkpart primary ext4 23051MB 100% # persistent
        parted -s ${DEVICE} set 1 ${BOOTFLAG} on
    else
        BOOT_NUM=
        OEM_NUM=1
        STATE_NUM=2
        RECOVERY_NUM=3
        PERSISTENT_NUM=4
        parted -s ${DEVICE} mkpart primary ext4 0% 50MB # oem
        parted -s ${DEVICE} mkpart primary ext4 50MB 15050MB # state
        parted -s ${DEVICE} mkpart primary ext4 15050MB 23050MB # recovery
        parted -s ${DEVICE} mkpart primary ext4 23050MB 100% # persistent
        parted -s ${DEVICE} set 2 ${BOOTFLAG} on
    fi

    part_probe $DEVICE

    PREFIX=${DEVICE}
    if [ ! -e ${PREFIX}${STATE_NUM} ]; then
        PREFIX=${DEVICE}p
    fi

    if [ ! -e ${PREFIX}${STATE_NUM} ]; then
        echo Failed to find ${PREFIX}${STATE_NUM} or ${DEVICE}${STATE_NUM} to format
        exit 1
    fi

    if [ -n "${BOOT_NUM}" ]; then
        BOOT=${PREFIX}${BOOT_NUM}
    fi
    STATE=${PREFIX}${STATE_NUM}
    OEM=${PREFIX}${OEM_NUM}
    RECOVERY=${PREFIX}${RECOVERY_NUM}
    PERSISTENT=${PREFIX}${PERSISTENT_NUM}

    mkfs.ext4 -F -L COS_STATE ${STATE}
    if [ -n "${BOOT}" ]; then
        mkfs.vfat -F 32 ${BOOT}
        fatlabel ${BOOT} COS_GRUB
    fi

    mkfs.ext4 -F -L COS_RECOVERY ${RECOVERY}
    mkfs.ext4 -F -L COS_OEM ${OEM}
    mkfs.ext4 -F -L COS_PERSISTENT ${PERSISTENT}
}

do_mount()
{
    echo "Mounting critical endpoints.."

    mkdir -p ${TARGET}

    STATEDIR=/tmp/mnt/STATE
    mkdir -p $STATEDIR || true
    mount ${STATE} $STATEDIR

    mkdir -p ${STATEDIR}/cOS
    # TODO: Size should be tweakable
    dd if=/dev/zero of=${STATEDIR}/cOS/active.img bs=1M count=$DEFAULT_IMAGE_SIZE
    mkfs.ext2 ${STATEDIR}/cOS/active.img -L COS_ACTIVE
    sync
    LOOP=$(losetup --show -f ${STATEDIR}/cOS/active.img)
    mount -t ext2 $LOOP $TARGET

    mkdir -p ${TARGET}/boot
    if [ -n "${BOOT}" ]; then
        mkdir -p ${TARGET}/boot/efi
        mount ${BOOT} ${TARGET}/boot/efi
    fi

    mkdir -p ${TARGET}/oem
    mount ${OEM} ${TARGET}/oem
    mkdir -p ${TARGET}/usr/local
    mount ${PERSISTENT} ${TARGET}/usr/local
}

get_url()
{
    FROM=$1
    TO=$2
    case $FROM in
        ftp*|http*|tftp*)
            n=0
            attempts=5
            until [ "$n" -ge "$attempts" ]
            do
                curl -o $TO -fL ${FROM} && break
                n=$((n+1))
                echo "Failed to download, retry attempt ${n} out of ${attempts}"
                sleep 2
            done
            ;;
        *)
            cp -f $FROM $TO
            ;;
    esac
}

get_iso()
{
    if [ -n "$COS_INSTALL_ISO_URL" ]; then
        ISOMNT=$(mktemp -d -p /tmp cos.XXXXXXXX.isomnt)
        TEMP_FILE=$(mktemp -p /tmp cos.XXXXXXXX.iso)
        get_url ${COS_INSTALL_ISO_URL} ${TEMP_FILE}
        ISO_DEVICE=$(losetup --show -f $TEMP_FILE)
        mount -o ro ${ISO_DEVICE} ${ISOMNT}
    fi
}

do_copy()
{
    echo "Copying cOS.."

    rsync -aqAX --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' ${DISTRO}/ ${TARGET}
     if [ -n "$COS_INSTALL_CONFIG_URL" ]; then
        OEM=${TARGET}/oem/99_custom.yaml
        get_url "$COS_INSTALL_CONFIG_URL" $OEM
        chmod 600 ${OEM}
    fi
}

SELinux_relabel()
{
    if which setfiles > /dev/null && [ -e ${TARGET}/etc/selinux/targeted/contexts/files/file_contexts ]; then
        setfiles -r ${TARGET} ${TARGET}/etc/selinux/targeted/contexts/files/file_contexts ${TARGET}
    fi
}

install_grub()
{
    echo "Installing GRUB.."

    if [ "$COS_INSTALL_DEBUG" ]; then
        GRUB_DEBUG="cos.debug"
    fi

    if [ -z "${COS_INSTALL_TTY}" ]; then
        TTY=$(tty | sed 's!/dev/!!')
    else
        TTY=$COS_INSTALL_TTY
    fi

    if [ "$COS_INSTALL_NO_FORMAT" = "true" ]; then
        return 0
    fi

    if [ "$COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        GRUB_TARGET="--target=${ARCH}-efi --efi-directory=${TARGET}/boot/efi"
    fi

    mkdir ${TARGET}/proc || true
    mkdir ${TARGET}/dev || true
    mkdir ${TARGET}/sys || true
    mkdir ${TARGET}/tmp || true

    grub2-install ${GRUB_TARGET} --root-directory=${TARGET}  --boot-directory=${STATEDIR} --removable ${DEVICE}

    GRUBDIR=
    if [ -d "${STATEDIR}/grub" ]; then
        GRUBDIR="${STATEDIR}/grub"
    elif [ -d "${STATEDIR}/grub2" ]; then
        GRUBDIR="${STATEDIR}/grub2"
    fi

    cp -rf $GRUBCONF $GRUBDIR/grub.cfg

    if [ -e "/dev/${TTY%,*}" ] && [ "$TTY" != tty1 ] && [ "$TTY" != console ] && [ -n "$TTY" ]; then
        sed -i "s!console=tty1!console=tty1 console=${TTY}!g" $GRUBDIR/grub.cfg
    fi
}

setup_style()
{
    if [ "$COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        PARTTABLE=gpt
        BOOTFLAG=esp
        if [ ! -e /sys/firmware/efi ]; then
            echo WARNING: installing EFI on to a system that does not support EFI
        fi
    elif [ "$COS_INSTALL_FORCE_GPT" = "true" ]; then
        PARTTABLE=gpt
        BOOTFLAG=bios_grub
    else
        PARTTABLE=msdos
        BOOTFLAG=boot
    fi
}

validate_progs()
{
    for i in $PROGS; do
        if [ ! -x "$(which $i)" ]; then
            MISSING="${MISSING} $i"
        fi
    done

    if [ -n "${MISSING}" ]; then
        echo "The following required programs are missing for installation: ${MISSING}"
        exit 1
    fi
}

validate_device()
{
    DEVICE=$COS_INSTALL_DEVICE
    if [ ! -b ${DEVICE} ]; then
        echo "You should use an available device. Device ${DEVICE} does not exist."
        exit 1
    fi
}

## END INSTALLER

## UPGRADER

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
    if [ -n "$COS_ACTIVE" ]; then
        CURRENT=active.img
    fi

    COS_PASSIVE=$(blkid -L COS_PASSIVE || true)
    if [ -n "$COS_PASSIVE" ]; then
        CURRENT=passive.img
    fi

    if [ -z "$CURRENT" ]; then
        # We booted from an ISO or some else medium. We assume we want to fixup the current label
        read -p "Could not determine current partition. Do you want to overwrite your current active partition? (CURRENT=active.img) [y/N] : " -n 1 -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]
        then
            [[ "$0" = "$BASH_SOURCE" ]] && exit 1 || return 1 # handle exits from shell or function but don't exit interactive shell
        fi
        CURRENT=active.img
        echo
    fi

    echo "-> Upgrade target: $CURRENT"
}

find_recovery() {
    RECOVERY=$(blkid -L COS_RECOVERY || true)
    if [ -z "$RECOVERY" ]; then
        echo "COS_RECOVERY partition cannot be found"
        exit 1
    fi
}

# cos-upgrade-image: system/cos
find_upgrade_channel() {

    load_config

    if [ -e "/etc/cos-upgrade-image" ]; then
        source /etc/cos-upgrade-image
    fi

    if [ -n "$NO_CHANNEL" ] && [ $NO_CHANNEL == true ]; then
        CHANNEL_UPGRADES=false
    fi

    if [ -n "$COS_IMAGE" ]; then
        UPGRADE_IMAGE=$COS_IMAGE
        echo "Upgrading to image $UPGRADE_IMAGE"
    else

        if [ -z "$UPGRADE_IMAGE" ]; then
            UPGRADE_IMAGE="system/cos"
        fi

        if [ -n "$UPGRADE_RECOVERY" ] && [ $UPGRADE_RECOVERY == true ] && [ -n "$RECOVERY_IMAGE" ]; then
            UPGRADE_IMAGE=$RECOVERY_IMAGE
        fi
    fi

    # export cosign values after loading values from file in case we have them setup there
    export COSIGN_REPOSITORY=$COSIGN_REPOSITORY
    export COSIGN_EXPERIMENTAL=$COSIGN_EXPERIMENTAL
    export COSIGN_PUBLIC_KEY_LOCATION=$COSIGN_PUBLIC_KEY_LOCATION
}

is_squashfs() {
    if [ -e "${STATEDIR}/cOS/recovery.squashfs" ]; then
        return 0
    else
        return 1
    fi
}

recovery_boot() {
    cmdline="$(cat /proc/cmdline)"
    if echo $cmdline | grep -q "COS_RECOVERY" || echo $cmdline | grep -q "COS_SYSTEM"; then
        return 0
    else
        return 1
    fi
}

prepare_squashfs_target() {
    rm -rf $TARGET || true
    TARGET=${STATEDIR}/tmp/target
    mkdir -p $TARGET
}

mount_state() {
    STATEDIR=/run/initramfs/state
    mkdir -p $STATEDIR
    mount ${STATE} ${STATEDIR}
}

prepare_statedir() {
    local target=$1
    case $target in
    recovery)
        STATEDIR=/tmp/recovery
        TARGET=/tmp/upgrade

        mkdir -p $TARGET || true
        mkdir -p $STATEDIR || true
        mount $RECOVERY $STATEDIR
        if is_squashfs; then
            echo "Preparing squashfs target"
            prepare_squashfs_target
        else
            echo "Preparing image target"
            prepare_target
        fi
        ;;
    *)
        STATEDIR=/run/initramfs/cos-state
        TARGET=/tmp/upgrade

        mkdir -p $TARGET || true

        if [ -d "$STATEDIR" ]; then
            if recovery_boot; then
                mount_state
            else
                mount -o remount,rw ${STATE} ${STATEDIR}
            fi
        else
            mount_state
        fi
        ;;
    esac
}

mount_image() {
    local target=$1

    prepare_statedir $target

    case $target in
    upgrade)
        prepare_target
        ;;
    deploy)
        is_mounted /usr/local || mount ${PERSISTENT} /usr/local
        prepare_deploy_target
        ;;
    esac
}

switch_active() {
    if [[ "$CURRENT" == "active.img" ]]; then
        mv -f ${STATEDIR}/cOS/$CURRENT ${STATEDIR}/cOS/passive.img
        tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
    fi

    mv -f ${STATEDIR}/cOS/transition.img ${STATEDIR}/cOS/active.img
    tune2fs -L COS_ACTIVE ${STATEDIR}/cOS/active.img
}

switch_recovery() {
    if is_squashfs; then
        if [[ "${ARCH}" == "arm64" ]]; then
          XZ_FILTER="arm"
        else
          XZ_FILTER="x86"
        fi
        mksquashfs $TARGET ${STATEDIR}/cOS/transition.squashfs -b 1024k -comp xz -Xbcj ${XZ_FILTER}
        mv ${STATEDIR}/cOS/transition.squashfs ${STATEDIR}/cOS/recovery.squashfs
        rm -rf $TARGET
    else
        mv -f ${STATEDIR}/cOS/transition.img ${STATEDIR}/cOS/recovery.img
        tune2fs -L COS_SYSTEM ${STATEDIR}/cOS/recovery.img
    fi
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

do_upgrade() {
    ensure_dir_structure
    hook_name=$1
    upgrade_state_dir="/usr/local/.cos-upgrade"
    temp_upgrade=$upgrade_state_dir/tmp/upgrade
    rm -rf $upgrade_state_dir || true
    mkdir -p $temp_upgrade


    if [ "$STRICT_MODE" = "true" ]; then
      cos-setup before-$hook_name
    else 
      cos-setup before-$hook_name || true
    fi

    # FIXME: XDG_RUNTIME_DIR is for containerd, by default that points to /run/user/<uid>
    # which might not be sufficient to unpack images. Use /usr/local/tmp until we get a separate partition
    # for the state
    # FIXME: Define default /var/tmp as tmpdir_base in default luet config file
    export XDG_RUNTIME_DIR=$temp_upgrade
    export TMPDIR=$temp_upgrade

    args=""
    if [ -z "$VERIFY" ] || [ "$VERIFY" == true ]; then
        args="--plugin luet-mtree"
    fi

    if [ -z "$COSIGN" ]; then
      args+=" --plugin luet-cosign"
    fi

    if [ -n "$CHANNEL_UPGRADES" ] && [ "$CHANNEL_UPGRADES" == true ]; then
        echo "Upgrading from release channel"
        set -x
        luet install --enable-logfile --logfile /tmp/luet.log $args --system-target $TARGET --system-engine memory -y $UPGRADE_IMAGE
        luet cleanup
        set +x
    elif [ "$DIRECTORY" == true ]; then
        echo "Upgrading from local folder: $UPGRADE_IMAGE"
        rsync -axq --exclude='host' --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' ${UPGRADE_IMAGE}/ $TARGET
    else
        echo "Upgrading from container image: $UPGRADE_IMAGE"
        set -x
        # unpack doesnt like when you try to unpack to a non existing dir
        mkdir -p $upgrade_state_dir/tmp/rootfs || true
        luet util unpack --enable-logfile --logfile /tmp/luet.log $args $UPGRADE_IMAGE $upgrade_state_dir/tmp/rootfs
        set +x
        rsync -aqzAX --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' $upgrade_state_dir/tmp/rootfs/ $TARGET
        rm -rf $upgrade_state_dir/tmp/rootfs
    fi

    chmod 755 $TARGET
    SELinux_relabel

    mount $PERSISTENT $TARGET/usr/local
    mount $OEM $TARGET/oem
    if [ "$STRICT_MODE" = "true" ]; then
        run_hook after-$hook_name-chroot $TARGET
    else 
        run_hook after-$hook_name-chroot $TARGET || true
    fi
    umount $TARGET/oem
    umount $TARGET/usr/local

    if [ "$STRICT_MODE" = "true" ]; then
      cos-setup after-$hook_name
    else 
      cos-setup after-$hook_name || true
    fi

    rm -rf $upgrade_state_dir
    umount $TARGET || true
}

upgrade_cleanup2()
{
    rm -rf /usr/local/tmp/upgrade || true
    mount -o remount,ro ${STATE} ${STATEDIR} || true
    if [ -n "${TARGET}" ]; then
        umount ${TARGET}/boot/efi || true
        umount ${TARGET}/ || true
        rm -rf ${TARGET}
    fi
    if [ -n "$UPGRADE_RECOVERY" ] && [ $UPGRADE_RECOVERY == true ]; then
	    umount ${STATEDIR} || true
    fi
    if [ "$STATEDIR" == "/run/initramfs/state" ]; then
        umount ${STATEDIR}
        rm -rf $STATEDIR
    fi
}

upgrade_cleanup()
{
    EXIT=$?
    upgrade_cleanup2 2>/dev/null || true
    return $EXIT
}


## END UPGRADER

## START DEPLOYER

find_deploy_partitions() {
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

set_active_passive() {
    tune2fs -L COS_ACTIVE ${STATEDIR}/cOS/active.img

    cp -f ${STATEDIR}/cOS/active.img ${STATEDIR}/cOS/passive.img
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
}

## END DEPLOYER

## START COS-RESET

reset_grub()
{
    if [ "$COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        GRUB_TARGET="--target=${ARCH}-efi --efi-directory=${STATEDIR}/boot/efi"
    fi
    #mount -o remount,rw ${STATE} /boot/grub2
    grub2-install ${GRUB_TARGET} --root-directory=${STATEDIR} --boot-directory=${STATEDIR} --removable ${DEVICE}

    GRUBDIR=
    if [ -d "${STATEDIR}/grub" ]; then
        GRUBDIR="${STATEDIR}/grub"
    elif [ -d "${STATEDIR}/grub2" ]; then
        GRUBDIR="${STATEDIR}/grub2"
    fi

    cp -rfv /etc/cos/grub.cfg $GRUBDIR/grub.cfg
}

reset_state() {
    rm -rf /oem/*
    rm -rf /usr/local/*
}

copy_passive() {
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
    cp -rf ${STATEDIR}/cOS/passive.img ${STATEDIR}/cOS/active.img
    tune2fs -L COS_ACTIVE ${STATEDIR}/cOS/active.img
}

run_reset_hook() {
    loop_dir=$(mktemp -d -t loop-XXXXXXXXXX)
    mount -t ext2 ${STATEDIR}/cOS/passive.img $loop_dir
        
    mount $PERSISTENT $loop_dir/usr/local
    mount $OEM $loop_dir/oem

    run_hook after-reset-chroot $loop_dir

    umount $loop_dir/oem
    umount $loop_dir/usr/local
    umount $loop_dir
    rm -rf $loop_dir
}

copy_active() {
    if is_booting_from_squashfs; then
        tmp_dir=$(mktemp -d -t squashfs-XXXXXXXXXX)
        loop_dir=$(mktemp -d -t loop-XXXXXXXXXX)

        # Squashfs is at ${RECOVERYDIR}/cOS/recovery.squashfs. 
        mount -t squashfs -o loop ${RECOVERYDIR}/cOS/recovery.squashfs $tmp_dir
        
        TARGET=$loop_dir
        # TODO: Size should be tweakable
        dd if=/dev/zero of=${STATEDIR}/cOS/transition.img bs=1M count=$DEFAULT_IMAGE_SIZE
        mkfs.ext2 ${STATEDIR}/cOS/transition.img -L COS_PASSIVE
        sync
        LOOP=$(losetup --show -f ${STATEDIR}/cOS/transition.img)
        mount -t ext2 $LOOP $TARGET
        rsync -aqzAX --exclude='mnt' \
        --exclude='proc' --exclude='sys' \
        --exclude='dev' --exclude='tmp' \
        $tmp_dir/ $TARGET
        ensure_dir_structure

        SELinux_relabel

        # Targets are ${STATEDIR}/cOS/active.img and ${STATEDIR}/cOS/passive.img
        umount $tmp_dir
        rm -rf $tmp_dir
        umount $TARGET
        rm -rf $TARGET

        mv -f ${STATEDIR}/cOS/transition.img ${STATEDIR}/cOS/passive.img
        sync
    else
        cp -rf ${RECOVERYDIR}/cOS/recovery.img ${STATEDIR}/cOS/passive.img
    fi
    
    run_reset_hook
    copy_passive
}

reset_cleanup2()
{  
    umount /boot/efi || true
    umount /boot/grub2 || true
}

reset_cleanup()
{
    EXIT=$?
    reset_cleanup2 2>/dev/null || true
    return $EXIT
}

check_recovery() {
    if ! is_booting_from_squashfs; then
        system=$(blkid -L COS_SYSTEM || true)
        if [ -z "$system" ]; then
            echo "cos-reset can be run only from recovery"
            exit 1
        fi
        recovery=$(blkid -L COS_RECOVERY || true)
        if [ -z "$recovery" ]; then
            echo "Can't find COS_RECOVERY partition"
            exit 1
        fi
    fi
}

find_recovery_partitions() {
    STATE=$(blkid -L COS_STATE || true)
    if [ -z "$STATE" ]; then
        echo "State partition cannot be found"
        exit 1
    fi
    DEVICE=/dev/$(lsblk -no pkname $STATE)

    BOOT=$(blkid -L COS_GRUB || true)

    OEM=$(blkid -L COS_OEM || true)

    PERSISTENT=$(blkid -L COS_PERSISTENT || true)
    if [ -z "$PERSISTENT" ]; then
        echo "Persistent partition cannot be found"
        exit 1
    fi
}

do_recovery_mount()
{
    STATEDIR=/tmp/state
    mkdir -p $STATEDIR || true

    if is_booting_from_squashfs; then
        RECOVERYDIR=/run/initramfs/live
    else
        RECOVERYDIR=/run/initramfs/cos-state
    fi

    #mount -o remount,rw ${STATE} ${STATEDIR}

    mount ${STATE} $STATEDIR

    if [ -n "${BOOT}" ]; then
        mkdir -p $STATEDIR/boot/efi || true
        mount ${BOOT} $STATEDIR/boot/efi
    fi
}

## END COS-RESET

## START COS-rebrand

rebrand_grub_menu() {
	local grub_entry="$1"

	STATEDIR=$(blkid -L COS_STATE)
	mkdir -p /run/boot
	
	if ! is_mounted /run/boot; then
	   mount $STATEDIR /run/boot
	fi

    grub2-editenv /run/boot/grub_oem_env set default_menu_entry="$grub_entry"

    umount /run/boot
}

rebrand_cleanup2()
{
    sync
    umount ${STATEDIR}
}

rebrand_cleanup()
{
    EXIT=$?
    rebrand_cleanup2 2>/dev/null || true
    return $EXIT
}

# END COS_REBRAND

rebrand() {
    load_config
    grub_entry="${GRUB_ENTRY_NAME:-cOS}"

    #trap rebrand_cleanup exit

    rebrand_grub_menu "$grub_entry"
}

reset() {
    trap reset_cleanup exit

    check_recovery

    find_recovery_partitions

    do_recovery_mount

    load_config

    if [ "$STRICT_MODE" = "true" ]; then
        cos-setup before-reset
    else 
        cos-setup before-reset || true
    fi

    if [ -n "$PERSISTENCE_RESET" ] && [ "$PERSISTENCE_RESET" == "true" ]; then
        reset_state
    fi

    copy_active

    reset_grub

    #cos-rebrand
    rebrand

    if [ "$STRICT_MODE" = "true" ]; then
        cos-setup after-reset
    else 
        cos-setup after-reset || true
    fi
}

deploy() {

    while [ "$#" -gt 0 ]; do
        case $1 in
            --docker-image)
                NO_CHANNEL=true
                ;;
            --no-verify)
                VERIFY=false
                ;;
            --no-cosign)
                COSIGN=false
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
                COS_IMAGE=$1
                break
                ;;
        esac
        shift 1
    done
    find_upgrade_channel

    trap upgrade_cleanup exit

    echo "Deploying system.."

    find_deploy_partitions

    mount_image "deploy"

    do_upgrade "deploy"

    set_active_passive

    rebrand

    echo "Flush changes to disk"
    sync

    echo "Deployment done, now you might want to reboot"
}

upgrade() {

    while [ "$#" -gt 0 ]; do
        case $1 in
            --docker-image)
                NO_CHANNEL=true
                ;;
            --directory)
                NO_CHANNEL=true
                DIRECTORY=true
                ;;
            --strict)
                STRICT_MODE=true
                ;;
            --recovery)
                UPGRADE_RECOVERY=true
                ;;
            --no-verify)
                VERIFY=false
                ;;
            --no-cosign)
                COSIGN=false
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
                INTERACTIVE=true
                COS_IMAGE=$1
                break
                ;;
        esac
        shift 1
    done

    find_upgrade_channel

    trap upgrade_cleanup exit

    if [ -n "$UPGRADE_RECOVERY" ] && [ $UPGRADE_RECOVERY == true ]; then
        echo "Upgrading recovery partition.."

        find_partitions

        find_recovery

        mount_image "recovery"

        do_upgrade "upgrade"

        switch_recovery
    else
        echo "Upgrading system.."

        find_partitions

        mount_image "upgrade"

        do_upgrade "upgrade"

        switch_active
    fi

    echo "Flush changes to disk"
    sync

    if [ -n "$INTERACTIVE" ] && [ $INTERACTIVE == false ]; then
        if grep -q 'cos.upgrade.power_off=true' /proc/cmdline; then
            poweroff -f
        else
            echo " * Rebooting system in 5 seconds (CTRL+C to cancel)"
            sleep 5
            reboot -f
        fi
    else
        echo "Upgrade done, now you might want to reboot"
    fi
}

install() {

    while [ "$#" -gt 0 ]; do
        case $1 in
            --no-format)
                COS_INSTALL_NO_FORMAT=true
                ;;
            --force-efi)
                COS_INSTALL_FORCE_EFI=true
                ;;
            --force-gpt)
                COS_INSTALL_FORCE_GPT=true
                ;;
            --poweroff)
                COS_INSTALL_POWER_OFF=true
                ;;
            --strict)
                STRICT_MODE=true
                ;;
            --debug)
                set -x
                COS_INSTALL_DEBUG=true
                ;;
            --config)
                shift 1
                COS_INSTALL_CONFIG_URL=$1
                ;;
            --partition-layout)
                shift 1
                COS_PARTITION_LAYOUT=$1
                ;;
            --iso)
                shift 1
                COS_INSTALL_ISO_URL=$1
                ;;
            --tty)
                shift 1
                COS_INSTALL_TTY=$1
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
                INTERACTIVE=true
                COS_INSTALL_DEVICE=$1
                break
                ;;
        esac
        shift 1
    done

    load_config

    if [ -z "$COS_INSTALL_DEVICE" ]; then
        usage
    fi

    validate_progs
    validate_device

    trap installer_cleanup exit

    if [ "$STRICT_MODE" = "true" ]; then
    cos-setup before-install
    else
    cos-setup before-install || true
    fi

    get_iso
    setup_style
    do_format
    do_mount
    do_copy
    install_grub

    SELinux_relabel

    if [ "$STRICT_MODE" = "true" ]; then
    run_hook after-install-chroot $TARGET
    else
    run_hook after-install-chroot $TARGET || true
    fi

    umount_target 2>/dev/null

    prepare_recovery
    prepare_passive

    rebrand

    if [ "$STRICT_MODE" = "true" ]; then
    cos-setup after-install
    else
    cos-setup after-install || true
    fi

    if [ -n "$INTERACTIVE" ]; then
        exit 0
    fi

    if [ "$COS_INSTALL_POWER_OFF" = true ] || grep -q 'cos.install.power_off=true' /proc/cmdline; then
        poweroff -f
    else
        echo " * Rebooting system in 5 seconds (CTRL+C to cancel)"
        sleep 5
        reboot -f
    fi
}

case $1 in
    install)
        shift 1
        install $@
        ;;
    upgrade)
        shift 1
        upgrade $@
        ;;
    rebrand)
        shift 1
        rebrand
        ;;
    deploy)
        shift 1
        deploy $@
        ;;
    reset)
        shift 1
        reset
        ;;
    -h)
        usage
        ;;
    --help)
        usage
        ;;
esac
