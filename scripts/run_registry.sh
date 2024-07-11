#!/bin/bash  

set -e

SCRIPT=$(realpath -s "${0}")
SCRIPTS_PATH=$(dirname "${SCRIPT}")
TESTS_PATH=$(realpath -s "${SCRIPTS_PATH}/../tests")

: "${ELMNTL_PREFIX:=}"
: "${ELMNTL_REGCONF:=${TESTS_PATH}/${ELMNTL_PREFIX}test-registry.yaml}"
: "${ELMNTL_IPADDR=$(ip -4 addr show $(ip route | awk '/default/ { print $5 }') | grep -oP '(?<=inet\s)\d+(\.\d+){3}')}"
: "${ELMNTL_REGPORT=5000}"

function _abort {
    echo "$@" && exit 1
}

function start {
  if [ "$( docker container inspect -f '{{.State.Status}}' elmntl_registry )" = "running" ]; then
    return
  fi

  docker run --rm -d \
    --name elmntl_registry \
    -p "${ELMNTL_REGPORT}:${ELMNTL_REGPORT}" \
    registry:2
}

function stop {
  docker stop elmntl_registry || true
  ([ -f "${ELMNTL_REGCONF}" ] && rm "${ELMNTL_REGCONF}") || true
}

function push {
  local reg_img

  for img in "$@"; do
    reg_img="${ELMNTL_IPADDR}.sslip.io:${ELMNTL_REGPORT}/${img}"
    echo "Pushing '${img}' to '${reg_img}'"

    # Ugly hack around podman to circumvent the need of adding insecure registries
    # at /etc/docker/daemon.json when using the docker client 
    docker save "${img}" | podman load
    tag=${img##*:}
    imgID=$(podman images -n | grep "${tag}" | awk '{print $3}' | head -n1)
    podman tag "${imgID}" "${reg_img}"
    podman push --tls-verify=false "${reg_img}"
    podman rmi -f "${imgID}"
  done
}


function url {
  echo "${ELMNTL_IPADDR}.sslip.io:${ELMNTL_REGPORT}"
}

cmd=$1

case $cmd in
  start)
    start
    ;;
  stop)
    stop
    ;;
  push)
    shift
    push $@
    ;;
  url)
    url
    ;;
  *)
    _abort "Unknown command: ${cmd}"
    ;;
esac

exit 0
