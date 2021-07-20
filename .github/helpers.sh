#!/bin/bash
set -e

YQ="${YQ:-/usr/bin/yq}"

cos_package_version() {
    echo $($YQ r packages/cos/collection.yaml 'packages.[0].version')
}

cos_version() {
    SHA=$(echo $GITHUB_SHA | cut -c1-8 )
    echo $(cos_package_version)-g$SHA
}

create_remote_manifest() {
    MANIFEST=$1
    cp -rf $MANIFEST $MANIFEST.remote
    $YQ w -i $MANIFEST.remote 'luet.repositories[0].name' 'cOS'
    $YQ w -i $MANIFEST.remote 'luet.repositories[0].enable' true
    $YQ w -i $MANIFEST.remote 'luet.repositories[0].priority' 90
    $YQ w -i $MANIFEST.remote 'luet.repositories[0].type' 'docker'
    $YQ w -i $MANIFEST.remote 'luet.repositories[0].urls[0]' $FINAL_REPO
}

drop_recovery() {
    MANIFEST=$1
    $YQ d -i $MANIFEST 'packages.isoimage(.==recovery/cos-img)'
}