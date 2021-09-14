#!/bin/bash
set -ex
YQ="${YQ:-yq}"

for i in $(ls config); do
    filename=$(basename -s .yaml $i)
    for arch in $($YQ r config/"$i" -p "p" 'arches.[*]' | cut -d "." -f 2); do
      $YQ r config/"$i" "arches[$arch]" | gomplate --left-delim "{{{" --right-delim "}}}" -V --datasource config="stdin:/?type=application/yaml" --file build.yaml.gomplate --out workflows/build-"$filename"-"$arch".yaml
      sed -i '/^[[:space:]]*$/d' workflows/build-"$filename"-"$arch".yaml
    done
done
