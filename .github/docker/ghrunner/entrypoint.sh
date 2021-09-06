#!/bin/bash
ARCH=${ARCH:-x64}
VERSION=${VERSION:-2.280.3}
CHECKSUM=${CHECKSUM:-69dc323312e3c5547ba1e1cc46c127e2ca8ee7d7037e17ee6965ef6dac3c142b}
ORG=${ORG:-dragonchaser}
REPO=${REPO:-dockerhub-autobuild}
OS=${OS:-linux}

if [ -z "${ORG}" ]; then
    echo "missing ORG, bailing out!"
fi

if [ -z "${REPO}" ]; then
    echo "missing REPO, bailing out!"
fi

if [ -z "${TOKEN}" ]; then
    echo "missing TOKEN, bailing out!"
    exit 1
fi

FILE="actions-runner-${OS}-${ARCH}-${VERSION}.tar.gz"
curl -o ${FILE} -L https://github.com/actions/runner/releases/download/v${VERSION}/${FILE}
echo "${CHECKSUM}  ${FILE}" | shasum -a 256 -c
tar xzf ./${FILE}
./bin/installdependencies.sh
su runner -c "./config.sh --unattended --url https://github.com/${ORG}/${REPO} --token ${TOKEN} --name docker-runner-$(hostname) --labels=${ARCH},${OS},self-hosted"
su runner -c "./run.sh"
