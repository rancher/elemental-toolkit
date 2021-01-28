#!/bin/bash
set -e

PROG=$0
PROGS="dd curl mkfs.ext4 mkfs.vfat fatlabel parted partprobe grub2-install"
DISTRO=/tmp/mnt/image
ISOBOOT=/tmp/mnt/device/boot
TARGET=/run/cos/target

if [ "$COS_DEBUG" = true ]; then
    set -x
fi

cleanup2()
{
    if [ -n "${TARGET}" ]; then
        umount ${TARGET}/proc || true
        umount ${TARGET}/dev || true
        umount ${TARGET}/sys || true
        umount ${TARGET}/boot/efi || true
        umount ${TARGET} || true

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
    echo "Usage: $PROG [--force-efi] [--debug] [--tty TTY] [--poweroff] [--no-format] DEVICE"
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

do_format()
{
    if [ "$COS_INSTALL_NO_FORMAT" = "true" ]; then
        STATE=$(blkid -L COS_STATE || true)
        if [ -z "$STATE" ] && [ -n "$DEVICE" ]; then
            tune2fs -L COS_STATE $DEVICE
            STATE=$(blkid -L COS_STATE)
        fi

        return 0
    fi

    dd if=/dev/zero of=${DEVICE} bs=1M count=1
    parted -s ${DEVICE} mklabel ${PARTTABLE}
    if [ "$PARTTABLE" = "gpt" ]; then
        BOOT_NUM=1
        STATE_NUM=2
        parted -s ${DEVICE} mkpart primary fat32 0% 50MB
        parted -s ${DEVICE} mkpart primary ext4 50MB 750MB
    else
        BOOT_NUM=
        STATE_NUM=1
        parted -s ${DEVICE} mkpart primary ext4 0% 700MB
    fi
    parted -s ${DEVICE} set 1 ${BOOTFLAG} on
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

    mkfs.ext4 -F -L COS_STATE ${STATE}
    if [ -n "${BOOT}" ]; then
        mkfs.vfat -F 32 ${BOOT}
        fatlabel ${BOOT} COS_GRUB
    fi
}

do_mount()
{
    mkdir -p ${TARGET}
    mount ${STATE} ${TARGET}
    mkdir -p ${TARGET}/boot
    if [ -n "${BOOT}" ]; then
        mkdir -p ${TARGET}/boot/efi
        mount ${BOOT} ${TARGET}/boot/efi
    fi
}

do_copy()
{
    #tar cf - -C ${DISTRO} cos | tar xvf - -C ${TARGET}
    rsync -aqz --exclude='mnt' --exclude='proc' --exclude='sys' --exclude='dev' --exclude='tmp' ${DISTRO}/ ${TARGET}
    cp -rf ${ISOBOOT}/rootfs.xz ${TARGET}/boot/rootfs.xz
    pushd ${TARGET}/boot >/dev/null
    ln -s rootfs.xz Initrd
    # ln -s vmlinuz-* bzImage
    popd >/dev/null
}

install_grub()
{
    if [ "$COS_INSTALL_DEBUG" ]; then
        GRUB_DEBUG="cos.debug"
    fi

    # FIXME: vmlinuz-vanilla needs to be a generic one. e.g. vmlinuz
    mkdir -p ${TARGET}/boot/grub2
    cat > ${TARGET}/boot/grub2/grub.cfg << EOF
set default=0
set timeout=10

set gfxmode=auto
set gfxpayload=keep
insmod all_video
insmod gfxterm

menuentry "cOS" {
  linux /boot/vmlinuz-vanilla console=tty1
  initrd /boot/Initrd
}
EOF
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
    echo "GRUB_CMDLINE_LINUX_DEFAULT=\"root=${STATE}\"" >> /etc/default/grub
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

create_opt()
{
    mkdir -p "${TARGET}/cos/data/opt"
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
create_opt

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
