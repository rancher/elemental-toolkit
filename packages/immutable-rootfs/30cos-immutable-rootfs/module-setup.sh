#!/bin/bash

# called by dracut
check() {
    return 255
}

# called by dracut
depends() {
    echo rootfs-block dm 
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
    inst_multiple \
        mount mountpoint yip cos-setup sort rmdir
    inst_hook cmdline 30 "${moddir}/parse-cos-overlay.sh"
    inst_hook initqueue/finished 30 "${moddir}/cos-wait-oem.sh"
    inst_hook pre-pivot 10 "${moddir}/cos-mount-layout.sh"
    inst_hook pre-pivot 20 "${moddir}/cos-config-launcher.sh"
    inst_script "${moddir}/cos-generator.sh" \
        "${systemdutildir}/system-generators/dracut-cos-generator"
    dracut_need_initqueue
}
