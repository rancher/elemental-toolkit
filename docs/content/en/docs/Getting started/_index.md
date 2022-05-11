---
title: "Getting Started"
linkTitle: "Getting Started"
weight: 1
description: >
  Getting started with cOS
---

![](https://docs.google.com/drawings/d/e/2PACX-1vRSuocC4_2rHeJAWW2vqinw_EZeZxTzJFo5ZwnJaL_sdKab_R_OsCTLT_LFh1_L5fUcA_2i9FIe-k69/pub?w=1223&h=691)


cOS provides a runtime and buildtime framework in order to boot containers in VMs, Baremetals and Cloud.

You can either choose to **build** a cOS derivative or **run** cOS to boostrap a new system.

cOS vanilla images are published to allow to deploy user-built derivatives. 

cOS is designed to run, deploy and upgrade derivatives that can be built just as standard OCI container images. cOS assets can be used to either drive unattended deployments of a derivative or used to create custom images (with packer).

## Philosophy

{{<image_right image="https://docs.google.com/drawings/d/e/2PACX-1vRLayrWAJo6g8ssUwKmREIkwcOHWOn_nlUUNgFxkn9HcZkE3RrAXTBWd4gVj1rxPHg559kAzUk_rsqr/pub?w=384&h=255">}}

Philosophy behind cos-toolkit is simple: it allows you to create Linux derivatives from container images.

- **Container registry as a single source of truth**
- Hybrid way to access your image for different scopes (development, debugging, ..)
- No more inconsistent states between nodes. A “Store” to keep your (tagged) shared states where you can rollback and upgrade into.
- “Stateless”: Images with upgrades are rebuilt from scratch instead of applying upgrades. 
- A/B upgrades, immutable systems

The container image is booted as-is, encapsulating all the needed components (kernel, initrd included) and can be pulled locally for inspection, development and debugging. At the same time it can be used also to create installation medium as ISO, Raw images, OVA or Cloud specific images.

A derivative automatically inherits the following featureset:
- [Can upgrade to another container image](./upgrading)
- [Can deploy a system from scratch from an image](./deploy)
- [Reset or recovery to a specific image](./recovery)
- [Customize the image during runtime to persist changes across reboots](../customizing/runtime_persistent_changes)
- [Perform an installation from the LiveCD medium](./booting)

## Building cOS derivatives

The starting point to use cos-toolkit is to check out our [examples](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/examples) and our [creating bootable images](../creating-derivatives/creating_bootable_images) section.

The only requirement to build derivatives with `cos-toolkit` is Docker installed. If you are interested in building cOS-toolkit itself, see [Development notes](../development).

The toolkit itself is delivered as a set of standalone, re-usable OCI artifacts which are tagged and tracked as standard OCI images and it is installed inside the container image to provide the same featureset among derivatives, see [how to create bootable images](../creating-derivatives/creating_bootable_images).

## Vanilla images

`cOS` releases are composed of vanilla images that are used internally for testing and can be used as a starting point to deploy derivatives in specific environments (e.g. AWS) or just to try out the cOS featureset. 

The vanilla images ships no specific business-logic aside serving as a base for testing and deploying other derivatives.

### What to do next?

Check out [how to create bootable images](../creating-derivatives/creating_bootable_images) or [download the cOS vanilla images](../getting-started/download) to give cOS a try!

Here below you will find the common documentation that applies to any derivative built with cOS and the cOS vanilla images.