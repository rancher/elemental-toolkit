FROM ubuntu:latest
COPY entrypoint.sh /
ENV TZ=Europe/Berlin
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install \
    curl \
    libdigest-sha-perl \
    tzdata \
    sudo
RUN useradd -ms /bin/bash runner
RUN usermod -aG sudo runner
WORKDIR /runner
RUN chown runner:runner /runner -Rfv
ENTRYPOINT ["/entrypoint.sh"]

