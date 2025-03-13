ARG BASE_OS_IMAGE=registry.opensuse.org/opensuse/leap
ARG BASE_OS_VERSION=15.5
ARG GO_VERSION=1.23

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine as elemental-bin

ENV CGO_ENABLED=0
WORKDIR /src/

# Add specific dirs to the image so cache is not invalidated when modifying non go files
ADD go.mod .
ADD go.sum .
ADD vendor vendor

# Do not download neither verify vendored folder, yip should be bumped to a recent version as
# some of its dependencies are no longer available upstream. Relay entirely on vendored folder
# RUN go mod download

ADD cmd cmd
ADD internal internal
ADD pkg pkg
ADD main.go .

# Set arg/env after go mod download, otherwise we invalidate the cached layers due commit hash changes
ARG ELEMENTAL_VERSION=0.0.1
ARG ELEMENTAL_COMMIT=""
ENV ELEMENTAL_VERSION=${ELEMENTAL_VERSION}
ENV ELEMENTAL_COMMIT=${ELEMENTAL_COMMIT}
ARG TARGETOS 
ARG TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go generate ./...
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags "-w -s \
    -X github.com/rancher/elemental-toolkit/v2/internal/version.version=${ELEMENTAL_VERSION} \
    -X github.com/rancher/elemental-toolkit/v2/internal/version.gitCommit=${ELEMENTAL_COMMIT}" \
    -o /usr/bin/elemental

FROM ${BASE_OS_IMAGE}:${BASE_OS_VERSION} AS elemental-toolkit
# This helps invalidate the cache on each build so the following steps are really run again getting the latest packages
# versions, as long as the elemental commit has changed
ARG ELEMENTAL_COMMIT=""
ENV ELEMENTAL_COMMIT=${ELEMENTAL_COMMIT}

ARG TARGETARCH

RUN ARCH=$(uname -m); \
    [[ "${ARCH}" == "aarch64" ]] && ARCH="arm64"; \
    zypper --non-interactive removerepo repo-update || true; \
    zypper install -y --no-recommends xfsprogs \
        parted \
        util-linux-systemd \
        e2fsprogs \
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
        lvm2 && \
    zypper cc -a

# Copy the built CLI
COPY --from=elemental-bin /usr/bin/elemental /usr/bin/elemental

# Fix for blkid only using udev on opensuse
RUN echo "EVALUATE=scan" >> /etc/blkid.conf

ENTRYPOINT ["/usr/bin/elemental"]

# Add to /system/oem folder so install/upgrade/reset hooks will run when running this container.
# Needed for boot-assessment
COPY pkg/features/embedded/cloud-config-essentials/system/oem /system/oem/
