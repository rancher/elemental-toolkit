# run `make elemental` to build local/elemental image
FROM local/elemental-cli as CLI

# run `make elemental-toolkit` to build local/elemental-toolkit image
FROM local/elemental-toolkit as TOOLKIT

# OS base image of our choice
FROM opensuse/leap:15.4 as OS

# install kernel, systemd, dracut, grub2 and other basic tools
RUN zypper --non-interactive install -- \
  kernel-default \
  device-mapper \
  dracut \
  grub2 \
  grub2-x86_64-efi \
  shim \
  haveged \
  systemd \
  NetworkManager \
  timezone \
  parted \
  e2fsprogs \
  dosfstools \
  findutils \
  gptfdisk \
  rsync \
  squashfs \
  findutils \
  lvm2 \
  tar \
  gzip \
  vim \
  which \
  less \
  sed

# Create non FHS paths
RUN mkdir -p /oem /system

# Just add the elemental toolkit including
COPY --from=TOOLKIT /install-root /

# Just add the elemental-cli
COPY --from=CLI /usr/bin/elemental /usr/bin/elemental

# Enable cloud-init services
RUN systemctl enable cos-setup-rootfs.service && \
    systemctl enable cos-setup-initramfs.service && \
    systemctl enable cos-setup-reconcile.timer && \
    systemctl enable cos-setup-fs.service && \
    systemctl enable cos-setup-boot.service && \
    systemctl enable cos-setup-network.service

# Recreate the initrd according to the elemental config and modules
RUN mkinitrd

# Good for validation after the build
CMD /bin/bash
