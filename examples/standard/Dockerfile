ARG LUET_VERSION=0.20.6
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM registry.suse.com/suse/sle-micro-rancher/5.2

ENV COSIGN_EXPERIMENTAL=1
ENV COSIGN_REPOSITORY=raccos/releases-teal

ARG ARCH=amd64
ENV ARCH=${ARCH}
RUN zypper in -y \
    bash-completion \
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
    grub2-i386-pc \
    grub2-x86_64-efi \
    haveged \
    iproute2 \
    iptables \
    iputils \
    issue-generator \
    jq \
    kernel-default \
    kernel-firmware-bnx2 \
    kernel-firmware-i915 \
    kernel-firmware-intel \
    kernel-firmware-iwlwifi \
    kernel-firmware-mellanox \
    kernel-firmware-network \
    kernel-firmware-platform \
    kernel-firmware-realtek \
    less \
    lsscsi \
    lvm2 \
    mdadm \
    multipath-tools \
    nano \
    NetworkManager\
    nfs-utils \
    open-iscsi \
    open-vm-tools \
    parted \
    pigz \
    policycoreutils \
    procps \
    python-azure-agent \
    qemu-guest-agent \
    rng-tools \
    rsync \
    squashfs \
    strace \
    systemd \
    systemd-sysvinit \
    tar \
    timezone \
    vim \
    which

RUN zypper cc

# Configure NetworkManager as default network management service
RUN zypper remove -y wicked
RUN systemctl disable wicked \
    && systemctl enable NetworkManager

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml

# Copy luet from the official images
COPY --from=luet /usr/bin/luet /usr/bin/luet

RUN luet install -y meta/cos-verify

RUN luet install --plugin luet-cosign -y meta/cos-minimal \
    utils/k9s \
    utils/nerdctl

COPY files/ /
RUN mkinitrd
