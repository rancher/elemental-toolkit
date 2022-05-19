---
title: "General Configuration"
linkTitle: "General Configuration"
weight: 3
date: 2017-01-05
description: >
  Configuring a cOS derivative
---


cOS during installation, reset and upgrade (`elemental install`, `elemental reset` and `elemental upgrade` respectively) will read a configuration file in order to apply derivative customizations. The configuration files are sourced in precedence order and can be located in the following places:

- `/etc/os-release`
- `<config-dir>/config.yaml`
- `<config-dir>/config.d/*/yaml`

By default `<config-dir>` is set to `/etc/elemental` however this can be changed to any custom path by using the `--config-dir` runtime flag.

Below you can find an example of the config file including most of the available options:

{{<githubembed repo="rancher-sandbox/elemental" file="config.yaml.example" lang="yaml">}}
