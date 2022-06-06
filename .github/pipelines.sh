#!/bin/bash
set -e
YQ="${YQ:-yq}"

VERSION=$($YQ -V |rev|  cut -d " " -f 1 |rev| cut -d "." -f 1)
if [[ "$VERSION" == "3" ]]; then
    echo "yq version 3 detected, only version 4 is supported"
    exit 1
fi

for i in $(ls config); do
    filename=$(basename -s .yaml $i)
    for flavor in $(${YQ} '.flavors|keys|join(" ")' config/"$i"); do
      for arch in $(${YQ} ".flavors.$flavor.arches|keys|join(\" \")" config/"$i" ); do
        # Use explode(.) so anchors are fully resolved before parsing
        ${YQ} "explode(.)|.flavors.$flavor.arches.$arch" config/"$i" | gomplate --left-delim "{{{" --right-delim "}}}" -V --datasource config="stdin:/?type=application/yaml" --file build.yaml.gomplate --out workflows/build-"$filename"-"$flavor"-"$arch".yaml
        sed -i '/^[[:space:]]*$/d' workflows/build-"$filename"-"$flavor"-"$arch".yaml
      done
    done
done
