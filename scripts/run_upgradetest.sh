#!/bin/bash 

set -e

SCRIPT=$(realpath -s "${0}")
SCRIPTS_PATH=$(dirname "${SCRIPT}")
ROOT_PATH=$(realpath -s "${SCRIPTS_PATH}/..")

: "${ELMNTL_OLDPASS:=cos}"

function _abort {
    echo "$@" && exit 1
}

function start {
  local ginkgo="$1"
  local ginkgo_args="$2"
  local toolkit_img="$3"
  local upgrade_img="$4"
  local reg_url

  export VM_PID=$(${SCRIPTS_PATH}/run_vm.sh vmpid)
  export COS_PASS=${ELMNTL_OLDPASS}

  reg_url=$(${SCRIPTS_PATH}/run_registry.sh url)

  pushd "${ROOT_PATH}" > /dev/null
    go run ${ginkgo} ${ginkgo_args} ./tests/wait-active
    go run ${ginkgo} ${ginkgo_args} ./tests/upgrade -- \
      --toolkit-image=docker://${reg_url}/${toolkit_img} --upgrade-image=docker://${reg_url}/${upgrade_img} 
  popd > /dev/null
}

cmd=$1

case $cmd in
  start)
    shift
    if [[ $# -ne 4 ]]; then
      _abort "Wrong number of arguments"
    fi
    start "$1" "$2" "$3" "$4"
    ;;
  *)
    _abort "Unknown command: ${cmd}"
    ;;
esac

exit 0
