---
title: "Build ISOs"
linkTitle: "Build ISOs"
weight: 4
date: 2023-08-31
description: >
  Build ISOs from bootable images
---

In order to build an ISO we rely on `elemental build-iso` command. It accepts a YAML file denoting the sources to bundle in an ISO. In addition it can also overlay custom files or use container images from a registry as packages.

To build an ISO, just run:

```bash
docker run --rm -ti -v $(pwd):/build ghcr.io/rancher/elemental-toolkit/elemental-cli:latest --debug build-iso --bootloader-in-rootfs -o /build $SOURCE
```

Where `$SOURCE` might be the container image you want to build the ISO for, you might want to check on [how to build bootable images](../creating_bootable_images). Argument `$SOURCE` might be the reference to the directory, file, container image or channel we are building the ISO for, it should be provided as uri in following format <sourceType>:<sourceName>, where:
    * <sourceType> - might be ["dir", "file", "oci", "docker"], as default is taken "docker"
    * <sourceName> - is path to file or directory, channel or image name with tag version (if tag was not provided then "latest" is used)

`elemental build-iso` command also supports reading a configuration `manifest.yaml` file. It is loaded form the directory specified by `--config-dir` elemental's flag.

An example of a yaml file using the bootloader from the contained image:

```yaml
iso:
  bootloader-in-rootfs: true
  grub-entry-name: "Installer"

name: "Elemental-0"
date: true
```

## What's next?

- Check out on how to [build an image](../build_disk) from the ISO we have just created

## Syntax

Below you can find a full reference about the yaml file format.

```yaml
iso:
  # Sources to be installed in the rootfs
  rootfs:
  - ..
  # Sources to be installed in the uefi image
  uefi:
  - ..
  # Sources to be installed in the iso image
  image:
  - ..
  label: "COS_LIVE"
```

Sources can be an image reference (then an explicit tag is required) or a local path. Sources are stacked in the given order, so one can easily overwrite or append data by simply adding a local path as the last source.

### Command flags

- **name**: Name of the ISO image. It will be used to generate the `*.iso` file name
- **output**: Path of the destination folder of created images
- **date**: If present it includes the date in the generated file name
- **overlay-rootfs**: Sets the path of a tree to overlay on top of the system root-tree
- **overlay-uefi**: Sets the path of a tree to overaly on top of the EFI image root-tree
- **overlay-iso**: Sets the path of a tree to overlay on top of the ISO filesystem root-tree
- **label**: Sets the volume label of the ISO filesystem

## Configuration reference

### `iso.rootfs`

A list of sources in uri format (container image or local path) [ "docker", "oci", "dir", "file" ] to install in the rootfs. The rootfs will be squashed to a `rootfs.squashfs` file

### `iso.uefi`

A list of sources in uri format (container image or local path) [ "docker", "oci", "dir", "file" ] to install in the efi FAT image or partition.

### `iso.image`

A list of sources in uri format (container image or local path) [ "docker", "oci", "dir", "file" ] to install in ISO filesystem.

### `iso.label`

The label of the ISO filesystem. Defaults to `COS_LIVE`. Note this value is tied with the bootloader and kernel parameters to identify the root device.

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
  - oci:example-grub2-efi-image:latest
  image:
  - oci:example-grub2:latest
  - oci:example-grub2-efi-image:latest
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
  - oci:example-grub2-efi-image:latest
  image:
  - oci:example-grub2:latest
  - oci:example-grub2-efi-image:latest
  - dir:/my/path/to/overlay/iso
```

With the above the ISO will also include the files under `/my/path/to/overlay/iso` path. To customize the boot
menu parameters consider copy and modify relevant files from `example-grub2:latest` image. In this example the
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

To make an ISO with a separate recovery image as squashfs, you can either use the default from `Elemental`, by adding it in the iso yaml file:

```yaml
iso:
  rootfs:
  ..
  uefi:
  ..
  image:
  ...
  - oci:example-recovery:latest
```

The installer will detect the squashfs file in the iso, and will use it when installing the system. You can customize the recovery image as well by providing your own.

