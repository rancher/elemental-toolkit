---
title: "Runtime layout"
linkTitle: "Runtime layout"
weight: 4
date: 2021-10-11
description: >
  Runtime layout of a booted cOS derivative
---

This section describes the runtime layout of a derivative (or a cOS Vanilla image) once booted in a system.  

The cOS toolkit performs during installation a common setup which is equivalent across all derivatives. 

This mechanism ensures that a layout:

- it's simple and human friendly
- allows to switch easily derivatives
- allows to perform recovery tasks
- is resilient to upgrade failures

## Layout

The basic setup consists of:

- an `A/B` partitioning style. We have an 'active' and a 'passive' system too boot from in case of failures
- a Recovery system which allows to perform emergency tasks in case of failure of the 'A/B' partitions
- a Fallback mechanism that boots the partitions in this sequence: "A -> B -> Recovery" in case of booting failures

The upgrade happens in a transition image and take places only after all the necessary steps are completed. An upgrade of the 'A/B' partitions can be done by booting into them and running `elemental upgrade`. This will create a new pristine image that will be selected as active for the next reboot, the old one will be flagged as passive. If we are performing the same from the passive system, only the active is subject to changes.

Similarly, a recovery system can be upgraded as well by running `elemental upgrade --recovery`. This will upgrade the recovery system instead of the active/passive. Note both commands needs to be run inside the active or passive system.

## Partitions

![](https://docs.google.com/drawings/d/e/2PACX-1vSP-Pz9l9hwYDeIlej7qXzzcMzGYBiKjyFpiYYKlbNR3H37n_R_c0eBNeYa3msouOupmDim3ZYYBSxS/pub?w=812&h=646)

The default partitioning is created during installation and is expected to be present in a booted cOS system:

- a `COS_STATE` partition that will contain our active, passive and recovery images. The images are located under the `/cOS` directory
- a `COS_PERSISTENT` partition which contains the persistent user data. This directory is mounted over `/usr/local` during runtime
- a `COS_OEM` partition which contains the cloud-init oem files, which is mounted over `/oem` during runtime
- a `COS_RECOVERY` partition which contains the recovery system image

The `COS_STATE` partitions contains the `active`, `passive` . While the `active` and `passive` are `.img` files which are loopback mounted, the `recovery` system is in `COS_RECOVERY` and can also be a `squashfs` file (provided in `/cOS/recovery.squashfs`). This ensures the immutability aspect and ease out building derivative in constrained environments (e.g. when we have restricted permissions and we can't mount).

For more information about the immutability aspect of cOS, see [Immutable rootfs](../immutable_rootfs)
