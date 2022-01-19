FROM quay.io/luet/base:latest as builder

COPY cos.yaml /etc/luet/luet.yaml

ENV USER=root

# We set the shell to /usr/bin/luet, as the base image doesn't have busybox, just luet
# and certificates to be able to correctly handle TLS requests.
SHELL ["/usr/bin/luet", "install", "-y", "--system-target", "/framework"]

# Each package we want to install needs a new line here
RUN meta/cos-core
RUN meta/cos-verify

FROM scratch
COPY --from=builder /framework /