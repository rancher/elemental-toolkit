---
title: "Creating bootable images"
linkTitle: "Creating bootable images"
weight: 2
date: 2017-01-05
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
- grub (required)
- dracut (optional, kernel and initrd can be consumed from the Elemental repositories)
- microcode (optional, not required in order to boot, but recomended)
- [cosign and luet-cosign](../cosign) packages (optional, required if you want to verify the images installed by luet)

## Example

An illustrative example can be:


{{<githubembed repo="rancher/elemental-toolkit" file="examples/standard/Dockerfile" lang="Dockerfile">}}

With the config file:

{{<githubembed repo="rancher/elemental-toolkit" file="examples/standard/conf/luet.yaml" lang="yaml">}}


In the example above, the elemental-toolkit parts that are **required** are pulled in by `RUN luet install -y meta/cos-minimal`.
Afterwards we install k9s and nerdctl packages to create our derivative with those packages on it.

### Meta packages

The toolkit is split into several meta-packages to pull only the individual needed part or a subset of them which are tied to a specific purpose, the meta packages are [documented in their own section](../../reference/packages/meta-packages).

{{<package package="meta/cos-minimal" >}} is a meta-package that will pull {{<package package="toolchain/luet" >}}, {{<package package="toolchain/yip" >}}, {{<package package="utils/installer" >}}, {{<package package="system/cos-setup" >}}, {{<package package="system/immutable-rootfs" >}}, {{<package package="system/base-dracut-modules" >}}, {{<package package="system/grub2-config" >}}, {{<package package="system/cloud-config" >}}. 

{{< tabpane >}}
  {{< tab header="Minimal" subtitle="foo" >}}
# This meta package will pull the minimal set to have a bootable system. 
# This includes default username/password set as well (`root/cos`) during boot

RUN luet install -y meta/cos-minimal
  {{< /tab >}}
  {{< tab header="Core">}}
# The core subset includes the packages needed to have a bootable system. 
# This does not include any specific system configuration.

RUN luet install -y meta/cos-core

# Can be used in conjuction with others, to achieve the desired configuration

RUN luet install -y meta/cos-core cloud-config/live cloud-config/network
  {{< /tab >}}
  {{< tab header="Light" >}}
# The light subset includes the packages needed to have a bootable system, omitting any compiled binaries.
# This does not include as well any specific system configuration.

RUN luet install -y meta/cos-light

# Can be used in conjuction with others, to achieve the desired configuration

RUN luet install -y meta/cos-light meta/toolchain cloud-config/live cloud-config/network
  {{< /tab >}}
  {{< tab header="Toolchain" >}}
# The Toolchain subset includes only compiled binaries required into the rootfs image. 
# This does not include as well any specific system configuration.

RUN luet install -y meta/toolchain

# Can be used in conjuction with others, to achieve the desired configuration

RUN luet install -y meta/cos-light meta/toolchain
  {{< /tab >}}
{{< /tabpane >}}



{{% alert title="Note" %}}
{{<package package="system/cloud-config" >}} is optional, but provides `Elemental` defaults setting, for example default user/password, rootfs layout, and more. If you are not installing it directly, an equivalent cloud-config or a set has to be provided in order to properly boot and run a system, see [oem configuration](../../customizing/oem_configuration).
Individual cloud-configs can be installed as well as are available as standalone packages.
{{% /alert %}}

#### Using cosign in your derivative

The {{<package package="meta/cos-verify" >}} is a meta package that will pull {{<package package="toolchain/cosign" >}} and {{<package package="toolchain/luet-cosign" >}} .

{{<package package="toolchain/cosign" >}} and {{<package package="toolchain/luet-cosign" >}} are optional packages that would install cosign and luet-cosign in order to verify the packages installed by luet.

You can use cosign to both verify that packages coming from elemental-toolkit are verified and sign your own derivative artifacts

{{% alert title="Note" %}}
If you want to manually verify cosign and luet-cosign packages before installing them with luet, you can do so by:
 - Install [Cosign](https://github.com/sigstore/cosign)
 - Export the proper vars
   - `export COSIGN_EXPERIMENTAL=1` for keyless verify
 - Manually verify the signatures on both packages
   - Check the latest $VERSION for both packages at the repo (i.e. `https://quay.io/repository/costoolkit/releases-teal?tab=tags`) 
   - `cosign verify quay.io/costoolkit/releases-teal:luet-cosign-toolchain-$VERSION`
   - `cosign verify quay.io/costoolkit/releases-teal:cosign-toolchain-$VERSION`
{{% /alert %}}


For more info, check the [cosign](../cosign) page.

## Initrd
The image should provide at least `grub`, `systemd`, `dracut`, a kernel and an initrd. Those are the common set of packages between derivatives. See also [package stack](../package_stack). 
By default the initrd is expected to be symlinked to `/boot/initrd` and the kernel to `/boot/vmlinuz`, otherwise you can specify a custom path while [building an iso](../build_iso) and [by customizing grub](../../customizing/configure_grub).

{{<package package="system/base-dracut-modules" >}} is required to be installed with `luet` in case you are building manually the initrd from the Dockerfile and also to run `dracut` to build the initrd, the command might vary depending on the base distro which was chosen.

{{<package package="system/kernel" >}} and {{<package package="system/dracut-initrd" >}} can also be installed if you plan to use kernels and initrd from the `Elemental` repositories and don't build them / or install them from the official distro repositories (e.g. with `zypper`, or `dnf` or either `apt-get`...). In this case you don't need to generate initrd on your own, neither install the kernel coming from the base image.

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
- [Build an Amazon Image](../packer/build_ami)
- [Build a Google Cloud Image](../packer/build_gcp)
