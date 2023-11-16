#!/bin/bash

# called by dracut
check() {
    require_binaries "$systemdutildir"/systemd || return 1
    return 255
}

# called by dracut 
depends() {
    echo systemd rootfs-block dm fs-lib
    return 0
}

# called by dracut
install() {
    declare moddir=${moddir}
    declare systemdutildir=${systemdutildir}
    declare systemdsystemunitdir=${systemdsystemunitdir}

    inst_multiple -o \
        ln mkdir mount umount partx elemental

    inst_script "${moddir}/oem-generator.sh" \
        "${systemdutildir}/system-generators/dracut-elemental-oem-generator"

    dracut_need_initqueue
}

