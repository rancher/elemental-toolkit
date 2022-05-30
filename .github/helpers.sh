#!/bin/bash

YQ="${YQ:-/usr/bin/yq}"

cos_package_version() {
    echo $($YQ e '.packages.[0].version' packages/cos/collection.yaml)
}

cos_version() {
    SHA=$(echo $GITHUB_SHA | cut -c1-8 )
    echo $(cos_package_version)-g$SHA
}

create_remote_manifest() {
    MANIFEST=$1
    cp -rf $MANIFEST $MANIFEST.remote
    $YQ e -i '.luet.repositories[0].name="cOS"' $MANIFEST.remote
    $YQ e -i '.luet.repositories[0].enable=true' $MANIFEST.remote 
    $YQ e -i '.luet.repositories[0].priority=90' $MANIFEST.remote 
    $YQ e -i '.luet.repositories[0].type="docker"' $MANIFEST.remote 
    $YQ e -i ".luet.repositories[0].urls[0]=\"$FINAL_REPO\"" $MANIFEST.remote 
}
