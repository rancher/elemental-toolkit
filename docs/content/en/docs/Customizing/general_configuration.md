---
title: "General Configuration"
linkTitle: "General Configuration"
weight: 3
date: 2017-01-05
description: >
  Configuring a cOS derivative
---


cOS during installation, reset and upgrade (`elemental install`, `elemental reset` and `elemental upgrade` respectively) will read a configuration file in order to apply derivative customizations. The configuration files are sourced in precedence order and can be located in the following places:

- `/etc/environment`
- `/etc/os-release`
- `/etc/cos/config`

Below you can find a full example of the config file:

{{<githubembed repo="rancher-sandbox/cos-toolkit" file="packages/cos-config/cos-config" lang="bash">}}
