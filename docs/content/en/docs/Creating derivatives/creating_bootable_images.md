---
title: "Creating bootable images"
linkTitle: "Creating bootable images"
weight: 2
date: 2023-05-11
description: >
  This document describes the requirements to create standard container images that can be used for `Elemental` deployments
---

![](https://docs.google.com/drawings/d/e/2PACX-1vSmIZ5FTInGjtkGonUOgwhti6DZnSoeexGmWL9CAmbdiIGtBGnzDuGNj80Lj_206hP0MOxQGpEdYFvK/pub?w=1223&h=691)

A derivative is a simple container image which can be processed by the Elemental toolkit in order to be bootable and installable. This section describes the requirements to create a container image that can be run by `Elemental`.

## Requirements
{{<image_right image="https://docs.google.com/drawings/d/e/2PACX-1vQBfT10W88mD1bbReDmAJIOPF3tWdVHP7QE9w7W7ByOIzoKGOdh2z5YWsKf7wn8csFF_QGrDXgGsPWg/pub?w=478&h=178">}}

Bootable images are standard container images, that means the usual `build` and `push` workflow applies, and building images is also a way to persist [oem customizations](../../customizing). 

The base image can be any Linux distribution that is compatible with our flavors.

The image needs to ship:
- parts of the elemental-toolkit (required, see below)
- kernel (required)
- initrd (required)
- grub2 (required)
- dracut (required)
- microcode (optional, not required in order to boot, but recomended)
- [cosign](../cosign) packages (optional, required if you want to verify the images)

## Example

An illustrative example can be:


{{<githubembed repo="rancher/elemental-toolkit" file="examples/green/Dockerfile" lang="Dockerfile">}}

In the example above, the elemental-toolkit parts that are **required** are pulled in by `COPY --from=TOOLKIT /install-root /`.

## Initrd
The image should provide at least `grub`, `systemd`, `dracut`, a kernel and an initrd. Those are the common set of packages between derivatives. See also [package stack](../package_stack). 
By default the initrd is expected to be symlinked to `/boot/initrd` and the kernel to `/boot/vmlinuz`, otherwise you can specify a custom path while [building an iso](../build_iso) and [by customizing grub](../../customizing/configure_grub).

## Building

![](https://docs.google.com/drawings/d/e/2PACX-1vS6eRyjnjdQI7OBO0laYD6vJ2rftosmh5eAog6vk_BVj8QYGGvnZoB0K8C6Qdu7SDz7p2VTxejcZsF6/pub?w=956&h=339)

The workflow would be then:

1) `docker build` the image
2) `docker push` the image to some registry
3) `elemental upgrade --docker-image $IMAGE` from a Elemental machine or (`elemental reset` if bootstrapping a cloud image)

The following can be incorporated in any standard gitops workflow.

You can explore more examples in the [example section](../../examples/creating_bootable_images) on how to create bootable images.

## What's next?

Now that we have created our derivative container, we can either:

- [Build an iso](../build_iso)
