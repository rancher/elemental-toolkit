---
title: "Embedded configuration"
linkTitle: "Embedded configuration"
weight: 3
date: 2023-08-31
description: >
  Extracting default system configuration
---

Elemental-toolkit provides some default configuration files for the following components:

- GRUB2
- Dracut
- Cloud init files
- Boot assessment

These configuration files can be installed into a Derivative using the `elemental init`-command

The `init`-command should be used inside the Dockerfile as in the following example:

{{<githubembed repo="rancher/elemental-toolkit" file="examples/green/Dockerfile" lang="Dockerfile">}}

The current features available for the `init`-command is:

- immutable-rootfs: dracut configuration for mounting the immutable root filesystem.
- grub-config: grub configuration for booting the derivative.
- elemental-setup: services used for booting the system and running cloud-init files at boot/install/upgrade.
- dracut-config: default dracut configuration for generating an initrd.
- cloud-config-defaults: optional default settings for a derivative.
- cloud-config-essentials: essential cloud-init files.


