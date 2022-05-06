---
title: "Configuration persistency"
linkTitle: "Configuration persistency"
weight: 2
date: 2017-01-05
description: >
  Persisting configurations in cOS and derivatives
---


By default cOS and derivatives are reading and executing cloud-init files in (lexicopgrahic) sequence inside:

- `/system/oem`
- `/usr/local/cloud-config` 
- `/oem` 

It is also possible to run cloud-init file in a different location (URLs included, too) from boot cmdline by using  the `cos.setup=..` option.

{{% alert title="Note" %}}
It is possible to install a custom [cloud-init style file](../../reference/cloud_init/) during install with `--cloud-init` flag on `elemental install` command or, it's possible to add one or more files manually inside the `/oem` directory after installation.
{{% /alert %}}

While `/system/oem` is reserved for system configurations to be included directly in the derivative container image, the `/oem` folder instead is reserved for persistent cloud-init files that can be extended in runtime.

For example, if you want to change `/etc/issue` of the system persistently, you can create `/usr/local/cloud-config/90_after_install.yaml` or alternatively in `/oem/90_after_install.yaml` with the following content:

```yaml
# The following is executed before fs is setted up:
stages:
    fs:
        - name: "After install"
          files:
          - path: /etc/issue
            content: |
                    Welcome, have fun!
            permissions: 0644
            owner: 0
            group: 0
          systemctl:
            disable:
            - wicked
        - name: "After install (second step)"
          files:
          - path: /etc/motd
            content: |
                    Welcome, have more fun!
            permissions: 0644
            owner: 0
            group: 0
```

For more examples you can find `/system/oem` inside cOS vanilla images containing files used to configure on boot a pristine `cOS`. 
