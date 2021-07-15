# Grub configuration

cOS set to deploy a persistent `grub.cfg` into the `COS_RECOVERY` partition during
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

Someting relevant to note is that the kernel parameters are not part of the 
persistent `grub.cfg` file stored in `COS_RECOVERY` partition. Kernel parameters
are sourced from the loop device of the OS image to boot. This is mainly to
keep kernel parameters consistent across different potential OS images or
system upgrades. 

In fact, cOS images and its derivatives, are expected to include a
`/etc/cos/bootargs.cfg` file which provides the definition of the following
variables:

* `$kernel`: Path of the kernel binary 
* `$kernelcmd`: Kernel parameters
* `$initramfs`: Path of the initrd binary

This is the mechanism any cOS image or cOS derivative has to communicate
its boot parameters (kernel, kernel params and initrd file) to grub2.

## Grub2 default boot entry setup

cOS (since v0.5.8) makes use of the grub2 environment block which can used to define
persistent grub2 variables across reboots.

The default grub configuration loads the `/grubenv` of any available device
and evaluates on `next_entry` variable and `saved_entry` variable. By default
none is set.

The default boot entry is set to the value of `saved_entry`, in case the variable
is not set grub just defaults to the first menu entry.

`next_entry` variable can be used to overwrite the default boot entry for a single
boot. If `next_entry` variable is set this is only being used once, grub2 will
unset it after reading it for the first time. This is helpful to define the menu entry
to reboot to without having to make any permanent config change.

Use `grub2-editenv` command line utility to define desired values.

For instance use the following command to reboot to recovery system only once:

```bash
> grub2-editenv /oem/grubenv set next_entry=recovery
```

Or to set the default entry to `fallback` system:

```bash
> grub2-editenv /oem/grubenv set default=fallback
```

These examples make of the `COS_OEM` device, however it could use any device
detected by grub2 that includes the file `/grubenv`. First match wins.

