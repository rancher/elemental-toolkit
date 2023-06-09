# run `make build` to build local/elemental-toolkit image
ARG TOOLKIT_REPO
ARG VERSION
FROM ${TOOLKIT_REPO}:${VERSION} as TOOLKIT

# OS base image of our choice
FROM opensuse/leap:15.4 as OS
ARG REPO
ARG VERSION
ENV VERSION=${VERSION}

# install kernel, systemd, dracut, grub2 and other required tools
RUN zypper --non-interactive install --no-recommends -- \
      kernel-default \
      device-mapper \
      dracut \
      grub2 \
      grub2-x86_64-efi \
      grub2-arm64-efi \
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
      findutils \
      lvm2 \
      tar \
      gzip \
      vim \
      which \
      less \
      sudo \
      sed

# Create non FHS paths
RUN mkdir -p /oem /system /etc/elemental/config.d

# Just add the elemental toolkit including
COPY --from=TOOLKIT /install-root /
COPY --from=TOOLKIT /usr/bin/elemental /usr/bin/elemental

# Enable cloud-init services
RUN systemctl enable cos-setup-rootfs.service && \
    systemctl enable cos-setup-initramfs.service && \
    systemctl enable cos-setup-reconcile.timer && \
    systemctl enable cos-setup-fs.service && \
    systemctl enable cos-setup-boot.service && \
    systemctl enable cos-setup-network.service

# Enable essential services
RUN systemctl enable NetworkManager.service

# Enable /tmp to be on tmpfs
RUN cp /usr/share/systemd/tmp.mount /etc/systemd/system

# Generate initrd with required elemental services
RUN dracut -f --regenerate-all

# Update os-release file with some metadata
RUN echo IMAGE_REPO=\"${REPO}\"         >> /etc/os-release && \
    echo IMAGE_TAG=\"${VERSION}\"           >> /etc/os-release && \
    echo IMAGE=\"${REPO}:${VERSION}\" >> /etc/os-release && \
    echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release && \
    echo GRUB_ENTRY_NAME=\"Elemental\" >> /etc/os-release

# Good for validation after the build
CMD /bin/bash
