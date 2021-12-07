#!/bin/bash

sudo -E make PACKAGES="$1" build

docker images --filter="reference=$REPO_CACHE" --format='{{.Repository}}:{{.Tag}}' | xargs -r docker rmi --force
docker image prune --force