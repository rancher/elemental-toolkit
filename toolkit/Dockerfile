ARG GOLANG_IMAGE=1.17-bullseye

# Image to compile golang code (elemental-cli)
FROM golang:$GOLANG_IMAGE as GOLANG

ARG ELEMENTAL_REVISION=HEAD
ARG ELEMENTAL_CLI_REPO=https://github.com/rancher/elemental-cli

RUN echo $ELEMENTAL_CLI_REPO

RUN git clone --single-branch $ELEMENTAL_CLI_REPO && \
    cd elemental-cli && git checkout -b built $ELEMENTAL_REVISION

RUN cd elemental-cli && ELEMENTAL_VERSION=$(git describe --tags) && \
    ELEMENTAL_COMMIT=$(git rev-parse --short HEAD) && \
    go build \
    -ldflags "-w -s \
    -X github.com/rancher/elemental-cli/internal/version.version=$ELEMENTAL_VERSION \
    -X github.com/rancher/elemental-cli/internal/version.gitCommit=$ELEMENTAL_COMMIT" \
    -o /usr/bin/elemental

# Just use a small upstream image, probably busybox could be a nice alternative
FROM bash:latest as SHELL

# elemental-cli
COPY --from=GOLANG /usr/bin/elemental /install-root/usr/bin/elemental

# immutable-rootfs
COPY immutable-rootfs/30cos-immutable-rootfs /install-root/usr/lib/dracut/modules.d/30cos-immutable-rootfs

# init-setup
COPY init-setup/cos-setup* /install-root/usr/lib/systemd/system/
COPY init-setup/02-cos-setup-initramfs.conf /install-root/etc/dracut.conf.d/
# TODO create systemd enablement links?

# grub-config
COPY grub/config/grub.cfg /install-root/etc/cos/
COPY grub/config/bootargs.cfg /install-root/etc/cos/

# dracut-config
COPY dracut-config/50-elemental.conf /install-root/etc/dracut.conf.d/

# init-config
COPY init-config/oem /install-root/system/oem/
# TODO include args to filter few specific init files
# Essentials are 00, 05, 06, 07, 08 and 09. Others are not essential

# Good for validation
CMD ["/bin/sh"]
