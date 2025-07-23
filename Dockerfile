ARG GO_VERSION=1.20
ARG LEAP_VERSION=15.5

FROM golang:${GO_VERSION}-alpine as elemental-bin

ENV CGO_ENABLED=0
WORKDIR /src/

# needed for `go mod download`
RUN apk add git
# Add specific dirs to the image so cache is not invalidated when modifying non go files
ADD go.mod .
ADD go.sum .
ADD vendor vendor
ADD cmd cmd
ADD internal internal
ADD pkg pkg
ADD main.go .

# Set arg/env after go mod download, otherwise we invalidate the cached layers due to the commit changing easily
ARG ELEMENTAL_VERSION=0.0.1
ARG ELEMENTAL_COMMIT=""
ENV ELEMENTAL_VERSION=${ELEMENTAL_VERSION}
ENV ELEMENTAL_COMMIT=${ELEMENTAL_COMMIT}
RUN go build \
    -ldflags "-w -s \
    -X github.com/rancher/elemental-toolkit/internal/version.version=$ELEMENTAL_VERSION \
    -X github.com/rancher/elemental-toolkit/internal/version.gitCommit=$ELEMENTAL_COMMIT" \
    -o /usr/bin/elemental

FROM opensuse/leap:$LEAP_VERSION AS elemental
# This helps invalidate the cache on each build so the following steps are really run again getting the latest packages
# versions, as long as the elemental commit has changed
ARG ELEMENTAL_COMMIT=""
ENV ELEMENTAL_COMMIT=${ELEMENTAL_COMMIT}

RUN ARCH=$(uname -m); \
    if [[ $ARCH == "aarch64" ]]; then ARCH="arm64"; fi; \
    zypper install -y --no-recommends xfsprogs \
        parted \
        util-linux-systemd \
        e2fsprogs \
        util-linux \
        udev \
        rsync \
        grub2 \
        dosfstools \
        grub2-${ARCH}-efi \
        squashfs \
        mtools \
        xorriso \
        cosign \
        gptfdisk \
        lvm2

COPY --from=elemental-bin /usr/bin/elemental /usr/bin/elemental

# Fix for blkid only using udev on opensuse
RUN echo "EVALUATE=scan" >> /etc/blkid.conf
ENTRYPOINT ["/usr/bin/elemental"]

# Add to /system/oem folder so install/upgrade/reset hooks will run when running this container.
# Needed for boot-assessment
COPY pkg/features/embedded/cloud-config-essentials/system/oem /system/oem/
