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
installkernel() {
    instmods overlay
}

# called by dracut
install() {
    declare moddir=${moddir}
    declare systemdutildir=${systemdutildir}
    declare systemdsystemunitdir=${systemdsystemunitdir}

    inst_multiple \
        mount cut basename lsblk elemental

    inst_simple "/etc/elemental/config.yaml"

    inst_simple "/etc/systemd/system/elemental-rootfs.service" \
        "${systemdsystemunitdir}/elemental-rootfs.service"
    mkdir -p "${initdir}/${systemdsystemunitdir}/initrd-fs.target.wants"
    ln_r "../elemental-rootfs.service" \
        "${systemdsystemunitdir}/initrd-fs.target.wants/elemental-rootfs.service"

    dracut_need_initqueue
}
