# run `make build` to build local/elemental-toolkit image
ARG TOOLKIT_REPO
ARG VERSION
FROM ${TOOLKIT_REPO}:${VERSION} AS TOOLKIT

# OS base image of our choice
FROM registry.opensuse.org/opensuse/leap:15.5 as OS
ARG REPO
ARG VERSION
ENV VERSION=${VERSION}

# install kernel, systemd, dracut, grub2 and other required tools
RUN ARCH=$(uname -m); \
    [[ "${ARCH}" == "aarch64" ]] && ARCH="arm64"; \
    zypper --non-interactive install --no-recommends -- \
      kernel-default \
      device-mapper \
      dracut \
      grub2 \
      grub2-${ARCH}-efi \
      shim \
      haveged \
      systemd \
      NetworkManager \
      openssh-server \
      openssh-clients \
      timezone \
      parted \
      e2fsprogs \
      dosfstools \
      mtools \
      xorriso \
      findutils \
      gptfdisk \
      rsync \
      squashfs \
      lvm2 \
      tar \
      gzip \
      vim \
      which \
      less \
      sudo \
      curl \
      iproute2 \
      btrfsprogs \
      btrfsmaintenance \
      snapper \
      podman \
      sed && \
    zypper clean --all

# Just add the elemental cli
COPY --from=TOOLKIT /usr/bin/elemental /usr/bin/elemental

# Enable essential services
RUN systemctl enable NetworkManager.service

# Enable /tmp to be on tmpfs
RUN cp /usr/share/systemd/tmp.mount /etc/systemd/system

# Add default network configuration
ADD 05_network.yaml /system/oem/05_network.yaml

# Generate initrd with required elemental services
RUN elemental init --debug --force

# Add default snapshotter setup
ADD snapshotter.yaml /etc/elemental/config.d/snapshotter.yaml

# Update os-release file with some metadata
RUN echo IMAGE_REPO=\"${REPO}\"         >> /etc/os-release && \
    echo IMAGE_TAG=\"${VERSION}\"           >> /etc/os-release && \
    echo IMAGE=\"${REPO}:${VERSION}\" >> /etc/os-release && \
    echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release && \
    echo GRUB_ENTRY_NAME=\"Elemental\" >> /etc/os-release

# Good for validation after the build
CMD /bin/bash
