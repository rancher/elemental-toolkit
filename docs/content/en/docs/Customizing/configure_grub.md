---
title: "GRUB"
linkTitle: "GRUB"
weight: 4
date: 2017-01-05
description: >
  GRUB 2 Configuration
---

COS is set to deploy a persistent `grub.cfg` into the `COS_RECOVERY` partition during
the system installation or image creation. COS grub configuration
includes three menu entries: first for the main OS system, second for the
fallback OS system and a third for the recovery OS.

For example the main OS system menu entry could be something like:

```
menuentry "cOS" --id cos {
  search.fs_label COS_STATE root
  set img=/cOS/active.img
  set label=COS_ACTIVE
  loopback loop0 /$img
  set root=($root)
  source (loop0)/etc/cos/bootargs.cfg
  linux (loop0)$kernel $kernelcmd
  initrd (loop0)$initramfs
}
```

{{% alert title="Kernel parameters" %}}
The kernel parameters are not part of the persistent `grub.cfg` file stored in `COS_RECOVERY` partition. 
Kernel parameters are sourced from the loop device of the OS image to boot. This is mainly to
keep kernel parameters consistent across different potential OS images or
system upgrades. 
{{% /alert %}}

## Specifying default custom boot options

cOS images and its derivatives, are expected to include a
`/etc/cos/bootargs.cfg` file which provides the definition of the following
variables:

* `$kernel`: Path of the kernel binary 
* `$kernelcmd`: Kernel parameters
* `$initramfs`: Path of the initrd binary

This is the mechanism any cOS image or cOS derivative has to communicate
its boot parameters (kernel, kernel params and initrd file) to GRUB2.

For example, the default cOS bootarg.cfg file is:

```
set kernel=/boot/vmlinuz
if [ -n "$recoverylabel" ]; then
    # Boot arguments when the image is used as recovery
    set kernelcmd="console=tty1 root=live:CDLABEL=$recoverylabel rd.live.dir=/ rd.live.squashimg=$img panic=5"
else
    # Boot arguments when the image is used as active/passive
    set kernelcmd="console=tty1 root=LABEL=$label iso-scan/filename=$img panic=5 security=selinux rd.cos.oemlabel=COS_OEM selinux=1"
fi

set initramfs=/boot/initrd
```

You can tweak that file to suit your needs if you need to specify persistent boot arguments.

{{% alert title="Note" %}}
`rd.cos.oemlabel=COS_OEM` is required inside the bootargs in order to access to the `/oem` mount within the rootfs stage. `COS_OEM` is the default label for the oem partition.
{{% /alert %}}

## Grub environment variables

cOS (since v0.5.8) makes use of the GRUB2 environment block which can used to define
persistent GRUB2 variables across reboots.

Use `grub2-editenv` command line utility to define the desired values.

| Variable               |  Description                                            |
|------------------------|---------------------------------------------------------|
| next_entry             | Set the next reboot entry                               |
| saved_entry            | Set the default boot entry                              |
| default_menu_entry     | Set the name entries on the GRUB menu                   |
| extra_active_cmdline   | Set additional boot commands when booting into active   |
| extra_passive_cmdline  | Set additional boot commands when booting into passive  |
| extra_recovery_cmdline | Set additional boot commands when booting into recovery |
| extra_cmdline          | Set additional boot commands for all entries            |
| default_fallback       | Sets default fallback logic                             |


For instance use the following command to reboot to recovery system only once:

```bash
> grub2-editenv /oem/grubenv set next_entry=recovery
```

{{% alert title="Note" %}}
The examples below make of the `COS_OEM` device, however it could use any device
detected by GRUB2 that includes the file `/grubenv`. First match wins.
{{% /alert %}}

### Default boot entry

The default grub configuration loads the `/grubenv` of any available device
and evaluates on `next_entry` variable and `saved_entry` variable. By default
none is set.

The default boot entry is set to the value of `saved_entry`, in case the variable
is not set grub just defaults to the first menu entry.

`next_entry` variable can be used to overwrite the default boot entry for a single
boot. If `next_entry` variable is set this is only being used once, GRUB2 will
unset it after reading it for the first time. This is helpful to define the menu entry
to reboot to without having to make any permanent config change.

Use `grub2-editenv` command line utility to define desired values.

For instance use the following command to reboot to recovery system only once:

```bash
> grub2-editenv /oem/grubenv set next_entry=recovery
```

Or to set the default entry to `fallback` system:

```bash
> grub2-editenv /oem/grubenv set saved_entry=fallback
```

## Boot menu

By default `cOS` and derivatives shows the default boot menu entry while booting (`cOS`).

The grub menu entry is generated during installation and can be configured by setting `GRUB_ENTRY_NAME` in the [cOS configuration file](../general_configuration) inside the derivative, or either via cloud-init before installation.

For example, specifying in `/etc/cos/config`:

```bash
GRUB_ENTRY_NAME=myOS
```

will automatically set the GRUB menu entries for active, passive and recovery to the specified value.

The grub menu boot entry can also be set with `grub2-editenv`:

```bash
> grub2-editenv /oem/grubenv set default_menu_entry=fooOS
```

{{% alert title="Additional menu entries" %}}

Since {{<package package="system/grub2-config" >}} >= 0.0.14 it is possible to add multiple custom menu entries to GRUB by creating a `/grubmenu` config file in one of the available partitions detected by GRUB2 during boot. The file will be loaded from the first partition found by GRUB that have the `grubmenu` file. First match wins. 
The `grubmenu` file will be sourced at the end of the boot process, and can contain several `menuentry` blocks.

{{% /alert %}}

## Persistent boot option flags

It is possible to define persistent boot flag for each menu entry also via `grub2-editenv`:

- `extra_active_cmdline`: extra bootflags to be applied only on active boot
- `extra_passive_cmdline`: extra bootflags to be applied only on passive boot
- `extra_recovery_cmdline`: extra bootflags to be applied only on recovery
- `extra_cmdline`: will be applied to each boot entry


## Customizing fallback logic

By default cOS boots into active, and if there are failures will boot into the passive, and finally if keeps failing, will boot into recovery.

It is possible to override the default fallback logic by setting `default_fallback` as grub environment, consider for example:

```bash
> grub2-editenv /oem/grubenv set default_fallback="2 0 1"
```

Will set the default fallback to "2 0 1" instead of the default "0 1 2".
