#!/bin/bash
exec >> /tmp/image-mtree-check.log
exec 2>&1
event="$1"

if [ "$event" == "package.install" ]; then
  payload="$2"
  file=$(echo "$payload" | jq -r .file)

  version=$(jq -r .Package.version < "$file")
  repo=$(jq -r .Repository.urls[0] < "$file")
  name=$(jq -r .Package.name < "$file")
  category=$(jq -r .Package.category < "$file")
  download_version=$(echo "$version"|tr "+" "-")

  image="$repo":"$name"-"$category"-"$download_version".metadata.yaml
  tmpdir=/tmp/"$name"-"$category"-"$download_version"-metadata
  mtree_output=/tmp/"$name"-"$category"-"$download_version".mtree

  echo "Getting $image metadata"

  luet util unpack "$image" "$tmpdir"
  yq read "$tmpdir"/"$name"-"$category"-"$version".metadata.yaml mtree > "$mtree_output"
  luet mtree -- check /tmp/upgrade "$mtree_output"
  rm "$mtree_output"
  rm  -Rf "$tmpdir"
  exit $?
fi
