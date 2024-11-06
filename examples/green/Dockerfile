# run `make build` to build local/elemental-toolkit image
ARG TOOLKIT_REPO=local/elemental-toolkit
ARG VERSION=latest
ARG OS_IMAGE=registry.opensuse.org/opensuse/tumbleweed
ARG OS_VERSION=latest

FROM ${TOOLKIT_REPO}:${VERSION} AS toolkit

# OS base image of our choice
FROM ${OS_IMAGE}:${OS_VERSION} AS os
ARG REPO
ARG VERSION
ENV REPO=${REPO}
ENV VERSION=${VERSION}

# Workaround for RISC-V, specific kernel might be needed for some boards
ARG ADD_REPO
ENV ADD_REPO=${ADD_REPO}

# Install kernel, systemd, dracut, grub2 and other required tools
RUN ARCH=$(uname -m); \
    zypper --non-interactive removerepo repo-update || true; \
    if [[ -n "${ADD_REPO}" ]]; then \
      zypper --non-interactive addrepo --enable --refresh ${ADD_REPO} added-repo; \
    fi; \
    if [[ "${ARCH}" != "riscv64" ]]; then \
      ADD_PKGS+=" shim"; \
      [[ "${ARCH}" == "aarch64" ]] && ARCH="arm64"; \
    fi; \
    zypper --non-interactive --gpg-auto-import-keys install --no-recommends -- \
      kernel-default \
      device-mapper \
      dracut \
      grub2 \
      grub2-${ARCH}-efi \
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
      sed \
      iproute2 \
      podman \
      audit \
      patterns-microos-selinux \
      btrfsprogs \
      btrfsmaintenance \
      snapper \
      xterm-resize \
      ${ADD_PKGS} && \
    zypper clean --all

# Just add the elemental cli
COPY --from=toolkit /usr/bin/elemental /usr/bin/elemental

# Enable essential services
RUN systemctl enable NetworkManager.service && \
    systemctl enable sshd.service

# Workaround to make sure there are no pending sysusers to be created (boo#1231244)
RUN systemd-sysusers

# This is for automatic testing purposes, do not do this in production.
RUN echo "PermitRootLogin yes" > /etc/ssh/sshd_config.d/rootlogin.conf

# SELinux in enforce mode
#RUN sed -i "s|SELINUX=.*|SELINUX=enforcing|g" /etc/selinux/config

# Add default snapshotter setup
ADD snapshotter.yaml /etc/elemental/config.d/snapshotter.yaml

# Generate initrd with required elemental services
RUN elemental --debug init --force

# Update os-release file with some metadata
RUN echo IMAGE_REPO=\"${REPO}\"         >> /etc/os-release && \
    echo IMAGE_TAG=\"${VERSION}\"           >> /etc/os-release && \
    echo IMAGE=\"${REPO}:${VERSION}\" >> /etc/os-release && \
    echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release && \
    echo GRUB_ENTRY_NAME=\"Elemental\" >> /etc/os-release

# Good for validation after the build
CMD ["/bin/bash"]
