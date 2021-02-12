#!/bin/sh

prepare_workarea() {
  mount -t devtmpfs none /dev
  mount -t proc none /proc
  mount -t tmpfs none /tmp -o mode=1777
  mount -t sysfs none /sys

  mkdir -p /dev/pts
  mount -t devpts none /dev/pts

  # Create the new mountpoint in RAM.
  mount -t tmpfs none /mnt

  # Create folders for all critical file systems.
  mkdir /mnt/dev
  mkdir /mnt/sys
  mkdir /mnt/proc
  mkdir /mnt/tmp
  echo "Created folders for all critical file systems."
}

shell() {
  # Set flag which indicates that we have obtained controlling terminal.
  export PID1_SHELL=true

  # Interactive shell with controlling tty as PID 1.
  exec setsid sh
}

parse_cmdline() {
	read -r cmdline < /proc/cmdline

	for param in $cmdline ; do
		case $param in
			*=*) key=${param%%=*}; value=${param#*=} ;;
			'#'*) break ;;
			*) key=$param
		esac
		case $key in
			ro|rw) rwopt=$key ;;
			[![:alpha:]_]*|[[:alpha:]_]*[![:alnum:]_]*) ;;
			*) eval "$key"=${value:-y} ;;
		esac
		unset key value
	done

	case "$root" in
		/dev/* ) device=$root ;;
		UUID=* ) eval $root; device="/dev/disk/by-uuid/$UUID"  ;;
		LABEL=*) eval $root; device=$(blkid -t LABEL=$LABEL -o device) ;;
	esac
}

shell() {
	setsid sh -c 'exec sh </dev/tty1 >/dev/tty1 2>&1'
}

mount_root() {
	newroot=$1
	dev=$2
	if [ ! "$dev" ]; then
		echo "device not specified!"
		shell
	fi
	if ! mount -n ${rootfstype:+-t $rootfstype} -o ${rwopt:-ro}${rootflags:+,$rootflags} "$dev" "$newroot" ; then
		echo "cant mount: $dev"
		shell
	fi
}

search_overlay() {
  echo "Searching available devices for overlay content."
  for DEVICE in /dev/* ; do
    DEV=$(echo "${DEVICE##*/}")
    SYSDEV=$(echo "/sys/class/block/$DEV")

    case $DEV in
      *loop*) continue ;;
    esac

    if [ ! -d "$SYSDEV" ] ; then
      continue
    fi

    mkdir -p /tmp/mnt/device
    DEVICE_MNT=/tmp/mnt/device

    OVERLAY_DIR=""
    OVERLAY_MNT=""
    UPPER_DIR=""
    WORK_DIR=""

    mount $DEVICE $DEVICE_MNT 2>/dev/null
    if [ -f $DEVICE_MNT/rootfs.squashfs ] ; then
      #image
      echo -e "  Found \\e[94m/rootfs.squashfs\\e[0m image on device \\e[31m$DEVICE\\e[0m."

      mkdir -p /tmp/mnt/image
      IMAGE_MNT=/tmp/mnt/image

      LOOP_DEVICE=$(losetup -f)
      losetup $LOOP_DEVICE $DEVICE_MNT/rootfs.squashfs
      mount $LOOP_DEVICE $IMAGE_MNT -t squashfs
      OUT=$?
      if [ ! "$OUT" = "0" ] ; then
        echo -e "  \\e[31mMount failed (squashfs).\\e[0m"
      fi
      
      OVERLAY_DIR=$IMAGE_MNT
      OVERLAY_MNT=$IMAGE_MNT
      UPPER_DIR=$DEFAULT_UPPER_DIR
      WORK_DIR=$DEFAULT_WORK_DIR
    
    fi

    if [ "$OVERLAY_DIR" != "" -a "$UPPER_DIR" != "" -a "$WORK_DIR" != "" ] ; then
      mkdir -p $OVERLAY_DIR
      mkdir -p $UPPER_DIR
      mkdir -p $WORK_DIR


      modprobe overlay
      OUT=$?
      if [ ! "$OUT" = "0" ] ; then
        echo -e "  \\e[31mModprobe failed (overlay).\\e[0m"
      fi
      
      mount -t overlay -o lowerdir=$OVERLAY_DIR:/mnt,upperdir=$UPPER_DIR,workdir=$WORK_DIR none /mnt
      OUT=$?

      if [ ! "$OUT" = "0" ] ; then
        echo -e "  \\e[31mMount failed (overlayfs).\\e[0m"

        umount $OVERLAY_MNT 2>/dev/null
        rmdir $OVERLAY_MNT 2>/dev/null

        rmdir $DEFAULT_OVERLAY_DIR 2>/dev/null
        rmdir $DEFAULT_UPPER_DIR 2>/dev/null
        rmdir $DEFAULT_WORK_DIR 2>/dev/null
      else
        # All done, time to go.
        echo -e "  Overlay data from device \\e[31m$DEVICE\\e[0m has been merged."
        break
      fi
    else
      echo -e "  Device \\e[31m$DEVICE\\e[0m has no proper overlay structure."
    fi

    umount $DEVICE_MNT 2>/dev/null
    rm -rf $DEVICE_MNT 2>/dev/null
  done
}

delay() {
  # Give a chance to load usb and avoid races
  for x in $(cat /proc/cmdline); do
      case "$x" in
          rootdelay=*)
          sleep "${x#rootdelay=}"
          ;;
      esac
  done
}

mount_system() {
  # Parse cmdline for root device if the system was installed already
  parse_cmdline

  rootdevice=/mnt

  if [ -n "$device" ]; then
    # FIXME: Temporarly, until we separate COS_STATE from COS_PERSISTENT (/usr/local)
    rwopt="rw"
    mount_root $rootdevice $device

    persistent=$(blkid -t LABEL=COS_PERSISTENT -o device)
    if [ -n "$persistent" ]; then
      mount_root "$rootdevice/usr/local" $persistent
    fi

    oem=$(blkid -t LABEL=COS_OEM -o device)
    if [ -n "$oem" ]; then
      mount_root "$rootdevice/oem" $oem
    fi

  else
    search_overlay
  fi
}

switch_system() {
  if [ ! -e "/mnt/sbin/init" ]; then
    echo -e "  \\e[31m/sbin/init in rootfs not found, dropping to emergency shell\\e[0m"
    shell
  fi

  # Move critical file systems to the new mountpoint.
  mount --move /dev /mnt/dev
  mount --move /sys /mnt/sys
  mount --move /proc /mnt/proc
  mount --move /tmp /mnt/tmp
  echo -e "Mount locations \\e[94m/dev\\e[0m, \\e[94m/sys\\e[0m, \\e[94m/tmp\\e[0m and \\e[94m/proc\\e[0m have been moved to \\e[94m/mnt\\e[0m."

  chroot /mnt /usr/bin/cos-setup initramfs.after

  echo "Switching from initramfs root area to overlayfs root area."
  exec switch_root /mnt /sbin/init
}

export PATH=$PATH:/bin:/sbin:/usr/bin:/usr/sbin

DEFAULT_OVERLAY_DIR="/tmp/minimal/overlay"
DEFAULT_UPPER_DIR="/tmp/minimal/rootfs"
DEFAULT_WORK_DIR="/tmp/minimal/work"

rootfstype=auto

# Prepare work area with needed folder structures
prepare_workarea

# Delay boot if rootdelay is provided in cmdline
delay

# Mount the new system, or search for a squashfs (LiveCD)
mount_system

# Switch to the new system (if found)
switch_system
