#!/bin/bash

function doFsck {
    local partdev
    local partname
    local dev

    # Iterate over current device labels
    for partdev in $(lsblk -ln -o path,type | grep part | cut -d" " -f1); do
        partname=$(basename "${partdev}")
        [ -e "/tmp/elemental-fsck-${partname}" ] && continue
        > "/tmp/elemental-fsck-${partname}" 

        systemd-fsck "${partdev}"
    done
}

PATH=/usr/sbin:/usr/bin:/sbin:/bin

doFsck

exit 0
