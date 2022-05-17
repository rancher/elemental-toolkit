#!/bin/bash
if [ $(id -u) -ne 0 ]
  then echo "Please run the installer with sudo/as root"
  exit
fi

set -ex
export LUET_NOLOCK=true

LUET_ROOTFS=${LUET_ROOTFS:-/}
LUET_DATABASE_PATH=${LUET_DATABASE_PATH:-/var/luet/db}
LUET_DATABASE_ENGINE=${LUET_DATABASE_ENGINE:-boltdb}
LUET_CONFIG_PROTECT=${LUET_CONFIG_PROTECT:-1}
LUET_PACKAGE="${LUET_PACKAGE:-toolchain/luet}"
LUET_ARCH="${LUET_ARCH:-x86_64}"
LUET_INSTALL_FROM_COS_REPO="${LUET_INSTALL_FROM_COS_REPO:-true}"
# This is the luet bootstrap version. The latest available will be pulled later on
LUET_VERSION="${LUET_VERSION:-0.32.0}"

if [ -z "$LUET_ARCH" ]; then
    LUET_ARCH=$(uname -m)
fi

case $LUET_ARCH in
    amd64|x86_64)
        LUET_ARCH=amd64
        ;;
    arm64|aarch64)
        LUET_ARCH=arm64
        ;;
esac

if [[ "$LUET_ARCH" != "amd64" ]]; then
  REPO_URL="quay.io/costoolkit/releases-green-$LUET_ARCH"
else
  REPO_URL="quay.io/costoolkit/releases-green"
fi

if [[ "$DOCKER_INSTALL" == "true" ]]; then
  _DOCKER_IMAGE="$REPO_URL:luet-toolchain-$LUET_VERSION"
  echo "Using luet bootstrap version from docker image: ${_DOCKER_IMAGE}"
  docker run --entrypoint /usr/bin/luet --name luet ${_DOCKER_IMAGE} --version
  docker cp luet:/usr/bin/luet ./
  docker rm luet
else
  _LUET="luet-${LUET_VERSION}-linux-${LUET_ARCH}"
  _LUET_URL="https://github.com/mudler/luet/releases/download/${LUET_VERSION}/${_LUET}"
  _LUET_CHECKSUMS="https://github.com/mudler/luet/releases/download/${LUET_VERSION}/luet-${LUET_VERSION}-checksums.txt"
  echo "Using luet bootstrap version from URL: ${_LUET_URL}"
  curl -L $_LUET_CHECKSUMS --output checksums.txt
  curl -L $_LUET_URL --output luet
  sha=$(cat checksums.txt | grep ${_LUET} | awk '{ print $1 }')
  echo "$sha  luet" | sha256sum -c
  rm -rf checksum.txt
fi

chmod +x luet

mkdir -p /etc/luet/repos.conf.d || true
mkdir -p $LUET_DATABASE_PATH || true
mkdir -p /var/tmp/luet || true

cat > /etc/luet/luet.yaml <<EOF
general:
  debug: false
  enable_emoji: false
  spinner_charset: 9
system:
  rootfs: ${LUET_ROOTFS}
  database_path: "${LUET_DATABASE_PATH}"
  database_engine: "${LUET_DATABASE_ENGINE}"
  tmpdir_base: "/var/tmp/luet"
repositories:
- name: "cos"
  description: "cOS official"
  type: "docker"
  enable: true
  cached: true
  priority: 90
  verify: false
  urls:
  - ${REPO_URL}
EOF

if [[ "$LUET_INSTALL_FROM_COS_REPO" == "true" ]]; then
  ./luet install --no-spinner -y $LUET_PACKAGE
  rm -rf luet
else
  mv ./luet /usr/bin/luet
fi
