ARG LUET_VERSION=0.32.0
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM registry.opensuse.org/opensuse/leap:15.3

ENV COSIGN_EXPERIMENTAL=1
ENV LUET_NOLOCK=true

RUN zypper ref
RUN zypper in -y \
    # RPI
    aaa_base-extras \
    bcm43xx-firmware \
    conntrack-tools \
    coreutils \
    curl \
    device-mapper \
    dosfstools \
    dracut \
    e2fsprogs \
    findutils \
    gawk \
    gptfdisk \
    grub2 \
    grub2-arm64-efi \
    haveged \
    iproute2 \
    iputils \
    iw \
    jq \
    kernel-default \
    kernel-firmware-all \
    kmod \
    less \
    libudev1 \
    lsscsi \
    lvm2 \
    mdadm \
    multipath-tools \
    nano \
    NetworkManager \
    nfs-utils \
    open-iscsi \
    open-vm-tools \
    parted \
    python-azure-agent \
    qemu-guest-agent \
    raspberrypi-eeprom \
    rng-tools \
    rsync \
    squashfs \
    systemd \
    systemd-sysvinit \
    tar \
    timezone \
    vim \
    vim-small \
    which \
    wireless-tools \
    wpa_supplicant

RUN zypper cc

# Configure NetworkManager as default network management service
RUN zypper remove -y wicked
RUN systemctl disable wicked \
    && systemctl enable NetworkManager

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml
COPY --from=luet /usr/bin/luet /usr/bin/luet
RUN luet install -y meta/cos-verify
RUN luet install --plugin luet-cosign -y meta/cos-minimal

COPY files/ /

RUN mkinitrd
RUN ln -sf Image /boot/vmlinuz
