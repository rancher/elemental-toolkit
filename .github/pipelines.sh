#!/bin/bash
set -ex

for i in $(ls config); do
    filename=$(basename -s .yaml $i)
    for arch in $(yq r config/"$i" -p "p" 'arches.[*]' | cut -d "." -f 2); do
      yq r config/"$i" "arches[$arch]" | gomplate --left-delim "{{{" --right-delim "}}}" -V --datasource config="stdin:/?type=application/yaml" --file build.yaml.gomplate --out workflows/build-"$filename"-"$arch".yaml
      sed -i '/^[[:space:]]*$/d' workflows/build-"$filename"-"$arch".yaml
    done
done