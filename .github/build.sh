#!/bin/bash

sudo -E make PACKAGES="$1" build
rt=$?

docker system prune --force --volumes --all

exit $rt