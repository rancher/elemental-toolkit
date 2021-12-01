ARG LUET_VERSION=0.20.10
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM opensuse/leap:15.3

ENV COSIGN_EXPERIMENTAL=1
ENV COSIGN_REPOSITORY=raccos/releases-green

RUN zypper ref
RUN zypper in -y \
    # RPI
    raspberrypi-eeprom \
    bcm43xx-firmware \
    systemd-sysvinit \
    grub2-arm64-efi \
    iproute2 \
    squashfs \
    conntrack-tools \
    findutils \
    haveged \
    lsscsi \
    lvm2 \
    mdadm \
    multipath-tools \
    nfs-utils \
    open-iscsi \
    rng-tools \
    systemd \
    vim \
    parted \
    dracut \
    e2fsprogs \
    dosfstools \
    kernel-default \
    coreutils \
    less \
    device-mapper \
    grub2 \
    which \
    curl \
    open-vm-tools \
    python-azure-agent \
    qemu-guest-agent \
    wireless-tools \
    wpa_supplicant \
    iw \
    iproute2 \
    aaa_base-extras \
    iputils \
    kmod \
    libudev1 \
    vim-small \
    kernel-firmware-all \
    nano \
    gawk \
    haveged \
    tar \
    rsync \
    timezone \
    jq \
    gptfdisk

RUN zypper cc

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml
RUN curl -L https://github.com/mudler/luet/releases/download/0.20.10/luet-0.20.10-linux-arm64 --output /usr/bin/luet

RUN chmod +x /usr/bin/luet
RUN luet install -y meta/cos-verify

RUN luet install --plugin luet-cosign -y meta/cos-minimal

COPY files/ /


RUN mkinitrd

RUN ln -sf Image /boot/vmlinuz