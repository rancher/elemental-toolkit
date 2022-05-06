---
title: "Runtime persistent changes"
linkTitle: "Runtime persistent changes"
weight: 3
date: 2021-09-24
description: >
  Applying changes to cOS images in runtime or “how to install a package in an immutable OS at runtime?”
---

cOS and derivatives are [immutable](../../reference/immutable_rootfs) systems. That means that any change in the running OS will not persist after a reboot.

While [configurations can be persisted](../configuration_persistency), there are occasions where installing a custom package or provide additional persistent files in the end system is needed.

We will see here a way to install packages, drivers, or apply any modification we might want to do in the OS image during runtime, without any need to rebuild the derivative container image. This will let any user (and not derivative developer) to apply any needed customization and to be able to persist across upgrades.

## Transient changes

To apply transient changes, it's possible to boot a cOS derivative in read/write mode by specifying `rd.cos.debugrw` [see here for more details](../../reference/immutable_rootfs). This allows to do any change and will persist into the active/passive booting system (does NOT apply for recovery). Altough this methodology should be only considered for debugging purposes.

## Persist changes with Cloud init files

Note: The following applies only to derivatives with {{<package package="utils/installer" >}} at version `0.17` or newer

cOS allows to apply a set of commands, or cloud-init steps, during upgrade, deploy, install and reset in the context of the target image, in RW capabilities. This allows to carry on changes during upgrades on the target image without the need to re-build or have a custom derivative image.

All the configuration that we want to apply to the system will run each time we do an upgrade, a reset or an installation on top of the new downloaded image (in case of upgrade) or the image which is the target system. 

Between the available [stages](../stages) in the [cloud-init](../../reference/cloud_init/) there are `after-upgrade-chroot`,  `after-install-chroot`, `after-reset-chroot` and  `after-deploy-chroot`, for example, consider the following cloud-init file:

```yaml
stages:
name: "Install something"
stages:
   after-upgrade-chroot:
     - commands:
        - zypper in -y ...
   after-reset-chroot:
     - commands:
        - zypper in -y ...
   after-deploy-chroot:
     - commands:
        - zypper in -y ...
   after-install-chroot:
     - commands:
        - zypper in -y ...
```

It will run the `zypper in -y ...` calls during each stage, in the context of the target system, allowing to customize the target image with additional packages. 

{{% alert title="Note" %}}
`zypper` calls here are just an example. We could have used `dnf` for fedora based, or `apt-get` for ubuntu based images.
{{% /alert %}}

When running the cloud-init steps the `/oem` partition and `/usr/local` will be mounted to `COS_OEM` and `COS_PERSISTENT` respectively, allowing to load extra data (e.g. rpm files, or configuration).

## Example

If an user wants to install an additional package in the running system, and keep having that persistent across upgrades, he can copy the following file (`install.yaml`) inside the `/oem` folder, or `/usr/local/cloud-config`:

```yaml
stages:
name: "Install something"
stages:
   after-upgrade-chroot:
     - commands:
        - zypper in -y vim
```

and run `elemental upgrade`. 

It will automatically upgrade the system with the changes above included.
