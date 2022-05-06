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

Philosophy behind cos-toolkit is simple: it allows you to create Linux derivatives from container images.

- **Container registry as a single source of truth**
- Hybrid way to access your image for different scopes (development, debugging, ..)
- No more inconsistent states between nodes. A “Store” to keep your (tagged) shared states where you can rollback and upgrade into.
- “Stateless”: Images with upgrades are rebuilt from scratch instead of applying upgrades. 

A derivative which includes cos-toolkit, in runtime can:

{{<image_left image="https://docs.google.com/drawings/d/e/2PACX-1vRLayrWAJo6g8ssUwKmREIkwcOHWOn_nlUUNgFxkn9HcZkE3RrAXTBWd4gVj1rxPHg559kAzUk_rsqr/pub?w=384&h=255">}}

- [Upgrade to another container image](./upgrading)
- [Deploy a system from scratch from an image](./deploy)
- [Reset or recovery to an Image](./recovery)
- [Customize the image during runtime to persist changes across reboots](../customizing/runtime_persistent_changes)
- [Perform an installation from Live medium](./booting)

The container image, seamlessly:
- is booted as-is, encapsulating all the needed components (kernel, initrd, cos-toolkit, ecc)
- can be pulled locally for inspection, development and debugging
- can be used to create installation medium as ISO, Raw images, OVA, Cloud

## Build cOS derivatives


The starting point to use cos-toolkit is to check out our [examples](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/examples) and our [creating bootable images](../creating-derivatives/creating_bootable_images) section.

The only requirement to build derivatives with `cos-toolkit` is Docker installed. If you are interested in building cOS-toolkit itself, see [Development notes](../development).

The toolkit itself is delivered as a set of standalone, re-usable OCI artifacts which are tagged and tracked as standard OCI images and it is installed inside the container image to provide the same featureset among derivatives, see [how to create bootable images](../creating-derivatives/creating_bootable_images).

### What to do next?

Check out [how to create bootable images](../creating-derivatives/creating_bootable_images) or [download the cOS vanilla images](../getting-started/download) to give cOS a try!
