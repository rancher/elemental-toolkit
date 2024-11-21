# run `make build` to build local/elemental-toolkit image
ARG TOOLKIT_REPO=local/elemental-toolkit
ARG VERSION=latest
FROM ${TOOLKIT_REPO}:${VERSION} AS toolkit

# OS base image of our choice
FROM ubuntu:24.04 AS os
ARG REPO
ARG VERSION
ENV VERSION=${VERSION}

# install kernel, systemd, dracut, grub2 and other required tools
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  linux-generic \
  dmsetup \
  dracut-core \
  dracut-network \
  dracut-live \
  dracut-squash \
  grub2-common \
  grub-efi-amd64 \
  shim \
  shim-signed \
  haveged \
  systemd \
  systemd-sysv \
  systemd-timesyncd \
  systemd-resolved \
  openssh-server \
  openssh-client \
  tzdata \
  parted \
  e2fsprogs \
  dosfstools \
  mtools \
  xorriso \
  findutils \
  gdisk \
  rsync \
  squashfs-tools \
  lvm2 \
  vim \
  less \
  sudo \
  ca-certificates \
  curl \
  iproute2 \
  dbus-daemon \
  patch \
  netplan.io \
  locales \
  kbd \
  podman \
  btrfs-progs \
  btrfsmaintenance \
  xz-utils && \
  apt-get clean && rm -rf /var/lib/apt/lists/*

# Hack to prevent systemd-firstboot failures while setting keymap, this is known
# Debian issue (T_T) https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=790955
ARG KBD=2.6.4
RUN curl -L https://mirrors.edge.kernel.org/pub/linux/utils/kbd/kbd-${KBD}.tar.xz --output kbd-${KBD}.tar.xz && \
    tar xaf kbd-${KBD}.tar.xz && mkdir -p /usr/share/keymaps && cp -Rp kbd-${KBD}/data/keymaps/* /usr/share/keymaps/

# Symlink grub2-editenv
RUN ln -sf /usr/bin/grub-editenv /usr/bin/grub2-editenv

# Just add the elemental cli
COPY --from=toolkit /usr/bin/elemental /usr/bin/elemental

# Enable essential services
RUN systemctl enable systemd-networkd.service

# Enable /tmp to be on tmpfs
RUN cp /usr/share/systemd/tmp.mount /etc/systemd/system

# Generate en_US.UTF-8 locale, this the locale set at boot by
# the default cloud-init
RUN locale-gen --lang en_US.UTF-8

# Add default snapshotter setup
ADD snapshotter.yaml /etc/elemental/config.d/snapshotter.yaml

# Generate initrd with required elemental services
RUN elemental --debug init -f

# Update os-release file with some metadata
RUN echo IMAGE_REPO=\"${REPO}\"             >> /etc/os-release && \
    echo IMAGE_TAG=\"${VERSION}\"           >> /etc/os-release && \
    echo IMAGE=\"${REPO}:${VERSION}\"       >> /etc/os-release && \
    echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release && \
    echo GRUB_ENTRY_NAME=\"Elemental\"      >> /etc/os-release

# Adding specific network configuration based on netplan
ADD 05_network.yaml /system/oem/05_network.yaml

# Arrange bootloader binaries into /usr/lib/elemental/bootloader
# this way elemental installer can easily fetch them
RUN mkdir -p /usr/lib/elemental/bootloader && \
    cp /usr/lib/grub/x86_64-efi-signed/grubx64.efi.signed /usr/lib/elemental/bootloader/grubx64.efi && \
    cp /usr/lib/shim/shimx64.efi.signed.latest /usr/lib/elemental/bootloader/shimx64.efi && \
    cp /usr/lib/shim/mmx64.efi /usr/lib/elemental/bootloader/mmx64.efi

# Good for validation after the build
CMD ["/bin/bash"]
