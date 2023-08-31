
---
title: "Login"
linkTitle: "Login"
weight: 3
date: 2023-08-31
description: >
  Default login, and how to override it
---

By default you can login with the user `root` and `cos` in a vanilla Elemental image, this is also set automatically by the `cloud-config-defaults` feature if used by a derivative.

You can change this by overriding `/system/oem/04_accounting.yaml` in the container image if present, or via [cloud-init](../../reference/cloud_init/#stagesstage_idstep_nameusers).

### Examples
- [Example accounting file](https://github.com/rancher/elemental/blob/main/framework/files/system/oem/04_accounting.yaml)
