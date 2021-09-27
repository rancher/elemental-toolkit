#!/bin/bash
set -e
YQ="${YQ:-yq}"

for i in $(ls config); do
    filename=$(basename -s .yaml $i)
    for flavor in $(${YQ} r config/"$i" -p "p" 'flavors.[*]' | cut -d "." -f 2); do
      for arch in $(${YQ} r config/"$i" -p "p" "flavors.$flavor.arches.[*]" | cut -d "." -f 4); do
        ${YQ} r -X config/"$i" "flavors.$flavor.arches.$arch" | gomplate --left-delim "{{{" --right-delim "}}}" -V --datasource config="stdin:/?type=application/yaml" --file build.yaml.gomplate --out workflows/build-"$filename"-"$flavor"-"$arch".yaml
        sed -i '/^[[:space:]]*$/d' workflows/build-"$filename"-"$flavor"-"$arch".yaml
      done
    done
done
