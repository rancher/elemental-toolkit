---
title: "Build ISOs"
linkTitle: "Build ISOs"
weight: 4
date: 2017-01-05
description: >
  Build ISOs from bootable images
---

![](https://docs.google.com/drawings/d/e/2PACX-1vReZtyNs0imrji-AwnqK0-4ekCKLcKzfnQ_CwiMj93Q7IsycAJHwlNohwCv_hyHnaify7qO-v2Cecg5/pub?w=1223&h=691)

In order to build an iso we rely on [elemental build-iso](https://github.com/rancher-sandbox/elemental) command. It accepts a YAML file denoting the packages to bundle in an ISO and a list of luet repositories where to download the packages from. In addition it can also overlay custom files or use container images from a registry as packages.

To build an iso, just run:

```bash
docker run --rm -ti -v $(pwd):/build quay.io/costoolkit/elemental:v0.0.14-e4e39d4 --debug build-iso -o /build $IMAGE
```

Where `$IMAGE` is the container image you want to build the ISO for, you might want to check on [how to build bootable images](../creating_bootable_images).

`elemental build-iso` command also supports reading a configuration `manifest.yaml` file. It is loaded form the directory specified by `--config-dir` elemental's flag.

An example of a yaml file using the cos-toolkit opensuse repositories:

```yaml
iso:
  rootfs:
  - system/cos
  uefi:
  - live/grub2-efi-image
  image:
  - live/grub2-efi-image
  - live/grub2
  label: "COS_LIVE"

name: "cOS-0"
date: true
```

## What's next?

- Check out on how to [build a QCOW, Virtualbox or Vagrant image](../packer/build_images) from the ISO we have just created

## Syntax

Below you can find a full reference about the yaml file format.

```yaml
iso:
  # Packages to be installed in the rootfs
  rootfs:
  - ..
  # Packages to be installed in the uefi image
  uefi:
  - live/grub2-efi-image
  # Packages to be installed in the iso image
  image:
  - live/grub2-efi-image
  - live/grub2
  label: "COS_LIVE"
  
repositories:
  - uri: quay.io/costoolkit/releases-green

```

Packages or sources can be a Luet package (as in the example), an image reference (then an explicit tag is required) or a local path. Sources are stacked in the given order, so one can easily overwrite or append data by simply adding a local path as the last source.

### Command flags

- **name**: Name of the ISO image. It will be used to generate the `*.iso` file name
- **output**: Path of the destination folder of created images
- **date**: If present it includes the date in the generated file name
- **overlay-rootfs**: Sets the path of a tree to overlay on top of the system root-tree
- **overlay-uefi**: Sets the path of a tree to overaly on top of the EFI image root-tree
- **overlay-iso**: Sets the path of a tree to overlay on top of the ISO filesystem root-tree
- **label**: Sets the volume label of the ISO filesystem
- **repo**: Sets the URI of a repository to include together with the repositores set in manifest or the default one if no repositories are set in manifest. This option can be set multiple times.

## Configuration reference

### `iso.rootfs`

A list of sources (luet package, container image or local path) to install in the rootfs. The rootfs will be squashed to a `rootfs.squashfs` file

### `iso.uefi`

A list of sources (luet package, container image or local path) to install in the efi FAT image or partition.

### `iso.image`

A list of sources (luet package, container image or local path) to install in ISO filesystem.

### `iso.label`

The label of the ISO filesystem. Defaults to `COS_LIVE`. Note this value is tied with the bootloader and kernel parameters to identify the root device.

### `repositories`

A list of Luet package repositories

### `repositories.uri`

The URI of the repository, it is the only mandatory value for a repository. Repository type (`docker`, `disk` or `http`) is guessed from this URI if not provided.

### `repositories.type`

The repository type, it can be `docker` (the URI points to a registry), `http` (an HTTP(S) URI) or `disk` (the URI is then a local path).

### `repositories.name`

The repository name, if not provided a md5 sum of the URI is used instead.

### `repositories.priority`

The priority of the given repository, if unsed uses `0`, which is the highest priority.

### `name`

A string representing the ISO final image name without including the `.iso`

### `date`

Boolean indicating if the output image name has to contain the date

### `output`

Folder destination of the built artifacts. It attempts to create if it doesn't exist.

## Customize bootloader with GRUB

Boot menu and other bootloader parameters can then be easily customized by using the overlay parameters within the ISO config yaml manifest.

Assuming the ISO being built includes:

```yaml
iso:
  rootfs:
  - ...
  uefi:
  - live/grub2-efi-image
  image:
  - live/grub2
  - live/grub2-efi-image
```

We can customize either the `image` packages (in the referrence image `live/grub2` package
includes bootloader configuration) or make use of the overlay concept to include or
overwrite addition files for `image` section.

Consider the following example:

```yaml
iso:
  rootfs:
  - ...
  uefi:
  - live/grub2-efi-image
  image:
  - live/grub2
  - live/grub2-efi-image
  - /my/path/to/overlay/iso
```

With the above the ISO will also include the files under `/my/path/to/overlay/iso` path. To customize the boot
menu parameters consider copy and modify relevant files from `live/grub2` package. In this example the
`overlay` folder files list could be:

```bash
# image files for grub2 boot
boot/grub2/grub.cfg
```

Being `boot/grub2/grub.cfg` a custom grub2 configuration including custom boot menu entries. Consider the following `grub.cfg` example:

```
search --file --set=root /boot/kernel.xz
set default=0
set timeout=10
set timeout_style=menu
set linux=linux
set initrd=initrd
if [ "${grub_cpu}" = "x86_64" -o "${grub_cpu}" = "i386" ];then
    if [ "${grub_platform}" = "efi" ]; then
        set linux=linuxefi
        set initrd=initrdefi
    fi
fi

set font=($root)/boot/x86_64/loader/grub2/fonts/unicode.pf2
if [ -f ${font} ];then
    loadfont ${font}
fi

menuentry "Custom grub2 menu entry" --class os --unrestricted {
    echo Loading kernel...
    $linux ($root)/boot/kernel.xz cdroot root=live:CDLABEL=COS_LIVE rd.live.dir=/ rd.live.squashimg=rootfs.squashfs console=tty1 console=ttyS0 rd.cos.disable
    echo Loading initrd...
    $initrd ($root)/boot/rootfs.xz
}
```

## Separate recovery

To make an ISO with a separate recovery image as squashfs, you can either use the default from `cOS`, by adding it in the iso yaml file:

```yaml
iso:
  rootfs:
  ..
  uefi:
  ..
  image:
  ...
  - recovery/cos-img
```

The installer will detect the squashfs file in the iso, and will use it when installing the system. You can customize the recovery image as well by providing your own: see the `recovery/cos-img` package definition as a reference.

