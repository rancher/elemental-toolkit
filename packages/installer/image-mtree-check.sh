#!/bin/bash
# This connects stdout+stderr to the log file but leaves fd3 connected to the console
# To log to both use | tee /dev/fd/3 which sends the data to fd3 AND to tee fd1 which is connected to the log file
# You can also use 1>&3 to send it to console only
exec 3>&1 1>>/tmp/image-mtree-check.log 2>&1

event="$1"

if [ "$event" == "image.post.unpack" ]; then
  echo "[$(date)] Got event $1, processing."
  # Use the image name as base for everything
  fullimage=$(echo "$2" | jq -r .data | jq -r .Image )
  imagetype=$(echo "$fullimage" |cut -d ":" -f 2)
  # We should skip when unpacking the repository and tree images as they do not contain mtree values
  if [[ $imagetype == *"repository.yaml"* ]] || [[ $imagetype == *"tree.tar"* ]] || [[ $imagetype == *"repository.meta.yaml.tar"* ]] || [[ $imagetype == *"compilertree.tar"* ]]; then
    echo "[$(date)] Got $imagetype, skipping"
    echo "{}" 1>&3
    exit 0
  fi
  echo "[$(date)] Got $imagetype, continue..."
  destination=$(echo "$2" | jq -r .data | jq -r .Dest)
  image="$fullimage.metadata.yaml"
  tmpdir=/tmp/"$fullimage"-metadata
  mtree_output=/tmp/"$fullimage".mtree

  echo "[$(date)] Getting $image metadata"

  # TMPDIR here is a workaround because we are calling luet from inside luet and there is aggressive cleaning after the
  # luet command finishes (https://github.com/mudler/luet/blob/master/cmd/root.go#L111) which will cleanup ALL luet tmp
  # dirs, so we could be removing our recently unpacked image during install by using this
  # This just moves the luet tmp dir to /tmp/
  TMPDIR=/tmp luet util unpack "$image" "$tmpdir" >> /tmp/luet.log

  metadata=$(find "$tmpdir" -name "*.metadata.yaml")

  yq read "$metadata" mtree > "$mtree_output"
  luet_result=$(luet mtree -- check "$destination" "$mtree_output" -x "var/cache/luet" -x "usr/local/tmp" -x "oem/" -x "usr/local/cloud-config" -x "usr/local/lost+found" -x "lost+found" -x "tmp" -x "mnt")
  rm "$mtree_output"
  rm  -Rf "$tmpdir"
  if [[ $luet_result == "" ]]; then
    # empty output means no errors
    echo "[$(date)] Finished all checks with no errors"
    jq --arg key0 "state" --arg value0 "All checks succeeded" \
       --arg key1 "data"  --arg value1 "" \
       --arg key2 "error" --arg value2 "" \
       '. | .[$key0]=$value0 | .[$key1]=$value1 | .[$key2]=$value2' \
    <<<'{}' 1>&3
  else
    echo "[$(date)] Finished all checks with errors"
    echo "$luet_result" > /tmp/luet_mtree_failures.log
    error_message="Error while checking, see /tmp/luet_mtree_failures.log for the full failures log"
    jq --arg key0 "state" --arg value0 "Checks failed" \
       --arg key1 "data"  --arg value1 "" \
       --arg key2 "error" --arg value2 "$error_message" \
       '. | .[$key0]=$value0 | .[$key1]=$value1 | .[$key2]=$value2' \
    <<<'{}' 1>&3
    exit 1
  fi
else
  echo "{}" 1>&3
  exit 0
fi
