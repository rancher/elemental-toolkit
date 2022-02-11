ARG LUET_VERSION=0.20.6
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM fedora:35

ENV COSIGN_EXPERIMENTAL=1
ENV COSIGN_REPOSITORY=raccos/releases-orange

RUN dnf install -y \
    NetworkManager \
    squashfs-tools \ 
    dracut-live \
    efibootmgr \
    audit \
    kernel \
    systemd \
    parted \
    dracut \
    e2fsprogs \
    dosfstools \
    coreutils \
    device-mapper \
    grub2 \
    which \
    curl \
    nano \
    gawk \
    haveged \
    tar \
    rsync

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml

# Copy luet from the official images
COPY --from=luet /usr/bin/luet /usr/bin/luet

RUN luet install -y meta/cos-verify

RUN luet install --plugin luet-cosign -y \
    meta/cos-minimal \
    utils/k9s \
    utils/nerdctl

COPY files/ /

RUN dracut --regenerate-all -f
RUN kernel=$(ls /boot/vmlinuz-* | head -n1) && \
    ln -sf "${kernel#/boot/}" /boot/vmlinuz

RUN kernel=$(ls /lib/modules | head -n1) && \
    cd /boot && \
    ln -sf *.img initrd
