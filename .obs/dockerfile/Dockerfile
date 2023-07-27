# SPDX-License-Identifier: Apache-2.0
# Define the names/tags of the container
#!BuildTag: rancher/elemental-builder-image/%%SLEMICRO_VERSION%%:latest
#!BuildTag: rancher/elemental-builder-image/%%SLEMICRO_VERSION%%:%VERSION%
#!BuildTag: rancher/elemental-builder-image/%%SLEMICRO_VERSION%%:%VERSION%-%RELEASE%
#

ARG SLE_VERSION

FROM suse/sle15:$SLE_VERSION

RUN zypper in -y elemental-cli xfsprogs parted e2fsprogs udev rsync grub2 dosfstools squashfs mtools xorriso lvm2

ARG SLEMICRO_VERSION
ARG BUILD_REPO=%%IMG_REPO%%
ARG IMAGE_REPO=$BUILD_REPO/rancher/elemental-builder-image/$SLEMICRO_VERSION

# Define labels according to https://en.opensuse.org/Building_derived_containers
# labelprefix=com.rancher.elemental
LABEL org.opencontainers.image.title="Rancher Elemental Builder"
LABEL org.opencontainers.image.description="Provides Elemental Client and required dependencies for installation media building"
LABEL org.opencontainers.image.version="%VERSION%"
LABEL org.opencontainers.image.url="https://github.com/rancher/elemental-toolkit"
LABEL org.opencontainers.image.created="%BUILDTIME%"
LABEL org.opencontainers.image.vendor="SUSE LLC"
LABEL org.opensuse.reference=$IMAGE_REPO
LABEL org.openbuildservice.disturl="%DISTURL%"
LABEL com.suse.supportlevel="techpreview"
# endlabelprefix

# Fix for blkid only using udev on opensuse
RUN echo "EVALUATE=scan" >> /etc/blkid.conf

# Save some space
RUN zypper clean --all && \
    rm -rf /var/log/update* && \
    >/var/log/lastlog && \
    rm -rf /boot/vmlinux*
