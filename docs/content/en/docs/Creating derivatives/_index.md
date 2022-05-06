
---
title: "Creating derivatives"
linkTitle: "Creating derivatives"
weight: 4
date: 2017-01-05
description: >
  Documents various methods for creating cOS derivatives
---

A derivative is a standard container image which can be booted by cOS. 

We can identify a build phase where we build the derivative, and a "runtime phase" where we consume it.

![](https://docs.google.com/drawings/d/e/2PACX-1vTTOJ0G4aMpdfSHv13sgPIIFCTK3SDIlcqmDxfPbGz0AlpNPTz1FTUigr-9co33c6MwXhDcead5nWFw/pub?w=1270&h=717
)

The image is described by a Dockerfile, composed of a base OS of choice (e.g. openSUSE, Ubuntu, etc. ) and the cOS toolkit itself in order to be consumed by cOS and allow to be upgraded from by other derivatives. 

cOS-toolkit then converts the OCI artifact into a bootable medium (ISO, packer, ova, etc) and the image itself then can be used to bootstrap other derivatives, which can in turn upgrade to any derivative built with cOS.

A derivative can also be later re-used again as input as base-image for downstream derivatives.

All the documentation below imply that the container image generated will be the booting one, there are however several configuration entrypoint that you should keep in mind while building the image which are general across all the implementation:

- Custom persistent runtime configuration has to be provided in `/system/oem` for derivatives, [see also the documentation section](../customizing/configuration_persistency).  Everything under `/system/oem` will be loaded during the various stages (boot, network, initramfs). You can check [here](https://github.com/rancher-sandbox/cOS-toolkit/tree/e411d8b3f0044edffc6fafa39f3097b471ef46bc/packages/cloud-config/oem) for the `cOS` defaults. See `00_rootfs.yaml` to customize the booting layout.
- `/etc/cos/bootargs.cfg` contains the booting options required to boot the image with GRUB, [see grub customization](../customizing/configure_grub)
- `/etc/cos-upgrade-image` contains the default upgrade configuration for recovery and the booting system image, [see customizing upgrades](../customizing/upgrades)

Derivatives inherits `cOS` defaults, which you can override during the build process, however there are some defaults which are relevant and listed below:

