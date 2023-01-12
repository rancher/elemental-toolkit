#!/bin/bash
set -e

PROG=$0

## Installer
_PROGS="dd curl mkfs.ext4 mkfs.vfat fatlabel parted partprobe grub2-install grub2-editenv"
_DISTRO=/run/rootfsbase
_ISOMNT=/run/initramfs/live
_TARGET=/run/cos/target
_RECOVERYDIR=/run/cos/recovery
_RECOVERYSQUASHFS=${_ISOMNT}/recovery.squashfs
_GRUBCONF=/etc/cos/grub.cfg

# Default size (in MB) of disk image files (.img) created during upgrades
_DEFAULT_IMAGE_SIZE=3240

## cosign signatures
_COSIGN_REPOSITORY="raccos/releases-:FLAVOR:"
_COSIGN_EXPERIMENTAL=1
_COSIGN_PUBLIC_KEY_LOCATION=""

## Upgrades
_CHANNEL_UPGRADES="true"
_GRUB_ENTRY_NAME="cOs"

_ARCH=$(uname -p)
if [ "${_ARCH}" == "aarch64" ]; then
  _ARCH="arm64"
fi

if [ "$COS_DEBUG" = true ]; then
    set -x
fi

## COMMON


load_full_config() {
  # Config values are loaded in the following order:
  # defaults -> config -> env var -> flags
  # looks a bit convoluted but mainly we want to avoid that the load of config files overrides
  # actual env vars as they share the same name
  # so first we load the env vars and store them in temporary vars
  load_env_vars
  # then we load the config files and fill the internal variables used along the script with those values IF there isnt
  # a temporary env var for the same value
  load_config
  # afterwards we set the temporal store vars into the internal variables, overriding any defaults
  set_env_vars
  # after this, we load the flags and those will also override the internal values

  # export cosign values after loading all values
  export COSIGN_REPOSITORY=$_COSIGN_REPOSITORY
  export COSIGN_EXPERIMENTAL=$_COSIGN_EXPERIMENTAL
  export COSIGN_PUBLIC_KEY_LOCATION=$_COSIGN_PUBLIC_KEY_LOCATION
}

load_env_vars() {
    # Load vars from environment variables into temporal vars
    if [ -n "${VERIFY}" ]; then
      COS_ENV_VERIFY=$VERIFY
    fi

    if [ -n "${GRUB_ENTRY_NAME}" ]; then
      COS_ENV_GRUB_ENTRY_NAME=$GRUB_ENTRY_NAME
    fi

    if [ -n "${COSIGN_REPOSITORY}" ]; then
      COS_ENV_COSIGN_REPOSITORY=$COSIGN_REPOSITORY
    fi

    if [ -n "${COSIGN_EXPERIMENTAL}" ]; then
      COS_ENV_COSIGN_EXPERIMENTAL=$COSIGN_EXPERIMENTAL
    fi

    if [ -n "${COSIGN_PUBLIC_KEY_LOCATION}" ]; then
      COS_ENV_COSIGN_PUBLIC_KEY_LOCATION=$COSIGN_PUBLIC_KEY_LOCATION
    fi

    if [ -n "${DEFAULT_IMAGE_SIZE}" ]; then
      COS_ENV_DEFAULT_IMAGE_SIZE=$DEFAULT_IMAGE_SIZE
    fi

    if [ -n "${CHANNEL_UPGRADES}" ]; then
      COS_ENV_CHANNEL_UPGRADES=$CHANNEL_UPGRADES
    fi

    if [ -n "${UPGRADE_IMAGE}" ]; then
      COS_ENV_UPGRADE_IMAGE=$UPGRADE_IMAGE
    fi

    if [ -n "${RECOVERY_IMAGE}" ]; then
      COS_ENV_RECOVERY_IMAGE=$RECOVERY_IMAGE
    fi

    # Only support CURRENT override via env var, so send it directly into the internal var
    if [ -n "${CURRENT}" ]; then
      _CURRENT=$CURRENT
    fi
}

set_env_vars() {
    # Set the temp stored env vars into the internal values after loading the config ones
    # Load vars from environment variables into internal vars
    if [ -n "${COS_ENV_VERIFY}" ]; then
      _VERIFY=$COS_ENV_VERIFY
    fi

    if [ -n "${COS_ENV_GRUB_ENTRY_NAME}" ]; then
      _GRUB_ENTRY_NAME=$COS_ENV_GRUB_ENTRY_NAME
    fi

    if [ -n "${COS_ENV_COSIGN_REPOSITORY}" ]; then
      _COSIGN_REPOSITORY=$COS_ENV_COSIGN_REPOSITORY
    fi

    if [ -n "${COS_ENV_COSIGN_EXPERIMENTAL}" ]; then
      _COSIGN_EXPERIMENTAL=$COS_ENV_COSIGN_EXPERIMENTAL
    fi

    if [ -n "${COS_ENV_COSIGN_PUBLIC_KEY_LOCATION}" ]; then
      _COSIGN_PUBLIC_KEY_LOCATION=$COS_ENV_COSIGN_PUBLIC_KEY_LOCATION
    fi

    if [ -n "${COS_ENV_DEFAULT_IMAGE_SIZE}" ]; then
      _DEFAULT_IMAGE_SIZE=$COS_ENV_DEFAULT_IMAGE_SIZE
    fi

    if [ -n "${COS_ENV_CHANNEL_UPGRADES}" ]; then
      _CHANNEL_UPGRADES=$COS_ENV_CHANNEL_UPGRADES
    fi

    if [ -n "${COS_ENV_UPGRADE_IMAGE}" ]; then
      _UPGRADE_IMAGE=$COS_ENV_UPGRADE_IMAGE
    fi

    if [ -n "${COS_ENV_RECOVERY_IMAGE}" ]; then
      _RECOVERY_IMAGE=$COS_ENV_RECOVERY_IMAGE
    fi
}

load_config() {
    # in here we load the config files
    if [ -e /etc/environment ]; then
        source /etc/environment
    fi

    if [ -e /etc/os-release ]; then
        source /etc/os-release
    fi

    if [ -e /etc/cos/config ]; then
        source /etc/cos/config
    fi

    if [ -e /etc/cos-upgrade-image ]; then
        source /etc/cos-upgrade-image
    fi

    # Load vars from files into internal vars
    # Always check that the vars loaded from env are not in there
    # if we have vars from env, then skip overriding

    if [ -n "${VERIFY}" ] && [[ -z "${COS_ENV_VERIFY}" ]]; then
      _VERIFY=$VERIFY
    fi

    if [ -n "${GRUB_ENTRY_NAME}" ] && [[ -z "${COS_ENV_GRUB_ENTRY_NAME}" ]]; then
      _GRUB_ENTRY_NAME=$GRUB_ENTRY_NAME
    fi

    if [ -n "${COSIGN_REPOSITORY}" ] && [[ -z "${COS_ENV_COSIGN_REPOSITORY}" ]]; then
      _COSIGN_REPOSITORY=$COSIGN_REPOSITORY
    fi

    if [ -n "${COSIGN_EXPERIMENTAL}" ] && [[ -z "${COS_ENV_COSIGN_EXPERIMENTAL}" ]]; then
      _COSIGN_EXPERIMENTAL=$COSIGN_EXPERIMENTAL
    fi

    if [ -n "${COSIGN_PUBLIC_KEY_LOCATION}" ] && [[ -z "${COS_ENV_COSIGN_PUBLIC_KEY_LOCATION}" ]]; then
      _COSIGN_PUBLIC_KEY_LOCATION=$COSIGN_PUBLIC_KEY_LOCATION
    fi

    if [ -n "${DEFAULT_IMAGE_SIZE}" ] && [[ -z "${COS_ENV_DEFAULT_IMAGE_SIZE}" ]]; then
      _DEFAULT_IMAGE_SIZE=$DEFAULT_IMAGE_SIZE
    fi

    if [ -n "${CHANNEL_UPGRADES}" ] && [[ -z "${COS_ENV_CHANNEL_UPGRADES}" ]]; then
      _CHANNEL_UPGRADES=$CHANNEL_UPGRADES
    fi

    if [ -n "${UPGRADE_IMAGE}" ] && [[ -z "${COS_ENV_UPGRADE_IMAGE}" ]]; then
      _UPGRADE_IMAGE=$UPGRADE_IMAGE
    fi

    if [ -n "${RECOVERY_IMAGE}" ] && [[ -z "${COS_ENV_RECOVERY_IMAGE}" ]]; then
      _RECOVERY_IMAGE=$RECOVERY_IMAGE
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

run_chroot_hook() {
    local hook=$1
    local dir=$2

    prepare_chroot $dir
    chroot $dir /usr/bin/elemental run-stage $hook
    chroot $dir /usr/sbin/cos-rebrand
    cleanup_chroot $dir
}

is_mounted() {
    mountpoint -q "$1"
}

is_booting_from_squashfs() {
    if cat /proc/cmdline | grep -q "${RECOVERY_LABEL}"; then
        return 0
    else
        return 1
    fi
}

is_booting_from_live() {
    if [ -n "$_COS_BOOTING_FROM_LIVE" ]; then
        return 0
    fi

    if cat /proc/cmdline | grep -q "CDLABEL"; then
        return 0
    fi

    return 1
}

prepare_target() {
    mkdir -p ${_STATEDIR}/cOS || true
    rm -rf ${_STATEDIR}/cOS/transition.img || true
    dd if=/dev/zero of=${_STATEDIR}/cOS/transition.img bs=1M count=$_DEFAULT_IMAGE_SIZE
    mkfs.ext2 ${_STATEDIR}/cOS/transition.img
    mount -t ext2 -o loop ${_STATEDIR}/cOS/transition.img $_TARGET
}

usage()
{
    echo "Usage: $PROG install|deploy|upgrade|reset|rebrand [options]"
    echo ""
    echo "Example: $PROG-install /dev/vda"
    echo "  install:"
    echo "  [--partition-layout /path/to/config/file.yaml ] [--force-efi] [--force-gpt] [--docker-image IMAGE] [--no-verify] [--no-cosign] [--iso https://.../OS.iso] [--debug] [--tty TTY] [--poweroff] [--no-format] [--config https://.../config.yaml] DEVICE"
    echo ""
    echo "  upgrade:"
    echo "  [--strict] [--recovery] [--no-verify] [--no-cosign] [--directory] [--docker-image] (IMAGE/DIRECTORY)"
    echo ""
    echo "   DEVICE must be the disk that will be partitioned (/dev/vda). If you are using --no-format it should be the device of the ${STATE_LABEL} partition (/dev/vda2)"
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
    umount ${_TARGET}/oem
    umount ${_TARGET}/usr/local
    umount ${_TARGET}/boot/efi || true
    umount ${_TARGET}
    if [ -n "$_LOOP" ]; then
        losetup -d $_LOOP
    fi
}

installer_cleanup2()
{
    sync
    umount_target || true
    umount ${_STATEDIR}
    umount ${_RECOVERYDIR}
    [ -n "$_COS_INSTALL_ISO_URL" ] && umount ${_ISOMNT} || true
    if [ "$_COS_SILENCE_HOOKS" = "true" ]; then
        unset LOGLEVEL
    fi
}

installer_cleanup()
{
    EXIT=$?
    installer_cleanup2 2>/dev/null || true
    return $EXIT
}

prepare_recovery() {
    echo "Preparing recovery.."
    mkdir -p $_RECOVERYDIR

    mount $_RECOVERY $_RECOVERYDIR

    mkdir -p $_RECOVERYDIR/cOS

    if [ -e "$_RECOVERYSQUASHFS" ]; then
        echo "Copying squashfs.."
        cp -a $_RECOVERYSQUASHFS $_RECOVERYDIR/cOS/recovery.squashfs
    else
        if is_mounted /run/initramfs/cos-state; then
            echo "Recovery partition already mounted ande being used at /run/initramfs/cos-state. Backing off from replacing recovery"
            echo "Recovery partition can be upgraded while booting from an active or a passive partition only"
            umount $_RECOVERYDIR
            return
        fi
        echo "Copying image file.."
        cp -a $_STATEDIR/cOS/active.img $_RECOVERYDIR/cOS/recovery.img
        sync
        tune2fs -L ${SYSTEM_LABEL} $_RECOVERYDIR/cOS/recovery.img
    fi

    sync
    umount $_RECOVERYDIR
}

prepare_passive() {
    echo "Preparing passive boot.."
    cp -a ${_STATEDIR}/cOS/active.img ${_STATEDIR}/cOS/passive.img
    sync
    tune2fs -L ${PASSIVE_LABEL} ${_STATEDIR}/cOS/passive.img
    sync
}

part_probe() {
    local dev=$1

    # Don't require udevadm necessarly, but run it best-effort
    if hash udevadm 2>/dev/null; then
        udevadm settle
    fi

    partprobe ${dev} 2>/dev/null || true

    sync
    sleep 5

    dmsetup remove_all 2>/dev/null || true
}

blkid_probe() {
    _OEM=$(blkid -L ${OEM_LABEL} || true)
    _STATE=$(blkid -L ${STATE_LABEL} || true)
    _RECOVERY=$(blkid -L ${RECOVERY_LABEL} || true)
    _BOOT=$(blkid -L COS_GRUB || true)
    _PERSISTENT=$(blkid -L ${PERSISTENT_LABEL} || true)
}

check_required_partitions() {
    if [ -z "$_STATE" ]; then
            echo "State partition cannot be found"
            exit 1
    fi
}

do_format()
{
    if [ "$_COS_INSTALL_NO_FORMAT" = "true" ]; then
        _STATE=$(blkid -L ${STATE_LABEL} || true)
        if [ -z "$_STATE" ] && [ -n "$_DEVICE" ]; then
            tune2fs -L ${STATE_LABEL} $_DEVICE
        fi
        blkid_probe
        check_required_partitions
        return 0
    fi

    echo "Formatting drives.."

    if [ -n "$_COS_PARTITION_LAYOUT" ] && [ "$_PARTTABLE" != "gpt" ]; then
        echo "Custom layout only available with GPT based installations"
        exit 1
    fi

    dd if=/dev/zero of=${_DEVICE} bs=1M count=1
    parted -s ${_DEVICE} mklabel ${_PARTTABLE}

    local PREFIX

    # Partitioning via cloud-init config file
    if [ -n "$_COS_PARTITION_LAYOUT" ] && [ "$_PARTTABLE" = "gpt" ]; then
        if [ "$_BOOTFLAG" == "esp" ]; then
            parted -s ${_DEVICE} mkpart primary fat32 0% 50MB # efi
            parted -s ${_DEVICE} set 1 ${_BOOTFLAG} on

            part_probe ${_DEVICE}

            PREFIX=${_DEVICE}
            if [ ! -e ${PREFIX}1 ]; then
                PREFIX=${_DEVICE}p
            fi
            _BOOT=${PREFIX}1
            mkfs.vfat -F 32 ${_BOOT}
            fatlabel ${_BOOT} COS_GRUB
        elif [ "$_BOOTFLAG" == "bios_grub" ]; then
            parted -s ${_DEVICE} mkpart primary 0% 1MB # BIOS boot partition for GRUB
            parted -s ${_DEVICE} set 1 ${_BOOTFLAG} on
            part_probe ${_DEVICE}
        fi

        elemental cloud-init -s partitioning $_COS_PARTITION_LAYOUT

        part_probe $_DEVICE

        blkid_probe

        return 0
    fi

    local BOOT_NUM
    local OEM_NUM
    local STATE_NUM
    local RECOVERY_NUM
    local PERSISTENT_NUM

    # Standard partitioning
    if [ "$_PARTTABLE" = "gpt" ] && [ "$_BOOTFLAG" == "esp" ]; then
        BOOT_NUM=1
        OEM_NUM=2
        STATE_NUM=3
        RECOVERY_NUM=4
        PERSISTENT_NUM=5
        parted -s ${_DEVICE} mkpart primary fat32 0% 50MB # efi
        parted -s ${_DEVICE} mkpart primary ext4 50MB 100MB # oem
        parted -s ${_DEVICE} mkpart primary ext4 100MB 15100MB # state
        parted -s ${_DEVICE} mkpart primary ext4 15100MB 23100MB # recovery
        parted -s ${_DEVICE} mkpart primary ext4 23100MB 100% # persistent
        parted -s ${_DEVICE} set 1 ${_BOOTFLAG} on
    elif [ "$_PARTTABLE" = "gpt" ] && [ "$_BOOTFLAG" == "bios_grub" ]; then
        BOOT_NUM=
        OEM_NUM=2
        STATE_NUM=3
        RECOVERY_NUM=4
        PERSISTENT_NUM=5
        parted -s ${_DEVICE} mkpart primary 0% 1MB # BIOS boot partition for GRUB
        parted -s ${_DEVICE} mkpart primary ext4 1MB 51MB # oem
        parted -s ${_DEVICE} mkpart primary ext4 51MB 15051MB # state
        parted -s ${_DEVICE} mkpart primary ext4 15051MB 23051MB # recovery
        parted -s ${_DEVICE} mkpart primary ext4 23051MB 100% # persistent
        parted -s ${_DEVICE} set 1 ${_BOOTFLAG} on
    else
        BOOT_NUM=
        OEM_NUM=1
        STATE_NUM=2
        RECOVERY_NUM=3
        PERSISTENT_NUM=4
        parted -s ${_DEVICE} mkpart primary ext4 0% 50MB # oem
        parted -s ${_DEVICE} mkpart primary ext4 50MB 15050MB # state
        parted -s ${_DEVICE} mkpart primary ext4 15050MB 23050MB # recovery
        parted -s ${_DEVICE} mkpart primary ext4 23050MB 100% # persistent
        parted -s ${_DEVICE} set 2 ${_BOOTFLAG} on
    fi

    part_probe $_DEVICE

    PREFIX=${_DEVICE}
    if [ ! -e ${PREFIX}${STATE_NUM} ]; then
        PREFIX=${_DEVICE}p
    fi

    if [ ! -e ${PREFIX}${STATE_NUM} ]; then
        echo Failed to find ${PREFIX}${STATE_NUM} or ${_DEVICE}${STATE_NUM} to format
        exit 1
    fi

    if [ -n "${BOOT_NUM}" ]; then
        _BOOT=${PREFIX}${BOOT_NUM}
    fi
    _STATE=${PREFIX}${STATE_NUM}
    _OEM=${PREFIX}${OEM_NUM}
    _RECOVERY=${PREFIX}${RECOVERY_NUM}
    _PERSISTENT=${PREFIX}${PERSISTENT_NUM}

    mkfs.ext4 -F -L ${STATE_LABEL} ${_STATE}
    if [ -n "${_BOOT}" ]; then
        mkfs.vfat -F 32 ${_BOOT}
        fatlabel ${_BOOT} COS_GRUB
    fi

    mkfs.ext4 -F -L ${RECOVERY_LABEL} ${_RECOVERY}
    mkfs.ext4 -F -L ${OEM_LABEL} ${_OEM}
    mkfs.ext4 -F -L ${PERSISTENT_LABEL} ${_PERSISTENT}
}

do_mount()
{
    echo "Mounting critical endpoints.."

    mkdir -p ${_TARGET}
    ensure_dir_structure $_TARGET

    prepare_statedir "install"

    mkdir -p ${_STATEDIR}/cOS || true

    if [ -e "${_STATEDIR}/cOS/active.img" ]; then
        rm -rf ${_STATEDIR}/cOS/active.img
    fi

    dd if=/dev/zero of=${_STATEDIR}/cOS/active.img bs=1M count=$_DEFAULT_IMAGE_SIZE
    mkfs.ext2 -L ${ACTIVE_LABEL} ${_STATEDIR}/cOS/active.img

    if [ -z "$_${ACTIVE_LABEL}" ]; then
        sync
    fi

    _LOOP=$(losetup --show -f ${_STATEDIR}/cOS/active.img)
    mount -t ext2 $_LOOP $_TARGET

    mkdir -p ${_TARGET}/boot
    if [ -n "${_BOOT}" ]; then
        mkdir -p ${_TARGET}/boot/efi
        mount ${_BOOT} ${_TARGET}/boot/efi
    fi

    mkdir -p ${_TARGET}/oem
    mount ${_OEM} ${_TARGET}/oem
    mkdir -p ${_TARGET}/usr/local
    mount ${_PERSISTENT} ${_TARGET}/usr/local
}

get_url()
{
    local FROM=$1
    local TO=$2
    case $FROM in
        ftp*|http*|tftp*)
            local n=0
            local attempts=5
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
    local temp_file
    local iso_device
    if [ -n "$_COS_INSTALL_ISO_URL" ]; then
        _ISOMNT=$(mktemp --tmpdir -d cos.XXXXXXXX._ISOMNT)
        temp_file=$(mktemp --tmpdir cos.XXXXXXXX.iso)
        get_url ${_COS_INSTALL_ISO_URL} ${temp_file}
        iso_device=$(losetup --show -f $temp_file)
        mount -o ro ${iso_device} ${_ISOMNT}
    fi
}

get_image()
{
    if [ -n "$_UPGRADE_IMAGE" ]; then
        local temp
        _DISTRO=$(mktemp --tmpdir -d cos.XXXXXXXX.image)
        temp=$(mktemp --tmpdir -d cos.XXXXXXXX.image)
        create_rootfs "install" $_DISTRO $temp
    fi
}

do_copy()
{
    echo "Copying Elemental.."

    rsync -aqAX --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' ${_DISTRO}/ ${_TARGET}
    if [ -n "$_COS_INSTALL_CONFIG_URL" ]; then
        _OEM=${_TARGET}/oem/99_custom.yaml
        get_url "$_COS_INSTALL_CONFIG_URL" $_OEM
        chmod 600 ${_OEM}
    fi
    ensure_dir_structure $_TARGET
}

SELinux_relabel()
{
    if which setfiles > /dev/null && [ -e ${_TARGET}/etc/selinux/targeted/contexts/files/file_contexts ]; then
        setfiles -r ${_TARGET} ${_TARGET}/etc/selinux/targeted/contexts/files/file_contexts ${_TARGET}
    fi
}

install_grub()
{
    if [ -z "$_DEVICE" ]; then
        echo "No Installation device specified. Skipping GRUB installation"
        return 0
    fi
    local TTY

    echo "Installing GRUB.."

    if [ "$_COS_INSTALL_DEBUG" ]; then
        GRUB_DEBUG="cos.debug"
    fi

    if [ -z "${_COS_INSTALL_TTY}" ]; then
        TTY=$(tty | sed 's!/dev/!!')
    else
        TTY=$_COS_INSTALL_TTY
    fi

    if [ "$_COS_INSTALL_NO_FORMAT" = "true" ]; then
        return 0
    fi

    if [ "$_COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        _GRUB_TARGET="--target=${_ARCH}-efi --efi-directory=${_TARGET}/boot/efi"
    fi

    mkdir ${_TARGET}/proc || true
    mkdir ${_TARGET}/dev || true
    mkdir ${_TARGET}/sys || true
    mkdir ${_TARGET}/tmp || true

    grub2-install ${_GRUB_TARGET} --root-directory=${_TARGET}  --boot-directory=${_STATEDIR} --removable ${_DEVICE}

    local GRUBDIR
    if [ -d "${_STATEDIR}/grub" ]; then
        GRUBDIR="${_STATEDIR}/grub"
    elif [ -d "${_STATEDIR}/grub2" ]; then
        GRUBDIR="${_STATEDIR}/grub2"
    fi

    cp -rf $_GRUBCONF $GRUBDIR/grub.cfg

    if [ -e "/dev/${TTY%,*}" ] && [ "$TTY" != tty1 ] && [ "$TTY" != console ] && [ -n "$TTY" ]; then
        sed -i "s!console=tty1!console=tty1 console=${TTY}!g" $GRUBDIR/grub.cfg
    fi
}

setup_style()
{
    if [ "$_COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        _PARTTABLE=gpt
        _BOOTFLAG=esp
        if [ ! -e /sys/firmware/efi ]; then
            echo WARNING: installing EFI on to a system that does not support EFI
        fi
    elif [ "$_COS_INSTALL_FORCE_GPT" = "true" ]; then
        _PARTTABLE=gpt
        _BOOTFLAG=bios_grub
    else
        _PARTTABLE=msdos
        _BOOTFLAG=boot
    fi
}

validate_progs()
{
    local MISSING
    for i in $_PROGS; do
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
    _DEVICE=$_COS_INSTALL_DEVICE
    if [ -n "${_DEVICE}" ] && [ ! -b ${_DEVICE} ]; then
        echo "You should use an available device. Device ${_DEVICE} does not exist."
        exit 1
    fi
    if [ -n "$_COS_INSTALL_NO_FORMAT" ]; then
        _ACTIVE=$(blkid -L ${ACTIVE_LABEL} || true)
        _PASSIVE=$(blkid -L ${PASSIVE_LABEL} || true)
        if [ -n "$_ACTIVE" ] || [ -n "$_PASSIVE" ]; then
            if [ "$_FORCE" == "true" ]; then
                echo "Forcing overwrite current ${ACTIVE_LABEL} and ${PASSIVE_LABEL} partitions"
                return 0
            else
                echo "There is already an active deployment in the system, use '--force' flag to overwrite it"
                exit 1
            fi
        fi
    fi
}

## END INSTALLER

## UPGRADER

find_partitions() {
    _STATE=$(blkid -L ${STATE_LABEL} || true)
    if [ -z "$_STATE" ]; then
        echo "State partition cannot be found"
        exit 1
    fi

    _OEM=$(blkid -L ${OEM_LABEL} || true)

    _PERSISTENT=$(blkid -L ${PERSISTENT_LABEL} || true)
    if [ -z "$_PERSISTENT" ]; then
        echo "Persistent partition cannot be found"
        exit 1
    fi

    _ACTIVE=$(blkid -L ${ACTIVE_LABEL} || true)
    if [ -n "$_ACTIVE" ]; then
        _CURRENT=active.img
    fi

    _PASSIVE=$(blkid -L ${PASSIVE_LABEL} || true)
    if [ -n "$_PASSIVE" ]; then
        _CURRENT=passive.img
    fi

    if [ -z "$_CURRENT" ]; then
        # We booted from an ISO or some else medium. We assume we want to fixup the current label
        read -p "Could not determine current partition. Do you want to overwrite your current active partition? (CURRENT=active.img) [y/N] : " -n 1 -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]
        then
            [[ "$0" = "$BASH_SOURCE" ]] && exit 1 || return 1 # handle exits from shell or function but don't exit interactive shell
        fi
        _CURRENT=active.img
        echo
    fi

    echo "-> Upgrade target: $_CURRENT"
}

find_recovery() {
    _RECOVERY=$(blkid -L ${RECOVERY_LABEL} || true)
    if [ -z "$_RECOVERY" ]; then
        echo "${RECOVERY_LABEL} partition cannot be found"
        exit 1
    fi
}

# cos-upgrade-image: system/cos
find_upgrade_channel() {

    if [ -n "$_NO_CHANNEL" ] && [ $_NO_CHANNEL == true ]; then
        _CHANNEL_UPGRADES=false
    fi

    if [ -n "$_COS_IMAGE" ]; then
        # passing an image from command line so we override any defaults loaded from /etc/cos-upgrade-image
        _UPGRADE_IMAGE=$_COS_IMAGE
        echo "Upgrading to image $_UPGRADE_IMAGE"
    else
        if [ -z "$_UPGRADE_IMAGE" ]; then
            # there is no UPGRADE_IMAGE on /etc/cos-upgrade-image or env so we default to system/cos
            _UPGRADE_IMAGE="system/cos"
        fi

        if [ -n "$_UPGRADE_RECOVERY" ] && [ $_UPGRADE_RECOVERY == true ] && [ -n "$_RECOVERY_IMAGE" ]; then
            # if we have UPGRADE_RECOVERY set to true and there is a RECOVERY_IMAGE set we want to upgrade recovery
            # so set the upgrade image to the recovery one
            _UPGRADE_IMAGE=$_RECOVERY_IMAGE
        fi
    fi
}

is_squashfs() {
    if [ -e "${_STATEDIR}/cOS/recovery.squashfs" ]; then
        return 0
    else
        return 1
    fi
}

recovery_boot() {
    local cmdline
    cmdline="$(cat /proc/cmdline)"
    if echo $cmdline | grep -q "${RECOVERY_LABEL}" || echo $cmdline | grep -q "${SYSTEM_LABEL}"; then
        return 0
    else
        return 1
    fi
}

prepare_squashfs_target() {
    rm -rf $_TARGET || true
    _TARGET=${_STATEDIR}/tmp/target
    mkdir -p $_TARGET
}

mount_state() {
    if [ -n "${_STATE}" ]; then
        _STATEDIR=/run/initramfs/state
        mkdir -p $_STATEDIR
        mount ${_STATE} ${_STATEDIR}
    else
        echo "No state partition found. Skipping mount"
    fi
}

prepare_statedir() {
    local target=$1
    case $target in
    recovery)
        _STATEDIR=/tmp/recovery

        mkdir -p $_STATEDIR || true
        mount $_RECOVERY $_STATEDIR
        if is_squashfs; then
            echo "Preparing squashfs target"
            prepare_squashfs_target
        else
            echo "Preparing image target"
            prepare_target
        fi
        ;;
    *)
        _STATEDIR=/run/initramfs/cos-state

        if [ -d "$_STATEDIR" ]; then
            if recovery_boot; then
                mount_state
            else
                mount -o remount,rw ${_STATE} ${_STATEDIR}
            fi
        else
            mount_state
        fi
        ;;
    esac
}

mount_image() {
    local target=$1

    _TARGET=/tmp/upgrade

    mkdir -p $_TARGET || true
    prepare_statedir $target

    case $target in
    upgrade)
        prepare_target
        ;;
    esac
}

switch_active() {
    if [[ "$_CURRENT" == "active.img" ]]; then
        mv -f ${_STATEDIR}/cOS/$_CURRENT ${_STATEDIR}/cOS/passive.img
        tune2fs -L ${PASSIVE_LABEL} ${_STATEDIR}/cOS/passive.img
    fi

    mv -f ${_STATEDIR}/cOS/transition.img ${_STATEDIR}/cOS/active.img
    tune2fs -L ${ACTIVE_LABEL} ${_STATEDIR}/cOS/active.img
}

switch_recovery() {
    if is_squashfs; then
        local XZ_FILTER
        if [[ "${_ARCH}" == "arm64" ]]; then
          XZ_FILTER="arm"
        else
          XZ_FILTER="x86"
        fi
        mksquashfs $_TARGET ${_STATEDIR}/cOS/transition.squashfs -b 1024k -comp xz -Xbcj ${XZ_FILTER}
        mv ${_STATEDIR}/cOS/transition.squashfs ${_STATEDIR}/cOS/recovery.squashfs
        rm -rf $_TARGET
    else
        mv -f ${_STATEDIR}/cOS/transition.img ${_STATEDIR}/cOS/recovery.img
        tune2fs -L ${SYSTEM_LABEL} ${_STATEDIR}/cOS/recovery.img
    fi
}

ensure_dir_structure() {
    local target=$1
    for mnt in /sys /proc /dev /tmp /boot /usr/local /oem
    do
        if [ ! -d "${target}${mnt}" ]; then
          mkdir -p ${target}${mnt}
        fi
    done
}

luet_args() {
    local args
    args="--enable-logfile --logfile /tmp/luet.log"
    if [ -z "$_VERIFY" ] || [ "$_VERIFY" == true ]; then
        args+=" --plugin luet-mtree"
    fi

    if [ -z "$_COSIGN" ]; then
      args+=" --plugin luet-cosign"
    fi

    echo $args
}

unpack_args() {
    local args
    args="--logfile /tmp/elemental.log"
    if [ -z "$_VERIFY" ] || [ "$_VERIFY" == true ]; then
        args+=" --plugin luet-mtree"
    fi

    if [ -z "$_COSIGN" ]; then
      args+=" --plugin luet-cosign"
    fi

    echo $args
}

create_rootfs() {
    local hook_name=$1
    local target=$2
    local temp_dir=$3

    local upgrade_state_dir="$temp_dir"
    local temp_upgrade=$upgrade_state_dir/tmp/upgrade
    rm -rf $upgrade_state_dir || true
    mkdir -p $temp_upgrade


    if [ "$_STRICT_MODE" = "true" ]; then
      elemental run-stage before-$hook_name
    else 
      elemental run-stage before-$hook_name || true
    fi

    # FIXME: XDG_RUNTIME_DIR is for containerd, by default that points to /run/user/<uid>
    # which might not be sufficient to unpack images. Use /usr/local/tmp until we get a separate partition
    # for the state
    # FIXME: Define default /var/tmp as tmpdir_base in default luet config file
    export XDG_RUNTIME_DIR=$temp_upgrade
    export TMPDIR=$temp_upgrade
    local _args
    _args="$(luet_args)"
    _unpack_args="$(unpack_args)"
    if [ -n "$_CHANNEL_UPGRADES" ] && [ "$_CHANNEL_UPGRADES" == true ]; then
        echo "Upgrading from release channel"
        set -x
        luet install $_args --system-target $target --system-engine memory -y $_UPGRADE_IMAGE
        luet cleanup
        set +x
    elif [ "$_DIRECTORY" == true ]; then
        echo "Upgrading from local folder: $_UPGRADE_IMAGE"
        rsync -axq --exclude='host' --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' ${_UPGRADE_IMAGE}/ $target
    else
        echo "Upgrading from container image: $_UPGRADE_IMAGE"
        set -x
        # unpack doesnt like when you try to unpack to a non existing dir
        mkdir -p $upgrade_state_dir/tmp/rootfs || true
        elemental pull-image $_unpack_args $_UPGRADE_IMAGE $upgrade_state_dir/tmp/rootfs
        set +x
        rsync -aqzAX --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' $upgrade_state_dir/tmp/rootfs/ $target
        rm -rf $upgrade_state_dir/tmp/rootfs
    fi

    ensure_dir_structure $target

    chmod 755 $target
    SELinux_relabel

    if [ -n "$_PERSISTENT" ]; then
        mount $_PERSISTENT $target/usr/local
    fi
    if [ -n "$_OEM" ]; then
        mount $_OEM $target/oem
    fi
    if [ "$_STRICT_MODE" = "true" ]; then
        run_chroot_hook after-$hook_name-chroot $target
    else 
        run_chroot_hook after-$hook_name-chroot $target || true
    fi
    if [ -n "$_OEM" ]; then
     umount $target/oem
    fi
    if [ -n "$_PERSISTENT" ]; then
        umount $target/usr/local
    fi
    if [ "$_STRICT_MODE" = "true" ]; then
      elemental run-stage after-$hook_name
    else 
      elemental run-stage after-$hook_name || true
    fi

    rm -rf $upgrade_state_dir
    umount $target || true
}

upgrade_cleanup2()
{
    rm -rf /usr/local/tmp/upgrade || true
    mount -o remount,ro ${_STATE} ${_STATEDIR} || true
    if [ -n "${_TARGET}" ]; then
        umount ${_TARGET}/boot/efi || true
        umount ${_TARGET}/ || true
        rm -rf ${_TARGET}
    fi
    if [ -n "$_UPGRADE_RECOVERY" ] && [ $_UPGRADE_RECOVERY == true ]; then
	    umount ${_STATEDIR} || true
    fi
    if [ "$_STATEDIR" == "/run/initramfs/state" ]; then
        umount ${_STATEDIR}
        rm -rf $_STATEDIR
    fi
}

upgrade_cleanup()
{
    EXIT=$?
    upgrade_cleanup2 2>/dev/null || true
    return $EXIT
}


## END UPGRADER

## START COS-RESET

reset_grub()
{
    if [ "$_COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        _GRUB_TARGET="--target=${_ARCH}-efi --efi-directory=${_STATEDIR}/boot/efi"
    fi
    #mount -o remount,rw ${_STATE} /boot/grub2
    grub2-install ${_GRUB_TARGET} --root-directory=${_STATEDIR} --boot-directory=${_STATEDIR} --removable ${_DEVICE}

    local GRUBDIR=
    if [ -d "${_STATEDIR}/grub" ]; then
        GRUBDIR="${_STATEDIR}/grub"
    elif [ -d "${_STATEDIR}/grub2" ]; then
        GRUBDIR="${_STATEDIR}/grub2"
    fi

    cp -rfv $_GRUBCONF $GRUBDIR/grub.cfg
}

reset_state() {
    rm -rf /oem/*
    rm -rf /usr/local/*
}

copy_passive() {
    tune2fs -L ${PASSIVE_LABEL} ${_STATEDIR}/cOS/passive.img
    cp -rf ${_STATEDIR}/cOS/passive.img ${_STATEDIR}/cOS/active.img
    tune2fs -L ${ACTIVE_LABEL} ${_STATEDIR}/cOS/active.img
}

run_reset_hook() {
    local loop_dir
    loop_dir=$(mktemp -d -t loop-XXXXXXXXXX)
    mount -t ext2 ${_STATEDIR}/cOS/passive.img $loop_dir
        
    mount $_PERSISTENT $loop_dir/usr/local
    mount $_OEM $loop_dir/oem

    run_chroot_hook after-reset-chroot $loop_dir

    umount $loop_dir/oem
    umount $loop_dir/usr/local
    umount $loop_dir
    rm -rf $loop_dir
}

copy_active() {
    if is_booting_from_squashfs; then
        local tmp_dir
        local loop_dir

        tmp_dir=$(mktemp -d -t squashfs-XXXXXXXXXX)
        loop_dir=$(mktemp -d -t loop-XXXXXXXXXX)

        # Squashfs is at ${_RECOVERYDIR}/cOS/recovery.squashfs.
        mount -t squashfs -o loop ${_RECOVERYDIR}/cOS/recovery.squashfs $tmp_dir
        
        _TARGET=$loop_dir
        # TODO: Size should be tweakable
        dd if=/dev/zero of=${_STATEDIR}/cOS/transition.img bs=1M count=$_DEFAULT_IMAGE_SIZE
        mkfs.ext2 -L ${PASSIVE_LABEL} ${_STATEDIR}/cOS/transition.img
        sync
        _LOOP=$(losetup --show -f ${_STATEDIR}/cOS/transition.img)
        mount -t ext2 $_LOOP $_TARGET
        rsync -aqzAX --exclude='mnt' \
        --exclude='proc' --exclude='sys' \
        --exclude='dev' --exclude='tmp' \
        $tmp_dir/ $_TARGET
        ensure_dir_structure $_TARGET

        SELinux_relabel

        # Targets are ${_STATEDIR}/cOS/active.img and ${_STATEDIR}/cOS/passive.img
        umount $tmp_dir
        rm -rf $tmp_dir
        umount $_TARGET
        rm -rf $_TARGET

        mv -f ${_STATEDIR}/cOS/transition.img ${_STATEDIR}/cOS/passive.img
        sync
    else
        cp -rf ${_RECOVERYDIR}/cOS/recovery.img ${_STATEDIR}/cOS/passive.img
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
        local system
        local recovery

        system=$(blkid -L ${SYSTEM_LABEL} || true)
        if [ -z "$system" ]; then
            echo "cos-reset can be run only from recovery"
            exit 1
        fi
        recovery=$(blkid -L ${RECOVERY_LABEL} || true)
        if [ -z "$recovery" ]; then
            echo "Can't find ${RECOVERY_LABEL} partition"
            exit 1
        fi
    fi
}

find_recovery_partitions() {
    _STATE=$(blkid -L ${STATE_LABEL} || true)
    if [ -z "$_STATE" ]; then
        echo "State partition cannot be found"
        exit 1
    fi
    _DEVICE=/dev/$(lsblk -no pkname $_STATE)

    _BOOT=$(blkid -L COS_GRUB || true)

    _OEM=$(blkid -L ${OEM_LABEL} || true)

    _PERSISTENT=$(blkid -L ${PERSISTENT_LABEL} || true)
    if [ -z "$_PERSISTENT" ]; then
        echo "Persistent partition cannot be found"
        exit 1
    fi
}

do_recovery_mount()
{
    _STATEDIR=/tmp/state
    mkdir -p $_STATEDIR || true

    if is_booting_from_squashfs; then
        _RECOVERYDIR=/run/initramfs/live
    else
        _RECOVERYDIR=/run/initramfs/cos-state
    fi

    #mount -o remount,rw ${_STATE} ${_STATEDIR}

    mount ${_STATE} $_STATEDIR

    if [ -n "${_BOOT}" ]; then
        mkdir -p $_STATEDIR/boot/efi || true
        mount ${_BOOT} $_STATEDIR/boot/efi
    fi
}

## END COS-RESET

## START COS-rebrand

rebrand_grub_menu() {
	local grub_entry="$1"

	_STATEDIR=$(blkid -L ${STATE_LABEL})
	mkdir -p /run/boot
	
	if ! is_mounted /run/boot; then
	   mount $_STATEDIR /run/boot
	fi

    grub2-editenv /run/boot/grub_oem_env set default_menu_entry="$grub_entry"

    umount /run/boot
}

rebrand_cleanup2()
{
    sync
    umount ${_STATEDIR}
}

rebrand_cleanup()
{
    EXIT=$?
    rebrand_cleanup2 2>/dev/null || true
    return $EXIT
}

# END COS_REBRAND

rebrand() {
    load_full_config
    
    rebrand_grub_menu "${_GRUB_ENTRY_NAME}"
}

reset() {
    trap reset_cleanup exit
    load_full_config

    check_recovery

    find_recovery_partitions

    do_recovery_mount

    if [ "$_STRICT_MODE" = "true" ]; then
        elemental run-stage before-reset
    else 
        elemental run-stage before-reset || true
    fi

    if [ -n "$_PERSISTENCE_RESET" ] && [ "$_PERSISTENCE_RESET" == "true" ]; then
        reset_state
    fi

    copy_active

    reset_grub

    rebrand

    if [ "$_STRICT_MODE" = "true" ]; then
        elemental run-stage after-reset
    else 
        elemental run-stage after-reset || true
    fi
}

upgrade() {
    load_full_config
    while [ "$#" -gt 0 ]; do
        case $1 in
            --docker-image)
                _NO_CHANNEL=true
                ;;
            --directory)
                _NO_CHANNEL=true
                _DIRECTORY=true
                ;;
            --strict)
                _STRICT_MODE=true
                ;;
            --recovery)
                _UPGRADE_RECOVERY=true
                ;;
            --no-verify)
                _VERIFY=false
                ;;
            --no-cosign)
                _COSIGN=false
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
                _COS_IMAGE=$1
                break
                ;;
        esac
        shift 1
    done

    find_upgrade_channel

    trap upgrade_cleanup exit

    if [ -n "$_UPGRADE_RECOVERY" ] && [ $_UPGRADE_RECOVERY == true ]; then
        echo "Upgrading recovery partition.."

        find_partitions

        find_recovery

        mount_image "recovery"

        create_rootfs "upgrade" $_TARGET "/usr/local/.cos-upgrade"

        switch_recovery
    else
        echo "Upgrading system.."

        find_partitions

        mount_image "upgrade"

        create_rootfs "upgrade" $_TARGET "/usr/local/.cos-upgrade"

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
    load_full_config
    local _COS_INSTALL_POWER_OFF
    while [ "$#" -gt 0 ]; do
        case $1 in
            --docker-image)
                _NO_CHANNEL=true
                shift 1
                _COS_IMAGE=$1
                ;;
            --no-verify)
                _VERIFY=false
                ;;
            --no-cosign)
                _COSIGN=false
                ;;
            --no-format)
                _COS_INSTALL_NO_FORMAT=true
                ;;
            --force-efi)
                _COS_INSTALL_FORCE_EFI=true
                ;;
            --force)
                _FORCE=true
                ;;
            --force-gpt)
                _COS_INSTALL_FORCE_GPT=true
                ;;
            --poweroff)
                _COS_INSTALL_POWER_OFF=true
                ;;
            --strict)
                _STRICT_MODE=true
                ;;
            --debug)
                set -x
                _COS_INSTALL_DEBUG=true
                ;;
            --config)
                shift 1
                _COS_INSTALL_CONFIG_URL=$1
                ;;
            --partition-layout)
                shift 1
                _COS_PARTITION_LAYOUT=$1
                ;;
            --iso)
                shift 1
                _COS_INSTALL_ISO_URL=$1
                ;;
            --tty)
                shift 1
                _COS_INSTALL_TTY=$1
                ;;
            -h)
                usage
                ;;
            --help)
                usage
                ;;
            --silence-hooks)
                _COS_SILENCE_HOOKS=true
                ;;
            *)
                if [ "$#" -gt 2 ]; then
                    usage
                fi
                INTERACTIVE=true
                _COS_INSTALL_DEVICE=$1
                break
                ;;
        esac
        shift 1
    done

    # We want to find the upgrade channel if, no ISO url is supplied and:
    # 1: We aren't booting from LiveCD - the rootfs that we are going to install must be downloaded from somewhere
    # 2: If we specify directly an image to install
    if ! is_booting_from_live && [ -z "$_COS_INSTALL_ISO_URL" ] || [ -n "$_COS_IMAGE" ] && [ -z "$_COS_INSTALL_ISO_URL" ]; then
        find_upgrade_channel
    else
      # Beforehand the config was only loaded if the path above was set, so we loaded some vars needed for the upgrade
      # But now the config is loaded on start, so we need to unset some of those loaded vars if we dont need them, otherwise
      # we can wrongly install stuff
      unset _UPGRADE_IMAGE
      unset _RECOVERY_IMAGE
    fi

    if [ -z "$_COS_INSTALL_DEVICE" ] && [ -z "$_COS_INSTALL_NO_FORMAT" ]; then
        usage
    fi

    validate_progs
    validate_device

    trap installer_cleanup exit
    if [ "$_COS_SILENCE_HOOKS" = "true" ]; then
        export LOGLEVEL=error
    fi

    if [ "$_STRICT_MODE" = "true" ]; then
        elemental run-stage before-install
    else
        elemental run-stage before-install || true
    fi

    get_iso
    get_image
    setup_style
    do_format
    do_mount
    do_copy
    install_grub

    SELinux_relabel

    # Otherwise, hooks are executed in get_image
    if [ -z "$_UPGRADE_IMAGE" ]; then
        if [ "$_STRICT_MODE" = "true" ]; then
            run_chroot_hook after-install-chroot $_TARGET
        else
            run_chroot_hook after-install-chroot $_TARGET || true
        fi
    fi

    umount_target 2>/dev/null

    if ! is_booting_from_squashfs; then
        prepare_recovery
    fi
    prepare_passive

    if [ -z "$_UPGRADE_IMAGE" ]; then
        if [ "$_STRICT_MODE" = "true" ]; then
            elemental run-stage after-install
        else
            elemental run-stage after-install || true
        fi
    fi

    echo "Deployment done, now you might want to reboot"

    if [ -n "$INTERACTIVE" ]; then
        exit 0
    fi

    if [ "$_COS_INSTALL_POWER_OFF" = true ] || grep -q 'cos.install.power_off=true' /proc/cmdline; then
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
