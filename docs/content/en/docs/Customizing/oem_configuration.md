---
title: "OEM configuration"
linkTitle: "OEM configuration"
weight: 3
date: 2017-01-05
description: >
  OEM configuration reserved to cOS and derivatives
---

There are several way to customize cOS and a cos-toolkit derivative:

- declaratively in runtime with cloud-config file (by overriding, or extending)
- stateful, embedding any configuration in the container image to be booted.

For runtime persistence configuration, the only supported way is with cloud-config files, [see the relevant docs](../configuration_persistency).

A derivative automatically loads and executes cloud-config files during the various system [stages](../stages) also inside `/system/oem` which is read-only and reserved to the system.

{{% alert title="Note" %}}
The cloud-config mechanism works also as an emitter event pattern - running services or programs can emit new custom `stages` in runtime by running `cos-setup stage_name`.
{{% /alert %}}

Derivatives that wish to override default configurations can do that by placing extra cloud-init file, or overriding completely `/system/oem` in the target image.

This is to setup for example, the default root password or the prefered upgrade channel. 

The following are the `cOS` default oem files, which are shipped within the {{<package package="system/cloud-config" >}}, which is an optional package:

```
/system/oem/00_rootfs.yaml - defines the rootfs mountpoint layout setting
/system/oem/01_defaults.yaml - systemd defaults (keyboard layout, timezone)
/system/oem/02_upgrades.yaml - Settings for cOS vanilla channel upgrades
/system/oem/03_branding.yaml - Branding setting, Derivative name, /etc/issue content
/system/oem/04_accounting.yaml - Default user/pass
/system/oem/05_network.yaml - Default network setup
/system/oem/06_recovery.yaml - Executes additional commands when booting in recovery mode
```

You can either override the above files, or alternatively not consume the {{<package package="system/cloud-config" >}} package while building a derivative.
