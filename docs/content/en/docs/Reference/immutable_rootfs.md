---
title: "Immutable Root Filesystem"
linkTitle: "Immutable Rootfs"
weight: 2
date: 2023-05-11
description: >
  Immutable root filesystem configuration parameters
---

The immutable rootfs concept in Elemental is provided by a dracut module.
By default, `elemental` and derivatives will inherit an immutable setup.

![Partitioning layout](https://docs.google.com/drawings/d/e/2PACX-1vR-I5ZwwB5EjpsymUfcNADRTTKXrNMnlZHgD8RjDpzYhyYiz_JrWJwvpcfMcwfYet1oWCZVWH22aj1k/pub?w=533&h=443)

A running system will look like as follows:

```
/usr/local - persistent (COS_PERSISTENT)
/oem - persistent (COS_OEM)
/etc - ephemeral
/usr - read only
/ immutable
```

This means that any changes that are not specified as cloud-init configuration are not persisting across reboots.

You can place persisting cloud-init files either in `/oem` or `/usr/local/oem`, `Elemental` already supports cloud-init [datasources](https://cloudinit.readthedocs.io/en/latest/topics/datasources.html), so you can use also load cloud-init configuration as standard userdata, depending on the platform. For more details on the cloud-init syntax, see the [cloud-init configuration reference](../reference/cloud_init).
