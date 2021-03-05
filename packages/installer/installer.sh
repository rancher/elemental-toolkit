#!/bin/bash
set -e

PROG=$0
PROGS="dd curl mkfs.ext4 mkfs.vfat fatlabel parted partprobe grub2-install"
DISTRO=/run/rootfsbase
ISOBOOT=/run/initramfs/live/boot
TARGET=/run/cos/target
RECOVERYDIR=/run/cos/recovery

if [ "$COS_DEBUG" = true ]; then
    set -x
fi

umount_target() {
    sync
    if [ -n "${TARGET}" ]; then
        umount ${TARGET}/oem || true
        umount ${TARGET}/usr/local || true
        umount ${TARGET}/proc || true
        umount ${TARGET}/dev || true
        umount ${TARGET}/sys || true
        umount ${TARGET}/boot/efi || true
        umount ${TARGET}/boot/grub2 || true
        umount ${TARGET} || true
    fi
}

cleanup2()
{
    sync
    umount_target
    umount ${STATEDIR} || true
    umount ${RECOVERY} || true
}

cleanup()
{
    EXIT=$?
    cleanup2 2>/dev/null || true
    return $EXIT
}

usage()
{
    echo "Usage: $PROG [--force-efi] [--debug] [--tty TTY] [--poweroff] [--no-format] [--config https://.../config.yaml] DEVICE"
    echo ""
    echo "Example: $PROG /dev/vda"
    echo ""
    echo "DEVICE must be the disk that will be partitioned (/dev/vda). If you are using --no-format it should be the device of the COS_STATE partition (/dev/vda2)"
    echo ""
    echo "The parameters names refer to the same names used in the cmdline, refer to README.md for"
    echo "more info."
    echo ""
    exit 1
}

prepare_recovery() {
    echo "Preparing recovery.."

    mkdir -p $RECOVERYDIR
    mount $RECOVERY $RECOVERYDIR
    mkdir -p $RECOVERYDIR/cOS
    #rsync -aqz $STATEDIR/ ${RECOVERYDIR}
    cp -a $STATEDIR/cOS/active.img $RECOVERYDIR/cOS/recovery.img
    tune2fs -L COS_SYSTEM $RECOVERYDIR/cOS/recovery.img
}

prepare_passive() {
    echo "Preparing passive boot.."

    cp -a ${STATEDIR}/cOS/active.img ${STATEDIR}/cOS/passive.img
    tune2fs -L COS_PASSIVE ${STATEDIR}/cOS/passive.img
}

do_format()
{
    echo "Formatting drives.."

    if [ "$COS_INSTALL_NO_FORMAT" = "true" ]; then
        STATE=$(blkid -L COS_STATE || true)
        if [ -z "$STATE" ] && [ -n "$DEVICE" ]; then
            tune2fs -L COS_STATE $DEVICE
            STATE=$(blkid -L COS_STATE)
        fi
        OEM=$(blkid -L COS_OEM || true)
        STATE=$(blkid -L COS_STATE || true)
        RECOVERY=$(blkid -L COS_RECOVERY || true)
        BOOT=$(blkid -L COS_GRUB || true)
        return 0
    fi

    dd if=/dev/zero of=${DEVICE} bs=1M count=1
    parted -s ${DEVICE} mklabel ${PARTTABLE}
    if [ "$PARTTABLE" = "gpt" ]; then
        BOOT_NUM=1
        OEM_NUM=2
        STATE_NUM=3
        RECOVERY_NUM=4
        PERSISTENT_NUM=5
        parted -s ${DEVICE} mkpart primary fat32 0% 50MB # efi
        parted -s ${DEVICE} mkpart primary ext4 50MB 100MB # oem
        parted -s ${DEVICE} mkpart primary ext4 100MB 10100MB # active
        parted -s ${DEVICE} mkpart primary ext4 10100MB 18100MB # active
        parted -s ${DEVICE} mkpart primary ext4 18100MB 100% # persistent
        parted -s ${DEVICE} set 1 ${BOOTFLAG} on
    else
        BOOT_NUM=
        OEM_NUM=1
        STATE_NUM=2
        RECOVERY_NUM=3
        PERSISTENT_NUM=4
        parted -s ${DEVICE} mkpart primary ext4 0% 50MB # oem
        parted -s ${DEVICE} mkpart primary ext4 50MB 10050MB # active
        parted -s ${DEVICE} mkpart primary ext4 10050MB 18050MB # active
        parted -s ${DEVICE} mkpart primary ext4 18050MB 100% # persistent
        parted -s ${DEVICE} set 2 ${BOOTFLAG} on
    fi
   
    partprobe ${DEVICE} 2>/dev/null || true
    sleep 2

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
    dd if=/dev/zero of=${STATEDIR}/cOS/active.img bs=1M count=3240
    mkfs.ext4 ${STATEDIR}/cOS/active.img
    tune2fs -L COS_ACTIVE ${STATEDIR}/cOS/active.img
    mount -t ext4 -o loop ${STATEDIR}/cOS/active.img $TARGET

    mkdir -p ${TARGET}/boot
    if [ -n "${BOOT}" ]; then
        mkdir -p ${TARGET}/boot/efi
        mount ${BOOT} ${TARGET}/boot/efi
    fi
    mkdir -p ${TARGET}/boot/grub2
    mount ${STATE} ${TARGET}/boot/grub2

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

do_copy()
{
    echo "Copying cOS.."

    rsync -aqz --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' ${DISTRO}/ ${TARGET}
     if [ -n "$COS_INSTALL_CONFIG_URL" ]; then
        OEM=${TARGET}/oem/99_custom.yaml
        get_url "$COS_INSTALL_CONFIG_URL" $OEM
        chmod 600 ${OEM}
    fi
    mkdir -p $TARGET/usr/local/cloud-config
cat > $TARGET/usr/local/cloud-config/90_after_install.yaml <<EOF
# Execute this stage in the boot phase:
stages:
   fs.after:
     - name: "After install"
       files:
        - path: /etc/issue
          content: |
            Welcome to \S !
            IP address \4

            Login with user: root, password: cos
            To upgrade the system, run "cos-upgrade"
            To change this message permantly on boot, see /usr/local/cloud-config/90_after_install.yaml
          permissions: 0644
          owner: 0
          group: 0
EOF
    chmod 640 $TARGET/usr/local/cloud-config
    chmod 640 $TARGET/usr/local/cloud-config/90_after_install.yaml
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
    if [ -e "/dev/${TTY%,*}" ] && [ "$TTY" != tty1 ] && [ "$TTY" != console ] && [ -n "$TTY" ]; then
        sed -i "s!console=tty1!console=tty1 console=${TTY}!g" ${TARGET}/boot/grub2/grub.cfg
    fi

    if [ "$COS_INSTALL_NO_FORMAT" = "true" ]; then
        return 0
    fi

    if [ "$COS_INSTALL_FORCE_EFI" = "true" ]; then
        GRUB_TARGET="--target=x86_64-efi"
    fi

    mkdir ${TARGET}/proc || true
    mkdir ${TARGET}/boot || true
    mkdir ${TARGET}/dev || true
    mkdir ${TARGET}/sys || true
    mkdir ${TARGET}/tmp || true
    mount -t proc proc ${TARGET}/proc
    mount --rbind /dev ${TARGET}/dev
    mount --rbind /sys ${TARGET}/sys

    chroot ${TARGET} /bin/sh <<EOF
    grub2-install ${GRUB_TARGET} ${DEVICE}
EOF

    # XXX: This fails, while it shouldnt?
    # grub2-install ${GRUB_TARGET} --boot-directory=${TARGET}/boot --debug --removable ${DEVICE}
}

setup_style()
{
    if [ "$COS_INSTALL_FORCE_EFI" = "true" ] || [ -e /sys/firmware/efi ]; then
        PARTTABLE=gpt
        BOOTFLAG=esp
        if [ ! -e /sys/firmware/efi ]; then
            echo WARNING: installing EFI on to a system that does not support EFI
        fi
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

while [ "$#" -gt 0 ]; do
    case $1 in
        --no-format)
            COS_INSTALL_NO_FORMAT=true
            ;;
        --force-efi)
            COS_INSTALL_FORCE_EFI=true
            ;;
        --poweroff)
            COS_INSTALL_POWER_OFF=true
            ;;
        --debug)
            set -x
            COS_INSTALL_DEBUG=true
            ;;
        --config)
            shift 1
            COS_INSTALL_CONFIG_URL=$1
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

if [ -e /etc/environment ]; then
    source /etc/environment
fi

if [ -e /etc/os-release ]; then
    source /etc/os-release
fi

if [ -z "$COS_INSTALL_DEVICE" ]; then
    usage
fi

validate_progs
validate_device

trap cleanup exit

setup_style
do_format
do_mount
do_copy
install_grub

umount_target 2>/dev/null

prepare_recovery
prepare_passive

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
