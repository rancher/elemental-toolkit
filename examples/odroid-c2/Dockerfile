ARG LUET_VERSION=0.20.10
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM opensuse/leap:15.3

ENV COSIGN_EXPERIMENTAL=1
ENV COSIGN_REPOSITORY=raccos/releases-green

RUN zypper in -y \
    # Odroid
    kernel-default-extra \
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
    open-vm-tools \
    python-azure-agent \
    qemu-guest-agent \
    rng-tools \
    systemd \
    vim \
    parted \
    dracut \
    e2fsprogs \
    dosfstools \
    kernel-default \
    kernel-firmware-bnx2 \
    kernel-firmware-i915 \
    kernel-firmware-intel \
    kernel-firmware-iwlwifi \
    kernel-firmware-mellanox \
    kernel-firmware-network \
    kernel-firmware-platform \
    kernel-firmware-realtek \
    coreutils \
    less \
    device-mapper \
    grub2 \
    which \
    curl \
    nano \
    gawk \
    haveged \
    tar \
    rsync \
    timezone \
    jq \
    gptfdisk

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml
RUN curl -L https://github.com/mudler/luet/releases/download/0.20.10/luet-0.20.10-linux-arm64 --output /usr/bin/luet

RUN chmod +x /usr/bin/luet
RUN luet install -y meta/cos-verify

RUN luet install --plugin luet-cosign -y meta/cos-minimal

COPY files/ /
RUN mkinitrd
