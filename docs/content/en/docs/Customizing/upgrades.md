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


By default, `cos` derivatives if not specified will point to latest `cos-toolkit`. To override, you need to or overwrite the content of `/system/oem/02_upgrades.yaml` or supply an additional one, e.g. `/system/oem/03_upgrades.yaml` in the final image, see [the default here](https://github.com/rancher-sandbox/cOS-toolkit/blob/master/packages/cloud-config/oem/02_upgrades.yaml).

## Configuration

`elemental upgrade` during start reads the [cOS configuration file](../general_configuration) and allows to tweak the following:

```bash
# Tweak the package to upgrade to, or the docker image (full reference)
ELEMENTAL_UPGRADE_IMAGE=system/cos
# Turn on/off channel upgrades. If disabled, UPGRADE_IMAGE should be a full reference to a container image
ELEMENTAL_CHANNEL_UPGRADES=true
# Disable mtree verification. Enabled by default
ELEMENTAL_NO_VERIFY=true
# Specify a separate recovery image (defaults to UPGRADE_IMAGE)
ELEMENTAL_RECOVERY_IMAGE=recovery/cos
```

`elemental upgrade` also reads its configuration from `/etc/cos-upgrade-image` if the file is present in the system.

Specifically, it allows to configure:

- **ELEMENTAL_UPGRADE_IMAGE**: A container image reference ( e.g. `registry.io/org/image:tag` ) or a `luet` package ( e.g. `system/cos` )
- **ELEMENTAL_CHANNEL_UPGRADES**: Boolean indicating wether to use channel upgrades or not. If it is disabled **UPGRADE_IMAGE** should refer to a container image, e.g. `registry.io/org/image:tag`
- **ELEMENTAL_NO_VERIFY**: Turns off or on mtree verification.
- **ELEMENTAL_RECOVERY_IMAGE**: Allows to specify a different image for the recovery partition. Similarly to **UPGRADE_IMAGE** needs to be either an image reference or a package.


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
  - "quay.io/costoolkit/releases-green"
```

