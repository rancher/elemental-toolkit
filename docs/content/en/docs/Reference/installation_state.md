---
title: "Installation State"
linkTitle: "Installation State"
weight: 7
date: 2024-09-10
description: >
  Installation State
---

The Elemental toolkit installation state can be inspected at any time by running `elemental state`.  

The installation state provides information regarding the system and all deployed snapshots.  

```yaml
# The last state update date.
date: "2024-09-09T12:33:25Z"
# Snapshotter configuration.
snapshotter:
    type: btrfs
    max-snaps: 4
    config: {}
efi:
    label: COS_GRUB
oem:
    label: COS_OEM
persistent:
    label: COS_PERSISTENT
recovery:
    label: COS_RECOVERY
    # Recovery snapshot information.
    recovery:
        source: dir:///run/rootfsbase
        fs: squashfs
        labels:
            reason: provisioning
        date: "2024-09-09T12:31:50Z"
        fromAction: install
state:
    label: COS_STATE
    snapshots:
        1:
            source: dir:///run/rootfsbase
            labels:
                reason: provisioning
            date: "2024-09-09T12:31:50Z"
            fromAction: install
        2:
            # The source for the snapshot.
            source: oci://my-os-image:v1.2.3
            # If the source is an image, digest is provided.
            digest: sha256:11cb5c6f7b6b9e4daff67ec3ec7fa4a028c24445f2e4834a78e93c75d73eb5c3
            # Active snapshots are automatically started by the bootloader.
            active: true
            # User defined labels
            labels:
                reason: automatic-update
                version: v1.2.3
            # Creation date of this snapshot
            date: "2024-09-09T12:33:25Z"
            # elemental action that created this snapshot (upgrade, upgrade-recovery, install, reset).
            fromAction: upgrade
```

In order to correlate and identify snapshots, it is possible to add user defined labels to the `elemental` commands using the `--snapshot-labels` argument.  
For example during upgrades: `elemental upgrade --system oci://my-os-image:v1.2.3 --snapshot-labels reason=foo,version=v1.2.3`.  
Or install and resets: `elemental install --snapshot-labels reason=provisioning`, `elemental reset --snapshot-labels reason=decommissioning`
