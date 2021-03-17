#!/bin/bash

#======================================
# Functions
#--------------------------------------

function waitForOEM {
    local timeout="$1"
    local timeout_file="/tmp/cos-oem-timestamp"
    local current_time

    current_time="$(cat /proc/uptime)"
    current_time="${current_time%%.*}"

    if [ ! -f "${timeout_file}" ]; then
        echo "$((current_time + $timeout))" > "${timeout_file}"
    fi

    if [ ! -e "/dev/disk/by-label/${oem_label}" ] && [ "${current_time}" -lt "$(cat ${timeout_file})" ]; then
        info "Waiting for COS_OEM device"
        return 1
    fi
}

type info >/dev/null 2>&1 || . /lib/dracut-lib.sh
PATH=/usr/sbin:/usr/bin:/sbin:/bin

declare root=${root}
declare cos_oem_timeout=${cos_oem_timeout}
declare oem_label="COS_OEM"

[ ! "${root%%:*}" = "cos" ] && return 0

waitForOEM "${cos_oem_timeout}" || return 1

return 0
