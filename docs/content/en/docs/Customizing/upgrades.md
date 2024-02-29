---
title: "Upgrades"
linkTitle: "Upgrades"
weight: 3
date: 2023-05-31
description: >
  Customizing the default upgrade channel
---

`Elemental` vanilla images by default are picking upgrades by the standard upgrade channel. It means it will always get the latest published `Elemental` version by our CI.

However, it's possible to tweak the default behavior of `elemental upgrade` to point to a specific OCI image/tag, or a different release channel.

## Configuration

`elemental upgrade` during start reads the [Elemental configuration file](../general_configuration) and allows to tweak the following:

```yaml
# configuration used for the 'ugrade' command
upgrade:
  # if set to true upgrade command will upgrade recovery system instead
  # of main active system
  recovery: false

  # image used to upgrade main OS
  # size in MiB
  system:
    uri: <image-spec>

  # image used to upgrade recovery OS
  # recovery images can be set to use squashfs
  recovery-system:
    fs: squashfs
    uri: oci:recovery/cos
```

The `system` and `recovery-system` objects define the OS image used for the main active system and the recovery system respectively. They both are fined by a `<image-spec>`.

## Upgrading Recovery only

The `elemental upgrade-recovery` command can be used to upgrade the Recovery system only.  
The `upgrade` configuration can be used seamlessly between `upgrade` and `upgrade-recovery`, but for the latter only the `recovery-system` field applies:  

```yaml
# configuration used for the 'ugrade-recovery' command
upgrade:
  recovery-system:
    fs: squashfs
    uri: oci:recovery/cos
```

It is also possible to invoke it directly with: `elemental upgrade-recovery --recovery-system.uri oci:recovery/cos`
