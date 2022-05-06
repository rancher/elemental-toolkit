---
title: "Creating bootable images"
linkTitle: "Creating bootable images"
weight: 2
date: 2017-01-05
description: >
  This document describes the requirements to create standard container images that can be used for `cOS` deployments
---

![](https://docs.google.com/drawings/d/e/2PACX-1vSmIZ5FTInGjtkGonUOgwhti6DZnSoeexGmWL9CAmbdiIGtBGnzDuGNj80Lj_206hP0MOxQGpEdYFvK/pub?w=1223&h=691)

A derivative is a simple container image which can be processed by the cOS toolkit in order to be bootable and installable. This section describes the requirements to create a container image that can be run by `cOS`.

## Requirements
{{<image_right image="https://docs.google.com/drawings/d/e/2PACX-1vQBfT10W88mD1bbReDmAJIOPF3tWdVHP7QE9w7W7ByOIzoKGOdh2z5YWsKf7wn8csFF_QGrDXgGsPWg/pub?w=478&h=178">}}

Bootable images are standard container images, that means the usual `build` and `push` workflow applies, and building images is also a way to persist [oem customizations](../../customizing). 

The base image can be any Linux distribution that is compatible with our flavors.

The image needs to ship:
- parts of the cos-toolkit (required, see below)
- kernel (required)
- initrd (required)
- grub (required)
- dracut (optional, kernel and initrd can be consumed from the cOS repositories)
- microcode (optional, not required in order to boot, but recomended)
- [cosign and luet-cosign](../cosign) packages (optional, required if you want to verify the images installed by luet)

## Example

An illustrative example can be:


{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/standard/Dockerfile" lang="Dockerfile">}}

With the config file:

{{<githubembed repo="rancher-sandbox/cos-toolkit" file="examples/standard/conf/luet.yaml" lang="yaml">}}


In the example above, the cos-toolkit parts that are **required** are pulled in by `RUN luet install -y meta/cos-minimal`.
Afterwards we install k9s and nerdctl packages to create our derivative with those packages on it.

{{<package package="meta/cos-minimal" >}} is a meta-package that will pull {{<package package="toolchain/luet" >}}, {{<package package="toolchain/yip" >}}, {{<package package="utils/installer" >}}, {{<package package="system/cos-setup" >}}, {{<package package="system/immutable-rootfs" >}}, {{<package package="system/base-dracut-modules" >}}, {{<package package="system/grub2-config" >}}, {{<package package="system/cloud-config" >}}. 

{{% alert title="Note" %}}
{{<package package="system/cloud-config" >}} is optional, but provides `cOS` defaults setting, like default user/password and so on. If you are not installing it directly, an equivalent cloud-config has to be provided in order to properly boot and run a system, see [oem configuration](../../customizing/oem_configuration).
{{% /alert %}}

#### Using cosign in your derivative

The {{<package package="meta/cos-verify" >}} is a meta package that will pull {{<package package="toolchain/cosign" >}} and {{<package package="toolchain/luet-cosign" >}} .

{{<package package="toolchain/cosign" >}} and {{<package package="toolchain/luet-cosign" >}} are optional packages that would install cosign and luet-cosign in order to verify the packages installed by luet.

You can use cosign to both verify that packages coming from cos-toolkit are verified and sign your own derivative artifacts

{{% alert title="Note" %}}
If you want to manually verify cosign and luet-cosign packages before installing them with luet, you can do so by:
 - Install [Cosign](https://github.com/sigstore/cosign)
 - Export the proper vars
   - `export COSIGN_EXPERIMENTAL=1` for keyless verify
   - `export COSIGN_REPOSITORY=raccos/releases-green` to point cosign to the repo the signatures are stored on
 - Manually verify the signatures on both packages
   - Check the latest $VERSION for both packages at the repo (i.e. `https://quay.io/repository/costoolkit/releases-green?tab=tags`) 
   - `cosign verify quay.io/costoolkit/releases-green:luet-cosign-toolchain-$VERSION`
   - `cosign verify quay.io/costoolkit/releases-green:cosign-toolchain-$VERSION`
{{% /alert %}}


For more info, check the [cosign](../cosign) page.

## Initrd
The image should provide at least `grub`, `systemd`, `dracut`, a kernel and an initrd. Those are the common set of packages between derivatives. See also [package stack](../package_stack). 
By default the initrd is expected to be symlinked to `/boot/initrd` and the kernel to `/boot/vmlinuz`, otherwise you can specify a custom path while [building an iso](../build_iso) and [by customizing grub](../../customizing/configure_grub).

{{<package package="system/base-dracut-modules" >}} is required to be installed with `luet` in case you are building manually the initrd from the Dockerfile and also to run `dracut` to build the initrd, the command might vary depending on the base distro which was chosen.

{{<package package="system/kernel" >}} and {{<package package="system/dracut-initrd" >}} can also be installed if you plan to use kernels and initrd from the `cOS` repositories and don't build them / or install them from the official distro repositories (e.g. with `zypper`, or `dnf` or either `apt-get`...). In this case you don't need to generate initrd on your own, neither install the kernel coming from the base image.

## Building

![](https://docs.google.com/drawings/d/e/2PACX-1vS6eRyjnjdQI7OBO0laYD6vJ2rftosmh5eAog6vk_BVj8QYGGvnZoB0K8C6Qdu7SDz7p2VTxejcZsF6/pub?w=956&h=339)

The workflow would be then:

1) `docker build` the image
2) `docker push` the image to some registry
3) `elemental upgrade --no-verify --docker-image $IMAGE` from a cOS machine or (`elemental reset` if bootstrapping a cloud image)

The following can be incorporated in any standard gitops workflow.

You can explore more examples in the [example section](../../examples/creating_bootable_images) on how to create bootable images.

## What's next?

Now that we have created our derivative container, we can either:

- [Build an iso](../build_iso)
- [Build an Amazon Image](../packer/build_ami)
- [Build a Google Cloud Image](../packer/build_gcp)
