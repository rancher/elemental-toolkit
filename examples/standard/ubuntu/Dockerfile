ARG LUET_VERSION=0.20.6
FROM quay.io/luet/base:$LUET_VERSION AS luet

FROM ubuntu:20.04

ENV COSIGN_EXPERIMENTAL=1
ENV COSIGN_REPOSITORY=raccos/releases-orange

RUN apt install -y \
	systemd \
	grub2-common \
	iproute2 \
	squashfs-tools \
	parted dracut \
	dracut-network \
	e2fsprogs \
	dosfstools \
	coreutils \
	debianutils \
	curl \
	nano \
	gawk \
	haveged \
	rsync

# Copy the luet config file pointing to the upgrade repository
COPY conf/luet.yaml /etc/luet/luet.yaml

# Copy luet from the official images
COPY --from=luet /usr/bin/luet /usr/bin/luet

RUN luet install -y meta/cos-verify

RUN luet install --plugin luet-cosign -y \
    meta/cos-minimal \
    utils/k9s \
    utils/nerdctl

COPY files/ /

RUN dracut --regenerate-all -f
RUN kernel=$(ls /boot/vmlinuz-* | head -n1) && \
    ln -sf "${kernel#/boot/}" /boot/vmlinuz
