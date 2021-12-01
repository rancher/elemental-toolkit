#!/bin/bash

set -e

function usage {
  echo "Usage: $PROG <template> [<environment file>]"
  echo ""
  echo "It replaces placeholders from the template according to the"
  echo "values of an environment variables file. On success template"
  echo "file is renamed by removing the '.tmpl' extension."
  echo ""
  echo "template:"
  echo "  the template file to render, requries '.tmpl' extension."
  echo ""
  echo "environment file:"
  echo "  the environment file to load, defaults to /etc/cos/config"
  echo ""
}

PROG=$0

if [ $# -gt 2 ] || [ $# -lt 1 ]; then
  usage
  exit 1
fi

TMPL_FILE=$1
ENV_FILE=${2:-/etc/cos/config}

if [ ! -e "${TMPL_FILE}" ]; then
  >&2 echo "Could not find template file ${TMPL_FILE}"
  exit 1
fi

if [ ! "${TMPL_FILE##*.}" = "tmpl" ]; then
  >&2 echo "Template file requires '.tmpl' extension"
  exit 1
fi

if [ -e "${ENV_FILE}" ]; then
  source "${ENV_FILE}"
else
  >&2 echo "Failed loading environment variables file ${ENV_FILE}"
  exit 1
fi

OUT_FILE="${TMPL_FILE%.tmpl}"

cp "${TMPL_FILE}" "${OUT_FILE}"

trap "rm -f ${OUT_FILE}" ERR

for var in $(compgen -v); do
  if grep -q -E "^${var}=" "${ENV_FILE}"; then
    sed -i "s|@${var}@|${!var}|g" "${OUT_FILE}"
  fi
done

rm -f "${TMPL_FILE}"
