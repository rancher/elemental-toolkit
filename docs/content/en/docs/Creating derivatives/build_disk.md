---
title: "Build disk images with Elemental"
linkTitle: "Build disk images with Elemental"
weight: 4
date: 2023-08-31
description: >
  This section documents the procedure to build disk images using elemental
---

Requirements:

* `qemu-img` utility
* `elemental` binary
* elemental runtime dependencies

The suggested approach is based on using the Elemental installer (`elemental install` command) to run the installation
from a Linux to a loop device. The loop device can be a raw image created with `qemu-img create` that can easily be
converted to other formats after the installation by using `qemu-img convert`.

## Prepare the loop device

Preparing the a loop device for the installation is simple and straight forward.

```bash
# Create a raw image of 32G
> qemu-img create -f raw disk.img 32G

# Set the disk image as a loop device
> sudo losetup -f --show disk.img
<device>
```

## Run elemental installation

Execute the elemental installation as described in [installing](../../getting-started/install):

```bash
> sudo elemental install --firmware efi --system.uri oci:<image=ref> <device>
```

Where `<image-ref>` is the Elemental derivative container image we want to use for the disk creation and `<device>` is the
loop device previously created with `losetup` (e.g. `/dev/loop0`).


## Convert the RAW image to desired format

Once the installation is done just unsetting the loop device and converting the image to the desired format is missing:

```bash
# Unset the loop device
> sudo losetup -d <device>

# Convert the RAW image to qcow2
> qemu-img convert -f raw -O qcow2 disk.img disk.qcow2
```

QEMU supports a wide range of formats including common ones such as `vdi`, `vmdk` or `vhdx`.

The result can be easily tested on QEMU with:

```bash
> qemu -m 4096 -hda disk.qcow2 -bios /usr/share/qemu/ovmf-x86_64.bin
```

Note the firmware image path varies depending on the host distro, the path provided in this example is based on openSUSE Leap.
