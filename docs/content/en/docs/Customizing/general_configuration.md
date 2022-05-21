---
title: "General Configuration"
linkTitle: "General Configuration"
weight: 3
date: 2017-01-05
description: >
  Configuring a cOS derivative
---


cOS during installation, reset and upgrade (`elemental install`, `elemental reset` and `elemental upgrade` respectively) will read a configuration file in order to apply derivative customizations. The configuration files are sourced in precedence order and can be located in the following places:

- `/etc/os-release`
- `<config-dir>/config.yaml`
- `<config-dir>/config.d/*.yaml`

By default `<config-dir>` is set to `/etc/elemental` however this can be changed to any custom path by using the `--config-dir` runtime flag.

Below you can find an example of the config file including most of the available options:

{{<githubembed repo="rancher-sandbox/elemental" file="config.yaml.example" lang="yaml">}}


The `system` and `recovery-system` objects are an image specification. An image specification is defined by:

- **fs**: defines the filesystem of the image. Currently only `ext2` and `squashfs` should be used for images and `squashfs` is only supported for the `recovery-system` image.
- **label**: defines the filesystem label. It is strongly recommended to use default labels as it is easy to fall into inconsistent states when changing labels as all changes should also be reflected in several other parts such as the bootloader configuration. This attribute has no effect for `squashfs` filesystems.
- **uri**: defines the source of the image. The uri must include a valid scheme to identify the type of source. It supports `docker`, `channel`, `dir` and `file` schemes.
- **size**: defines the filesystem image size in MiB, it must be big enough to store the defined image source. This attribute has no effect for `squashfs` filesystems.


The `partitions` object lists partition specifications. A partition specifications is defined by:

- **fs**: defines the filesystem of the partition. Currently only `ext2`, `ext4` and `xfs` are supported being `ext4` the default.
- **label**: defines the label of the filesystem of the partition. It is strongly recommended to use default labels as it is easy to fall into inconsistent states when changing labels as all changes should also be reflected in several other parts such as the bootloader configuration.
- **size**: defines the partition size in MiB. A zero size means use all available disk, obviously this only makes sense for the last partition, the `persistent` partition.
- **flags**: is a list of strings, this is used as additional partition flags that are passed to `parted` (e.g. `boot` flag). Defaults should be just fine for most of the cases.


The `repositories` object lists repositories metadata. Each repository can be defined by:

- **name**: name of the repository.
- **priority**: of the repository, being 1 the highest priority.
- **uri**: the URI of the repository.
- **type**: type of the repository, currently `docker`, `disk` or `http`.
- **arch**: the architecture this repository is for.
- **reference**: the Luet repository reference of this repository.

