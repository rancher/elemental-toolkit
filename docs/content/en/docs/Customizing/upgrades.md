---
title: "Upgrades"
linkTitle: "Upgrades"
weight: 3
date: 2017-01-05
description: >
  Customizing the default upgrade channel
---

`cOS` vanilla images by default are picking upgrades by the standard upgrade channel. It means it will always get the latest published `cOS` version by our CI.

However, it's possible to tweak the default behavior of `elemental upgrade` to point to a specific docker image/tag, or a different release channel.


By default, `cos` derivatives if not specified will point to latest `elemental-toolkit`. To override, you need to or overwrite the content of `/system/oem/02_upgrades.yaml` or supply an additional one, e.g. `/system/oem/03_upgrades.yaml` in the final image, see [the default here](https://github.com/rancher/elemental-toolkit/blob/master/packages/cloud-config/oem/02_upgrades.yaml).

## Configuration

`elemental upgrade` during start reads the [cOS configuration file](../general_configuration) and allows to tweak the following:

```yaml
# configuration used for the 'ugrade' command
upgrade:
  # if set to true upgrade command will upgrade recovery system instead
  # of main active system
  recovery: false

  # image used to upgrade main OS
  # size in MiB
  system:
    <image-spec>

  # image used to upgrade recovery OS
  # recovery images can be set to use squashfs
  recovery-system:
    fs: squashfs
    uri: channel:recovery/cos
```

The `system` and `recovery-system` objects define the OS image used for the main active system and the recovery system respectively. They both are fined by a `<image-spec>`.

## Changing the default release channel

Release channels are standard luet repositories. To change the default release channel, create a `/etc/luet/luet.yaml` configuration file pointing to a valid luet repository:

```yaml
# For a full reference, see:
# https://luet-lab.github.io/docs/docs/getting-started/#configuration
logging:
  color: false
  enable_emoji: false
general:
    debug: false
    spinner_charset: 9
repositories:
- name: "sampleos"
  description: "sampleOS"
  type: "docker"
  enable: true
  cached: true
  priority: 1
  verify: false
  urls:
  - "quay.io/costoolkit/releases-teal"
```

Alternatively a repositories list can be included in `/etc/elemental/config.yaml` file and this will not affect system wide Luet configuration, see [general configuration](../../customizing/general_configuration) for a repositories setup example.
