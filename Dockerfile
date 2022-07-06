FROM registry.opensuse.org/opensuse/tumbleweed:latest

ENV LUET_NOLOCK=true
RUN zypper in -y curl make golang sudo docker git

COPY .github/build.go /build/
COPY .github/go.mod /build/
COPY .github/go.sum /build/
COPY scripts/get_luet.sh /build/
WORKDIR /build
RUN git config --global --add safe.directory /build

RUN ./get_luet.sh
RUN go build -o /usr/bin/build build.go
ENTRYPOINT ["/usr/bin/build"]