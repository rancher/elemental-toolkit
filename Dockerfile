FROM opensuse/leap

ENV LUET_NOLOCK=true

RUN zypper in -y docker curl squashfs xorriso make which mtools dosfstools jq gptfdisk

COPY . /cOS
WORKDIR /cOS

RUN make deps

ENTRYPOINT ["/usr/bin/make"]
CMD ["build", "local-iso"]

