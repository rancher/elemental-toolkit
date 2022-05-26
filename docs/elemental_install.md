## elemental install

elemental installer

```
elemental install DEVICE [flags]
```

### Options

```
  -c, --cloud-init string                Cloud-init config file
      --cosign                           Enable cosign verification (requires images with signatures)
      --cosign-key string                Sets the URL of the public key to be used by cosign validation
      --eject-cd                         Try to eject the cd on reboot, only valid if booting from iso
      --firmware string                  Firmware to install for ('esp' or 'bios') (default "efi")
      --force                            Force install
  -h, --help                             help for install
  -i, --iso string                       Performs an installation from the ISO url
      --local                            Use an image from local cache
      --no-format                        Don’t format disks. It is implied that COS_STATE, COS_RECOVERY, COS_PERSISTENT, COS_OEM are already existing
      --part-table string                Partition table type to use (default "gpt")
      --poweroff                         Shutdown the system after install
      --reboot                           Reboot the system after install
      --recovery-system.uri string       Sets the recovery image source and its type (e.g. 'docker:registry.org/image:tag')
  -x, --squash-compression stringArray   cmd options for compression to pass to mksquashfs. Full cmd including --comp as the whole values will be passed to mksquashfs. For a full list of options please check mksquashfs manual. (default value: '-comp xz -Xbcj ARCH')
      --strict                           Enable strict check of hooks (They need to exit with 0)
      --system.uri string                Sets the system image source and its type (e.g. 'docker:registry.org/image:tag')
      --tty string                       Add named tty to grub
      --verify                           Enable mtree checksum verification (requires images manifests generated with mtree separately)
```

### Options inherited from parent commands

```
      --config-dir string   set config dir (default is /etc/elemental) (default "/etc/elemental")
      --debug               enable debug output
      --logfile string      set logfile
      --quiet               do not output to stdout
```

### SEE ALSO

* [elemental](elemental.md)	 - elemental

