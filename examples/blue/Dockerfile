# run `make build` to build local/elemental-toolkit image
ARG TOOLKIT_REPO
ARG VERSION
FROM ${TOOLKIT_REPO}:${VERSION} as TOOLKIT

# OS base image of our choice
FROM fedora:39 as OS
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
    rsync \
    shim-x64 \
    squashfs-tools \
    systemd \
    tar \
    mtools \
    xorriso \
    patch \
    which

# Create non FHS paths
RUN mkdir -p /oem /system

# Just add the elemental cli
COPY --from=TOOLKIT /usr/bin/elemental /usr/bin/elemental

# This is patches are fix upstream dracut, see https://github.com/dracutdevs/dracut/pull/2525
ADD patches /

RUN cd /usr/lib/dracut && \
    patch -p 1 -f -i /0001-fix-dmsquash-live-restore-compatibility-with-earlier.patch && \
    patch -p 1 -f -i /0001-fix-overlayfs-split-overlayfs-mount-in-two-steps.patch && \
    rm /*.patch

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
CMD /bin/bash
