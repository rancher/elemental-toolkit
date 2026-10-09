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
        "$systemdutildir"/systemd-fsck ln mkdir mount umount systemd-escape e2fsck

    inst_hook cmdline 30 "${moddir}/elemental-cmdline.sh"

    inst_script "${moddir}/elemental-fsck.sh" "/sbin/elemental-fsck"
    ln_r "$systemdutildir"/systemd-fsck \
        "/sbin/systemd-fsck"

    inst_script "${moddir}/sysroot-generator.sh" \
        "${systemdutildir}/system-generators/dracut-elemental-sysroot-generator"

    dracut_need_initqueue
}
