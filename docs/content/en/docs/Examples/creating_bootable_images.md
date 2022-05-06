---
title: "Creating bootable images"
linkTitle: "Creating bootable images"
weight: 3
date: 2017-01-05
description: >
  This document describes the requirements to create standard container images that can be used for `cOS` deployments
---


You can find the examples below in the [examples](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/examples) folder.

## From standard images

Besides using the `cos-toolkit` toolchain, it's possible to create standard container images which are consumable by the vanilla `cOS` images (ISO, Cloud Images, etc.) during the upgrade and deploy phase.

An example of a Dockerfile image can be:


{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/standard/Dockerfile" lang="Dockerfile">}}

While the config file:

{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/standard/conf/luet.yaml" lang="yaml">}}

We can just run docker to build the image with 

```bash
docker build -t $IMAGE .
```

The important piece is that an image needs to ship at least:

```
toolchain/yip
utils/installer
system/cos-setup
system/immutable-rootfs
system/grub2-config
```

from the toolchain. If you want to customize further the container further add more step afterwards `luet install` see [the customizing section](../../customizing).

{{% alert title="Note" %}}
Depending on the base image (`FROM opensuse/leap:15.3` in the sample), you must set the corresponding repository for each flavor [see releases](../../getting-started/download#releases) in the luet config file ( which in the sample above points to the _green_ releases )
{{% /alert %}}

## Generating from CI image

Derivatives can be stacked on top of another, so it is possible to reuse directly also the vanilla cOS images:

{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/cos-official/Dockerfile" lang="Dockerfile">}}

The images contains already the toolkit, so they can be used as-is and apply further customization on top.

## From scratch

The luet image `quay.io/luet/base` contains just luet, and can be used to boostrap the base system from scratch:

conf/luet.yaml:
{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/scratch/conf/luet.yaml" lang="yaml">}}

Dockerfile:
{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/scratch/Dockerfile" lang="Dockerfile">}}

## Customizations

All the method above imply that the image generated will be the booting one, there are however several configuration entrypoint that you should keep in mind while building the image:

- Everything under `/system/oem` will be loaded during the various stage (boot, network, initramfs). You can check [here](https://github.com/rancher-sandbox/cOS-toolkit/tree/e411d8b3f0044edffc6fafa39f3097b471ef46bc/packages/cloud-config/oem) for the `cOS` defaults. See `00_rootfs.yaml` to customize the booting layout.
- `/etc/cos/bootargs.cfg` contains the booting options required to boot the image with GRUB
- `/etc/cos-upgrade-image` contains the default upgrade configuration for recovery and the booting system image

## Configuration file

The example configuration file shows how to enable the cos-toolkit repository:

{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/standard/conf/luet.yaml" lang="yaml">}}

Repositories have the following fields, notably:

- `name`: Repository name
- `enable`: Enable/disables the repository
- `arch`:  (optional) Denotes the arch repository. If present, it will enable the repository automatically if the corresponding arch is matching with the host running `luet`. `enable: true` would override this behavior
- `reference`: (optional) A reference to a repository index file to use to retrieve the repository metadata instead of latest. This can be used to point to a different or an older repository index to act as a "wayback machine". The client will consume the repository state from that snapshot instead of latest.
  
{{% alert title="Note" %}}
The `reference` field has to be a valid tag. For example, for the `green` flavor, browse the relevant [container image list page](https://quay.io/repository/costoolkit/releases-green?tab=tags). The repository index snapshots are prefixed with a timestamp, and ending in `repository.yaml`. For example ` 20211027153653-repository.yaml`
{{% /alert %}}
