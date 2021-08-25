#!/bin/bash

function get_arch() {
    un=$(uname -p)
    if [ "$un" == "aarch64" ]; then
        echo "arm64"
    elif [ "$un" == "x86_64" ]; then
        echo "amd64"
    else
        echo $un
     fi
}

if [ $(id -u) -ne 0 ]
  then echo "Please run the installer with sudo/as root"
  exit
fi

set -ex
export LUET_NOLOCK=true

LUET_VERSION=$(curl -s https://api.github.com/repos/mudler/luet/releases/latest | grep tag_name | awk '{ print $2 }' | sed -e 's/\"//g' -e 's/,//g' || echo "0.9.24" )
LUET_ROOTFS=${LUET_ROOTFS:-/}
LUET_DATABASE_PATH=${LUET_DATABASE_PATH:-/var/luet/db}
LUET_DATABASE_ENGINE=${LUET_DATABASE_ENGINE:-boltdb}
LUET_CONFIG_PROTECT=${LUET_CONFIG_PROTECT:-1}
LUET_PACKAGE="${LUET_PACKAGE:-toolchain/luet}"

curl -L https://github.com/mudler/luet/releases/download/${LUET_VERSION}/luet-${LUET_VERSION}-linux-$(get_arch) --output luet
chmod +x luet

mkdir -p /etc/luet/repos.conf.d || true
mkdir -p $LUET_DATABASE_PATH || true
mkdir -p /var/tmp/luet || true

cat > /etc/luet/luet.yaml <<EOF
general:
  debug: false
  enable_emoji: false
system:
  rootfs: ${LUET_ROOTFS}
  database_path: "${LUET_DATABASE_PATH}"
  database_engine: "${LUET_DATABASE_ENGINE}"
  tmpdir_base: "/var/tmp/luet"
general:
   debug: false
   spinner_charset: 9
repositories:
- name: "cos"
  description: "cOS official"
  type: "docker"
  enable: true
  cached: true
  priority: 1
  verify: false
  urls:
  - "quay.io/costoolkit/releases-green"
EOF

./luet install -y $LUET_PACKAGE

rm -rf luet
