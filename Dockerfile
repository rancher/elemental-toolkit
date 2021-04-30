FROM opensuse/leap

ENV LUET_NOLOCK=true

RUN zypper in -y docker curl squashfs xorriso make which

COPY . /cOS
WORKDIR /cOS

RUN make deps

ENTRYPOINT ["/usr/bin/make"]
CMD ["build", "local-iso"]

