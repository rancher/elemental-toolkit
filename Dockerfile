FROM opensuse/tumbleweed:latest

ENV LUET_NOLOCK=true

RUN zypper in -y docker curl squashfs xorriso make which mtools dosfstools jq gptfdisk git parted kpartx

COPY . /cOS
WORKDIR /cOS

RUN make deps

ENTRYPOINT ["/usr/bin/make"]
CMD ["build", "local-iso"]

