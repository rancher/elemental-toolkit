# run `make build` to build local/elemental-toolkit image
ARG TOOLKIT_REPO
ARG VERSION
FROM ${TOOLKIT_REPO}:${VERSION} as TOOLKIT

# OS base image of our choice
FROM ubuntu:23.04 as OS
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
  xz-utils

# Create non FHS paths
RUN mkdir -p /oem /system

# Just add the elemental cli
COPY --from=TOOLKIT /usr/bin/elemental /usr/bin/elemental

# Enable essential services
RUN systemctl enable systemd-networkd.service && \
    systemctl enable systemd-resolved.service

# Enable /tmp to be on tmpfs
RUN cp /usr/share/systemd/tmp.mount /etc/systemd/system

# This is patches are fix upstream dracut, see https://github.com/dracutdevs/dracut/pull/2525
ADD patches /

RUN cd /usr/lib/dracut && \
    patch -p 1 -f -i /0001-fix-dmsquash-live-restore-compatibility-with-earlier.patch && \
    patch -p 1 -f -i /0001-fix-overlayfs-split-overlayfs-mount-in-two-steps.patch && \
    rm /*.patch

# Generate initrd with required elemental services
RUN elemental --debug init -f

# Update os-release file with some metadata
RUN echo IMAGE_REPO=\"${REPO}\"         >> /etc/os-release && \
    echo IMAGE_TAG=\"${VERSION}\"           >> /etc/os-release && \
    echo IMAGE=\"${REPO}:${VERSION}\" >> /etc/os-release && \
    echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release
RUN echo GRUB_ENTRY_NAME=\"Elemental\" >> /etc/os-release

# Arrange bootloader binaries into /usr/lib/elemental/bootloader
# this way elemental installer can easily fetch them
RUN mkdir -p /usr/lib/elemental/bootloader && \
    cp /usr/lib/grub/x86_64-efi-signed/gcdx64.efi.signed /usr/lib/elemental/bootloader/grubx64.efi && \
    cp /usr/lib/shim/shimx64.efi.signed.latest /usr/lib/elemental/bootloader/shimx64.efi && \
    cp /usr/lib/shim/mmx64.efi /usr/lib/elemental/bootloader/mmx64.efi

# Good for validation after the build
CMD /bin/bash
