#!/bin/bash
set -e

BASE_URL="${BASE_URL:-}"

publicpath="${ROOT_DIR}/public"

rm -rf "${publicpath}" || true
[[ ! -d "${publicpath}" ]] && mkdir -p "${publicpath}"

# Note: It needs
# sudo npm install -g postcss-cli
#

npm install -D --save autoprefixer
npm install -D --save postcss-cli

HUGO_ENV="production" hugo --gc -b "${BASE_URL}" -s "${ROOT_DIR}/docs" -d "${publicpath}"

if [ -e docs/CNAME ]; then
    cp -rfv docs/CNAME "${publicpath}"
fi