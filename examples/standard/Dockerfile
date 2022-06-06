ARG LUET_VERSION=0.20.6
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM quay.io/costoolkit/releases-teal:cos-system-0.8.10-1

ENV COSIGN_EXPERIMENTAL=1

ARG ARCH=amd64
ENV ARCH=${ARCH}
ENV LUET_NOLOCK=true

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml

# Copy luet from the official images
COPY --from=luet /usr/bin/luet /usr/bin/luet

RUN luet install -y meta/cos-verify

RUN luet install --plugin luet-cosign -y meta/cos-light \
    utils/k9s \
    utils/nerdctl \
    toolchain/elemental-cli

COPY files/ /
RUN mkinitrd
