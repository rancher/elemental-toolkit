FROM ubuntu:latest
COPY entrypoint.sh /
ENV TZ=Europe/Berlin
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install \
    curl \
    libdigest-sha-perl \
    tzdata \
    sudo \
    git \
    make \
    jq \
    python2-minimal python2 dh-python 2to3 python-is-python3 python3-yaml \
    iputils-ping \
    apt-transport-https \
    ca-certificates \
    gnupg-agent \
    software-properties-common \
    && curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add - \
    && add-apt-repository \
   "deb [arch=$(dpkg --print-architecture)] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable" \
    && apt-get update && apt-get install -y mtools docker-ce docker-ce-cli && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -ms /bin/bash runner
RUN usermod -aG sudo runner && usermod -aG docker runner
RUN echo "%sudo ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
WORKDIR /runner
RUN chown runner:runner /runner -Rfv
RUN mkdir /home/runner/.docker
RUN chown runner:runner /home/runner -Rfv

ENTRYPOINT ["/entrypoint.sh"]

