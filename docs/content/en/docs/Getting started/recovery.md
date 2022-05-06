
---
title: "Recovery"
linkTitle: "Recovery"
weight: 5
date: 2017-01-04
description: >
  How to use the recovery partition to reset the system or perform upgrades.
---

cOS derivatives have a recovery mechanism built-in which can be leveraged to restore the system to a known point. At installation time, the recovery partition is created from the installation medium.

The recovery system can be accessed during boot by selecting the last entry in the menu (labeled by "recovery").

A derivative can be recovered anytime by booting into the ` recovery` partition and by running `elemental reset` from it. 

This command will regenerate the bootloader and the images in the `COS_STATE` partition by using the recovery image.

### Upgrading the recovery partition

From either the active or passive system, the recovery partition can also be upgraded by running 

```bash
elemental upgrade --recovery
``` 

It also supports to specify docker images directly:

```bash
elemental upgrade --recovery --docker-image <image>
```

### Upgrading the active system from the recovery

The recovery system can upgrade also the active system by running `elemental upgrade`, and it also supports to specify docker images directly:

```bash
elemental upgrade --recovery --docker-image <image>
```

