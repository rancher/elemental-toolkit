ARG LUET_VERSION=0.32.0
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM quay.io/costoolkit/releases-teal-arm64:cos-system-0.8.10

ENV COSIGN_EXPERIMENTAL=1
ENV LUET_NOLOCK=true

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml
COPY --from=luet /usr/bin/luet /usr/bin/luet
RUN luet install -y meta/cos-verify
RUN luet install --plugin luet-cosign -y \
    meta/cos-minimal \
    system/dracut-initrd

COPY files/ /

RUN mkinitrd
RUN ln -sf Image /boot/vmlinuz
