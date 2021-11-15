ARG LUET_VERSION=0.20.10
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM quay.io/costoolkit/examples:odroid-c2-ubuntu-base

ENV COSIGN_EXPERIMENTAL=1
ENV COSIGN_REPOSITORY=raccos/releases-green

RUN apt-get update -y
RUN apt-get install -y dracut-core curl dracut-network rsync

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml

RUN curl -L https://github.com/mudler/luet/releases/download/0.20.10/luet-0.20.10-linux-arm64 --output /usr/bin/luet

RUN chmod +x /usr/bin/luet
RUN luet install -y meta/cos-verify

RUN luet install --plugin luet-cosign -y meta/cos-minimal

COPY files/ /

# Handle initrd and vmlinuz symlinks
RUN kernel=$(ls /lib/modules | head -n1) && \
    dracut -f "/boot/initrd-${kernel}" "${kernel}" && \
    ln -sf "initrd-${kernel}" /boot/initrd

RUN kernel=/boot/Image && \
    ln -sf "${kernel#/boot/}" /boot/vmlinuz