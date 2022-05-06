
---
title: "Login"
linkTitle: "Login"
weight: 3
date: 2017-01-05
description: >
  Default login, and how to override it
---

By default you can login with the user `root` and `cos` in a vanilla cOS image, this is also set automatically by the {{<package package="system/cloud-config" >}} package if used by a derivative.

You can change this by overriding `/system/oem/04_accounting.yaml` in the container image if present, or via [cloud-init](../../reference/cloud_init/#stagesstage_idstep_nameusers).

### Examples
- [Example accounting file](https://github.com/mudler/c3os/blob/master/files/system/oem/10_accounting.yaml)
