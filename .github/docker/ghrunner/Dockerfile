FROM ubuntu:latest
COPY entrypoint.sh /
ENV TZ=Europe/Berlin
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install \
    curl \
    libdigest-sha-perl \
    tzdata
RUN useradd -ms /bin/bash runner
WORKDIR /runner
RUN chown runner:runner /runner -Rfv
ENTRYPOINT ["/entrypoint.sh"]

