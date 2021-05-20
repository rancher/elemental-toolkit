#!/bin/bash
# This connects stdout+stderr to the log file but leaves fd3 connected to the console
# To log to both use | tee /dev/fd/3 which sends the data to fd3 AND to tee fd1 which is connected to the log file
# You can also use 1>&3 to send it to console only
exec 3>&1 1>>/tmp/image-mtree-check.log 2>&1

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
  luet_result=$(luet mtree -- check /tmp/upgrade "$mtree_output" -f json)
  rm "$mtree_output"
  rm  -Rf "$tmpdir"
  echo "$luet_result" | tee /dev/fd/3
else
  echo "{}" | tee /dev/fd/3
fi
