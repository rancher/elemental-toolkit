#!/bin/bash
set -e

PROG=$0
PROGS="dd curl mkfs.ext4 mkfs.vfat fatlabel parted partprobe grub2-install"
DISTRO=/run/rootfsbase
ISOBOOT=/run/initramfs/live/boot
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
    echo "Usage: $PROG [--force-efi] [--debug] [--tty TTY] [--poweroff] [--no-format] [--config https://.../config.yaml] DEVICE"
    echo ""
    echo "Example: $PROG /dev/vda"
    echo ""
    echo "DEVICE must be the disk that will be partitioned (/dev/vda). If you are using --no-format it should be the device of the COS_ACTIVE partition (/dev/vda2)"
    echo ""
    echo "The parameters names refer to the same names used in the cmdline, refer to README.md for"
    echo "more info."
    echo ""
    exit 1
}

do_format()
{
    if [ "$COS_INSTALL_NO_FORMAT" = "true" ]; then
        STATE=$(blkid -L COS_ACTIVE || true)
        if [ -z "$STATE" ] && [ -n "$DEVICE" ]; then
            tune2fs -L COS_ACTIVE $DEVICE
            STATE=$(blkid -L COS_ACTIVE)
        fi
        OEM=$(blkid -L COS_OEM || true)
        STATE=$(blkid -L COS_ACTIVE || true)
        PERSISTENT=$(blkid -L COS_PERSISTENT || true)
        PASSIVE=$(blkid -L COS_PASSIVE || true)
        BOOT=$(blkid -L COS_GRUB || true)
        return 0
    fi

    dd if=/dev/zero of=${DEVICE} bs=1M count=1
    parted -s ${DEVICE} mklabel ${PARTTABLE}
    if [ "$PARTTABLE" = "gpt" ]; then
        BOOT_NUM=1
        OEM_NUM=2
        STATE_NUM=3
        PASSIVE_NUM=4
        PERSISTENT_NUM=5
        parted -s ${DEVICE} mkpart primary fat32 0% 50MB # efi
        parted -s ${DEVICE} mkpart primary ext4 50MB 100MB # oem
        parted -s ${DEVICE} mkpart primary ext4 100MB 2100MB # active
        parted -s ${DEVICE} mkpart primary ext4 2100MB 4100MB # passive
        parted -s ${DEVICE} mkpart primary ext4 4100MB 100% # persistent
    else
        BOOT_NUM=
        OEM_NUM=1
        STATE_NUM=2
        PASSIVE_NUM=3
        PERSISTENT_NUM=4
        parted -s ${DEVICE} mkpart primary ext4 0% 50MB # oem
        parted -s ${DEVICE} mkpart primary ext4 50MB 2050MB # active
        parted -s ${DEVICE} mkpart primary ext4 2050MB 4050MB # passive
        parted -s ${DEVICE} mkpart primary ext4 4050MB 100% # persistent
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
    OEM=${PREFIX}${OEM_NUM}
    PERSISTENT=${PREFIX}${PERSISTENT_NUM}
    PASSIVE=${PREFIX}${PASSIVE_NUM}

    mkfs.ext4 -F -L COS_ACTIVE ${STATE}
    if [ -n "${BOOT}" ]; then
        mkfs.vfat -F 32 ${BOOT}
        fatlabel ${BOOT} COS_GRUB
    fi

    mkfs.ext4 -F -L COS_OEM ${OEM}
    mkfs.ext4 -F -L COS_PERSISTENT ${PERSISTENT}
    mkfs.ext4 -F -L COS_PASSIVE ${PASSIVE}
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
   initramfs.after:
     - name: "After install"
       files:
        - path: /etc/issue
          content: |
            Welcome to cOS!
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
