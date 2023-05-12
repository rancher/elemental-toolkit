ARG GO_VERSION=1.20
ARG COSIGN_VERSION=1.4.1-5
ARG LEAP_VERSION=15.4

FROM quay.io/costoolkit/releases-green:cosign-toolchain-$COSIGN_VERSION AS cosign-bin

FROM golang:${GO_VERSION}-alpine as elemental-bin
ENV CGO_ENABLED=0
WORKDIR /src/
# Add specific dirs to the image so cache is not invalidated when modifying non go files
ADD go.mod .
ADD go.sum .
RUN go mod download
ADD cmd cmd
ADD internal internal
ADD tests tests
ADD pkg pkg
ADD main.go .
# Set arg/env after go mod download, otherwise we invalidate the cached layers due to the commit changing easily
ARG ELEMENTAL_VERSION=0.0.1
ARG ELEMENTAL_COMMIT=""
ENV ELEMENTAL_VERSION=${ELEMENTAL_VERSION}
ENV ELEMENTAL_COMMIT=${ELEMENTAL_COMMIT}
RUN go build \
    -ldflags "-w -s \
    -X github.com/rancher/elemental-cli/internal/version.version=$ELEMENTAL_VERSION \
    -X github.com/rancher/elemental-cli/internal/version.gitCommit=$ELEMENTAL_COMMIT" \
    -o /usr/bin/elemental

FROM opensuse/leap:$LEAP_VERSION AS elemental
# This helps invalidate the cache on each build so the following steps are really run again getting the latest packages
# versions, as long as the elemental commit has changed
ARG ELEMENTAL_COMMIT=""
ENV ELEMENTAL_COMMIT=${ELEMENTAL_COMMIT}
RUN zypper ref && zypper dup -y
RUN zypper ref && zypper in -y xfsprogs parted util-linux-systemd e2fsprogs util-linux udev rsync grub2 dosfstools grub2-x86_64-efi squashfs mtools xorriso lvm2
COPY --from=elemental-bin /usr/bin/elemental /usr/bin/elemental
COPY --from=cosign-bin /usr/bin/cosign /usr/bin/cosign
# Fix for blkid only using udev on opensuse
RUN echo "EVALUATE=scan" >> /etc/blkid.conf
ENTRYPOINT ["/usr/bin/elemental"]
