# Just use a small upstream image, probably busybox could be a nice alternative
FROM bash:latest as SHELL

# immutable-rootfs
COPY immutable-rootfs/30cos-immutable-rootfs /install-root/usr/lib/dracut/modules.d/30cos-immutable-rootfs

# init-setup
COPY init-setup/cos-setup* /install-root/usr/lib/systemd/system/
COPY init-setup/02-cos-setup-initramfs.conf /install-root/etc/dracut.conf.d/
# TODO create systemd enablement links?

# grub-config
COPY grub/config/grub.cfg /install-root/etc/cos/
COPY grub/config/bootargs.cfg /install-root/etc/cos/

# dracut-config
COPY dracut-config/50-elemental.conf /install-root/etc/dracut.conf.d/

# init-config
COPY init-config/oem /install-root/system/oem/
# TODO include args to filter few specific init files
# Essentials are 00, 05, 06, 07, 08 and 09. Others are not essential

# Good for validation
CMD ["/bin/sh"]

