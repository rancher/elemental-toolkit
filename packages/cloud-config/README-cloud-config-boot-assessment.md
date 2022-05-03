In order for a derivative to enable boot assessment, it is only required to have the cloud-config file installed.

The boot assessment process works in the following way:

- After an upgrade, sets a GRUB env sentinel variable indicating that we did run an upgrade
- At the first boot, if we an upgrade was attempted, we set another sentinel variable, which indicates a booting attempt
  - If we boot fine, sentinels are removed
  - If we get back again at the GRUB menu, a failure must have occurred and we select the fallback entry, creating also
    sentinels files and a specific cmdline option indicating we failed booting after an upgrade

The grub sentinel files are in the COS_STATE partition, and get installed automatically after reset, install and upgrade:

- `/boot_assessment` - contains the GRUB env sentinel variables
- `/grub_boot_assessment` - contains the GRUB logic for booting into fallback

Note: The extra GRUB logic is installed and sourced into `/grubcustom` inside `COS_STATE`.

When a boot failure is detected and the fallback is automatically selected, it will be created a `/run/cos/upgrade_failure` sentinel file during the boot process, which is accessible under the `boot` cloud-init stage.

To enable boot assessment always, besides upgrades, the package `cloud-config/boot-assessment-always` needs to be installed as well.

## Manually enabling boot assessment

To manually trigger the boot assessment for the next boot, you can run the following in a active/passive booted system:

```
sudo mount -o rw,remount /run/initramfs/cos-state
grub2-editenv /run/initramfs/cos-state/boot_assessment set enable_boot_assessment=yes
sudo mount -o ro,remount /run/initramfs/cos-state
```

## Maintenance

If the active partition fails, the boot assessment process will bring you back to the fallback partition. This is also the case if the dracut shell is displayed while dropping in emergency mode.

If you are planning to do manual intervention and want to hook up with that console, you can disable the boot assessment process by running in the fallback partition:

```
sudo mount -o rw,remount /run/initramfs/cos-state
grub2-editenv /run/initramfs/cos-state/boot_assessment set enable_boot_assessment=
grub2-editenv /run/initramfs/cos-state/boot_assessment set boot_assessment_tentative=
sudo mount -o ro,remount /run/initramfs/cos-state
```