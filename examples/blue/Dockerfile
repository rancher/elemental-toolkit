# run `make build` to build local/elemental-toolkit image
ARG TOOLKIT_REPO=local/elemental-toolkit
ARG VERSION=latest
FROM ${TOOLKIT_REPO}:${VERSION} AS toolkit

# OS base image of our choice
FROM fedora:40 AS os
ARG REPO
ARG VERSION
ENV VERSION=${VERSION}

# install kernel, systemd, dracut, grub2 and other required tools
RUN echo "install_weak_deps=False" >> /etc/dnf/dnf.conf
RUN dnf install -y \
    kernel \
    NetworkManager \
    audit \
    coreutils \
    curl \
    device-mapper \
    dosfstools \
    dracut \
    dracut-live \
    dracut-network \
    e2fsprogs \
    efibootmgr \
    gawk \
    grub2 \
    grub2-efi-x64 \
    grub2-efi-x64-modules \
    grub2-pc \
    haveged \
    vim \
    openssh-server \
    openssh-clients \
    glibc-langpack-en \
    parted \
    gdisk \
    rsync \
    shim-x64 \
    squashfs-tools \
    systemd \
    tar \
    mtools \
    xorriso \
    patch \
    podman \
    which

# Create non FHS paths
RUN mkdir -p /oem /system

# Just add the elemental cli
COPY --from=toolkit /usr/bin/elemental /usr/bin/elemental

# This is for automatic testing purposes, do not do this in production.
RUN echo "PermitRootLogin yes" > /etc/ssh/sshd_config.d/rootlogin.conf

# Add elemental config dir
RUN mkdir -p /etc/elemental/config.d

# Generate initrd with required elemental services
RUN elemental --debug init -f

# Update os-release file with some metadata
RUN echo IMAGE_REPO=\"${REPO}\"             >> /etc/os-release && \
    echo IMAGE_TAG=\"${VERSION}\"           >> /etc/os-release && \
    echo IMAGE=\"${REPO}:${VERSION}\"       >> /etc/os-release && \
    echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release && \
    echo GRUB_ENTRY_NAME=\"Elemental\"      >> /etc/os-release

# Good for validation after the build
CMD ["/bin/bash"]
