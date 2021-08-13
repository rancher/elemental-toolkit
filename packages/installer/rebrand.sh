#!/bin/bash
# This script allows to rebrand cOS

is_mounted() {
  mountpoint -q $1
}

rebrand_grub_menu() {

	local grub_entry="$1"

	STATEDIR=$(blkid -L COS_STATE)
	mkdir -p /run/boot
	
	if ! is_mounted; then
	   mount $STATEDIR /run/boot
	fi
	local grub_file=/run/boot/grub2/grub.cfg

	if [ ! -e "$grub_file" ]; then
	   grub_file="/run/boot/grub/grub.cfg"
	fi

	if [ ! -e "$grub_file" ]; then
	   echo "Grub config file not found"
	   exit 1
	fi

	sed -i "s/menuentry \"cOS/menuentry \"$grub_entry/g" "$grub_file"
}

cleanup2()
{
    sync
    umount ${STATEDIR}
}

cleanup()
{
    EXIT=$?
    cleanup2 2>/dev/null || true
    return $EXIT
}

if [ -e "/etc/cos/config" ]; then
  source /etc/cos/config
fi

grub_entry="${GRUB_ENTRY_NAME:-cOS}"

trap cleanup exit

rebrand_grub_menu "$grub_entry"
