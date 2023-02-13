#!/bin/bash -x

set -e

SCRIPT=$(realpath -s "${0}")
SCRIPTS_PATH=$(dirname "${SCRIPT}")
TESTS_PATH="${SCRIPTS_PATH}/../tests"

pushd "${TESTS_PATH}" > /dev/null
    export COS_HOST="$(vagrant ssh-config cos | grep HostName | sed -e 's|^.*HostName \(.*\)$|\1|g'):22"
    go run github.com/onsi/ginkgo/v2/ginkgo $@
popd
