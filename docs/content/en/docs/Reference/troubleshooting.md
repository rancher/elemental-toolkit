---
title: "Troubleshooting"
linkTitle: "Troubleshooting"
weight: 5
date: 2021-09-08
description: >
  Stuff can go wrong. This document tries to make them right with some useful tips
---

{{% pageinfo color="warning"%}}
Section under construction.
{{% /pageinfo %}}

While building a derivative, or on a running system things can go really wrong, the guide is aimed to give tips while building derivatives and also debugging running systems.

Don't forget tocheck the known issues for the [release you're using](https://github.com/rancher-sandbox/cOS-toolkit/issues).

Before booting, [several kernel parameters](../immutable_rootfs) can be used to help during debugging (also when booting an ISO). Those are meant to be used only while debugging, and they might defeat the concept of immutability.

## Disable Immutability

By adding `rd.cos.debugrw` to the boot parameters read only mode will be disabled. See [Immutable setup](../immutable_rootfs) for more options.

The derivative will boot into RW mode, that means any change made during runtime will persist across reboots. Use this feature with caution as defeats the concept of immutability.

`rd.cos.debugrw` applies only to active and passive partitions. The recovery image can't be mutated.

{{% alert title="Note" %}}
The changes made will persist during reboots but won't persist across upgrades. If you need to persist changes across upgrades in runtime (for example by adding additional packages on top of the derivative image), see [how to apply persistent changes](../../customizing/runtime_persistent_changes). 
{{% /alert %}}

## Debug initramfs issues

As derivative can ship and build their own initrd, the [official debug docs](https://fedoraproject.org/wiki/How_to_debug_Dracut_problems) contains valid information that can be used for troubleshooting.  

For example:

- `rd.break=pre-mount rd.shell`: Drop a shell before setting up mount points
- `rd.break=pre-pivot rd.shell`: Drop a shell before switch-root

## Recovery partition

If you can boot into the system, the recovery partition can be used to reset the state of the active/passive, but can also be used to upgrade to specific images. Be sure to read the [Recovery section in the docs](../../getting-started/recovery).

## Mutating derivative images

It can be useful to mutate derivative images and commit a container’s file changes or settings into a new image. 
This allows you to debug a container by running an interactive shell, and re-use the mutated image in cOS systems. Generally, it is better to use Dockerfiles to manage your images in a documented and maintainable way. [Read more about creating bootable images](../../creating-derivatives/creating_bootable_images).

Let's suppose we have the derivative original image at `$IMAGE` and we want to mutate it. We will push it later with another name `$NEW_IMAGE` and use it to our node downstream.

Run the derivative image locally, and perform any necessary change (e.g. add additional software):
```bash
$> docker run --entrypoint /bin/bash -ti --name updated-image $IMAGE
```

Commit any changes to a new image `$NEW_IMAGE`:
```bash
$> docker commit updated-image $NEW_IMAGE
```

And push the image to the container registry:
```bash
$> docker push $NEW_IMAGE
```

In the derivative then it's sufficient to upgrade to that image with `elemental upgrade`:

```bash
$> elemental upgrade --no-verify --docker-image $NEW_IMAGE
```

## Adding login keys at boot

To add users key from the GRUB menu prompt, edit the boot cmdline and add the following kernel parameters: 

`stages.boot[0].authorized_keys.root[0]=github:mudler`
