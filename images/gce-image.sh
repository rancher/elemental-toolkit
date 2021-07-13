#!/bin/bash

# Transform a raw image disk to gce compatible
RAWIMAGE="$1"
RAW_DISK="disk.raw"
COS_VERSION=$(yq r packages/cos/collection.yaml 'packages.[0].version')


GB=$((1024*1024*1024))
size=$(qemu-img info -f raw --output json "$RAWIMAGE" | gawk 'match($0, /"virtual-size": ([0-9]+),/, val) {print val[1]}')
# shellcheck disable=SC2004
ROUNDED_SIZE=$(echo "$size/$GB+1"|bc)
if [[ "$RAWIMAGE" != "$RAW_DISK" ]]; then
  echo "Renaming raw image to $RAW_DISK"
  mv "$RAWIMAGE" "$RAW_DISK"
fi
echo "Resizing raw image from $size to $ROUNDED_SIZE"
qemu-img resize -f raw "$RAW_DISK" "$ROUNDED_SIZE"G
echo "Compressing raw image"
tar -c -z --format=oldgnu -f cOS-Vanilla-"$COS_VERSION".tar.gz $RAW_DISK
echo "Done"