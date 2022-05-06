---
title: "Package stack"
linkTitle: "Package stack"
weight: 1
date: 2017-01-05
description: >
  Package stack for derivatives
---


When building a `cos-toolkit` derivative, a common set of packages are provided already with a common default configuration. Some of the most notably are:

- systemd as init system
- grub for boot loader
- dracut for initramfs

Each `cos-toolkit` flavor (opensuse, ubuntu, fedora) ships their own set of base packages depending on the distribution they are based against. You can find the list of packages in the `packages` keyword in the corresponding [values file for each flavor](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/values)