## elemental install

Elemental installer

```
elemental install DEVICE [flags]
```

### Options

```
  -c, --cloud-init strings               Cloud-init config files
      --cloud-init-paths strings         Cloud-init config files to run during install
      --cosign                           Enable cosign verification (requires images with signatures)
      --cosign-key string                Sets the URL of the public key to be used by cosign validation
      --disable-boot-entry               Dont create an EFI entry for the system install.
      --eject-cd                         Try to eject the cd on reboot, only valid if booting from iso
      --force                            Force install
  -h, --help                             help for install
  -i, --iso string                       Performs an installation from the ISO url
      --local                            Use an image from local cache
      --no-format                        Don’t format disks. It is implied that COS_STATE, COS_RECOVERY, COS_PERSISTENT, COS_OEM are already existing
      --platform string                  Platform to build the image for (default "linux/amd64")
      --poweroff                         Shutdown the system after install
      --reboot                           Reboot the system after install
      --recovery-system.uri string       Sets the recovery image source and its type (e.g. 'docker:registry.org/image:tag')
      --snapshot-labels stringToString   Add labels to the to the system (ex. --snapshot-labels my-label=foo,my-other-label=bar) (default [])
      --snapshotter.type string          Sets the snapshotter type to install (default "loopdevice")
  -x, --squash-compression stringArray   cmd options for compression to pass to mksquashfs. Full cmd including --comp as the whole values will be passed to mksquashfs. For a full list of options please check mksquashfs manual. (default value: '-comp xz -Xbcj ARCH')
      --squash-no-compression            Disable squashfs compression. Overrides any values on squash-compression
      --strict                           Enable strict check of hooks (They need to exit with 0)
      --system string                    Sets the system image source and its type (e.g. 'docker:registry.org/image:tag')
      --tls-verify                       Require HTTPS and verify certificates of registries (default: true) (default true)
      --verify                           Enable mtree checksum verification (requires images manifests generated with mtree separately)
```

### Options inherited from parent commands

```
      --config-dir string   Set config dir
      --debug               Enable debug output
      --logfile string      Set logfile
      --quiet               Do not output to stdout
```

### SEE ALSO

* [elemental](elemental.md)	 - Elemental

