---
title: "Creating bootable images"
linkTitle: "Creating bootable images"
weight: 3
date: 2023-05-31
description: >
  This document describes the requirements to create standard container images that can be used for `Elemental` deployments
---


You can find the examples below in the [examples](https://github.com/rancher/elemental-toolkit/tree/main/examples) folder.

## From standard images

Besides using the `elemental-toolkit` toolchain, it's possible to create standard container images which are consumable by the vanilla `Elemental` images (ISO, Cloud Images, etc.) during the upgrade and deploy phase.

An example of a Dockerfile image can be:


{{<githubembed repo="rancher/elemental-toolkit" file="examples/green/Dockerfile" lang="Dockerfile">}}

We can just run docker to build the image with 

```bash
docker build -t $IMAGE .
```

The important piece is that an image needs to ship at least:

```
grub2
systemd
kernel
dracut
```

## Customizations

All the method above imply that the image generated will be the booting one, there are however several configuration entrypoint that you should keep in mind while building the image:

- Everything under `/system/oem` will be loaded during the various stage (boot, network, initramfs). You can check [here](https://github.com/rancher/elemental-toolkit/tree/e411d8b3f0044edffc6fafa39f3097b471ef46bc/packages/cloud-config/oem) for the `Elemental` defaults. See `00_rootfs.yaml` to customize the booting layout.
- `/etc/cos/bootargs.cfg` contains the booting options required to boot the image with GRUB
