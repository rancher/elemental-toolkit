FROM opensuse/leap

RUN zypper in -y docker curl squashfs xorriso dosfstools make

RUN curl https://get.mocaccino.org/luet/get_luet_root.sh |  sh
ENV LUET_NOLOCK=true

RUN luet install -y repository/mocaccino-extra-stable
RUN luet install -y utils/jq utils/yq system/luet-devkit container/img

RUN zypper in -y which

COPY . /cOS
WORKDIR /cOS

ENTRYPOINT ["/usr/bin/make"]
CMD ["build", "local-iso"]

