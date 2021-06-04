#!/bin/bash
# This connects stdout+stderr to the log file but leaves fd3 connected to the console
# To log to both use | tee /dev/fd/3 which sends the data to fd3 AND to tee fd1 which is connected to the log file
# You can also use 1>&3 to send it to console only
exec 3>&1 1>>/tmp/image-mtree-check.log 2>&1

event="$1"

if [ "$event" == "package.install" ] || [ "$event" == "image.post.unpack" ]; then
  payload="$2"

  if [ "$event" == "package.install" ]; then
    # Extract all data from the payload
    file=$(echo "$payload" | jq -r .file)
    version=$(jq -r .Package.version < "$file")
    repo=$(jq -r .Repository.urls[0] < "$file")
    name=$(jq -r .Package.name < "$file")
    category=$(jq -r .Package.category < "$file")
    download_version=$(echo "$version"|tr "+" "-")
    image="$repo":"$name"-"$category"-"$download_version".metadata.yaml
    mtree_output=/tmp/"$name"-"$category"-"$download_version".mtree
    tmpdir=/tmp/"$name"-"$category"-"$download_version"-metadata
    destination=/tmp/upgrade
  else
    # Use the image name as base for everything
    fullimage=$(echo "$2" | jq -r .data | jq -r .Image )
    imagetype=$(echo "$fullimage" |cut -d ":" -f 2)
    # We should skip when unpacking the repository and tree images as they do not contain mtree values
    if [[ $imagetype == *"repository.yaml"* ]] || [[ $imagetype == *"tree.tar"* ]] || [[ $imagetype == *"repository.meta.yaml.tar"* ]] || [[ $imagetype == *"compilertree.tar"* ]]; then
      echo "{}" | tee /dev/fd/3
      exit 0
    fi
    destination=$(echo "$2" | jq -r .data | jq -r .Dest)
    image="$fullimage.metadata.yaml"
    tmpdir=/tmp/"$fullimage"-metadata
    mtree_output=/tmp/"$fullimage".mtree
  fi

  echo "Getting $image metadata"

  luet util unpack "$image" "$tmpdir"

  metadata=$(find "$tmpdir" -name "*.metadata.yaml")

  yq read "$metadata" mtree > "$mtree_output"
  luet_result=$(luet mtree -- check "$destination" "$mtree_output" -x "var/cache/luet" -x "usr/local/tmp" -x "oem/" -x "usr/local/cloud-config" -x "usr/local/lost+found" -x "lost+found" -x "tmp" -x "mnt")
  rm "$mtree_output"
  rm  -Rf "$tmpdir"
  if [[ $luet_result == "" ]]; then
    # empty output means no errors
    jq --arg key0 "state" --arg value0 "All checks succeeded" \
       --arg key1 "data"  --arg value1 "" \
       --arg key2 "error" --arg value2 "" \
       '. | .[$key0]=$value0 | .[$key1]=$value1 | .[$key2]=$value2' \
    <<<'{}' | tee /dev/fd/3
  else
    echo "$luet_result" > /tmp/luet_mtree_failures.log
    error_message="Error while checking, see /tmp/luet_mtree_failures.log for the full failures log"
    jq --arg key0 "state" --arg value0 "Checks failed" \
       --arg key1 "data"  --arg value1 "" \
       --arg key2 "error" --arg value2 "$error_message" \
       '. | .[$key0]=$value0 | .[$key1]=$value1 | .[$key2]=$value2' \
    <<<'{}' | tee /dev/fd/3
  fi
else
  echo "{}" | tee /dev/fd/3
fi
