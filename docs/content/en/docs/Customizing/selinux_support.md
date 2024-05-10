---
title: "SELinux Support"
linkTitle: "SELinux Support"
weight: 4
date: 2022-06-10
description: >
  SELinux support in Elemental
---

Elemental includes basic support for SELinux. From an elemental perspective SELinux is some custom configurationt that requires special treatment. Being specific it mostly nails down to apply SELinux labels at install and upgrade time. Since the rootfs is readonly they can't be easily applied at runtime or at boot time. As consequence of that SELinux autorelabel service should not be used within elemental as it expects a RW root and persistency across reboots (it essentially reboots after appliying labels). For the time being Elemental only considers the `targeted` SELinux policy.

`elemental-cli` utility applies SELinux contexts to the installed/upgraded system if three conditions are met:

* the installed system includes the `setfiles` command
* the installed system includes the targeted files context (`/etc/selinux/targeted/contexts/files/file_contexts` file)
* the binary for `targeted` policy is also present (`/etc/selinux/targeted/policy/policy.*` file)

In an Elemental workflow SElinux context labels should be applied at install/upgrade time for the readonly areas, but this is not enough as it doesn't cover the ephemeral filesystems (overlayfs on top of tmpfs), which are usually sensitive paths like `/etc/`, `/var`, `/srv`, etc. In order to properly apply file contexts over the ephemeral paths the relabelling has to happen at boot time once those overlayfs are created. During boot the `elemental mount` command will try to relabel the files in ephemeral and persistent storage if it can find the correct policy and setfiles utility in the mounted system.

## Using custom SELinux modules

Making use of `selinux` and including SELinux utilities and targeted policy within the base OS it is enough to get started with SELinux, however there is a great chance that this is too generic and requires some additional policy modules to be fully functional according to each specific use case.

The Type Enforcement file was created by booting an Elemental OS on permissive mode using `audit2allow` and other SELinux related utilities to generate the custom module out of the reported denials. Something like:

```bash
# Create the type enforcement file
cat /var/log/audit/audit.log | audit2allow -m elemental > elemental.te

# Create the policy module
checkmodule -M -m -o elemental.mod elemental.te

# Create the policy package out of the module
semodule_package -o elemental.pp -m elemental.mod
```

To make effective the policy package it has to be loaded or installed within the selinux policy, this can be easily done with the `semodule -i /usr/share/elemental/selinux/elemental.pp` command. So from a derivative perspective and following the example from [Creating bootable image](../../creating-derivatives/creating_bootable_images/#example) section adding the following lines to the Dockerfile should be enough to enable SELinux in enforcing mode:

```Dockerfile
# Install the custom policy package if any and the restore context stage in cloud-init config
RUN elemental init --force --features=cloud-config-defaults

# Load the policy package
RUN semodule -i /usr/share/elemental/selinux/elemental.pp

# Enable selinux in enforcing mode
RUN sed -i "s|^SELINUX=.*|SELINUX=enforcing|g" /etc/selinux/config
```

The above assumes the base image already includes the SELinux packages and utilities provided by the underlaying distro. It is suggested to set the enforcing mode via the config file rather than setting grub with the selinux kernel parameter (`enforcing=1`), this way it is easier, at any time, to temporarily add `enforcing=0` at runtime within the grub2 shell and temporarily set SELinux in permissive mode.

Notes when using a SELinux version prior to v3.4. If `libsemanage` version is lower than v3.4, it is likely that the `semodule -i *.pp` command fails with a cross-device linking issue, this is a known [issue](https://github.com/SELinuxProject/selinux/issues/343) upstream and already fixed since v3.4. Command `selinux -i <file>` mutates files under `/var/lib/selinux/targeted` and used to rename some files, this can be tricky when executed inside a container as hardlinks across filesystems are not permitted and this is actually what happens if the overlayfs driver is used. This can be worked around if all the originally mutated files are already modified within the execution layer (so they are part of the upper layer of the overlayfs). So the above specific example could be rewritten as:

```Dockerfile
# Install the custom policy package if any and the restore context stage in cloud-init config
RUN elemental init --force --features=cloud-config-defaults

# Artificially modify selinux files to copy them in within the overlyfs and then load the policy package
RUN mv /var/lib/selinux/targeted/active /var/lib/selinux/targeted/previous &&\
    cp --link --recursive /var/lib/selinux/targeted/previous /var/lib/selinux/targeted/active &&\
    semodule -i /usr/share/elemental/selinux/elemental.pp

# Enable selinux in enforcing mode
RUN sed -i "s|^SELINUX=.*|SELINUX=enforcing|g" /etc/selinux/config
```
