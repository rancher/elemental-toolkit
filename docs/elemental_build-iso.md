## elemental build-iso

Build bootable installation media ISOs

### Synopsis

Build bootable installation media ISOs

SOURCE - should be provided as uri in following format <sourceType>:<sourceName>
    * <sourceType> - might be ["dir", "file", "oci", "docker", "channel"], as default is "docker"
    * <sourceName> - is path to file or directory, image name with tag version or channel name

```
elemental build-iso SOURCE [flags]
```

### Options

```
      --bootloader-in-rootfs             Fetch ISO bootloader binaries from the rootfs
      --cosign                           Enable cosign verification (requires images with signatures)
      --cosign-key string                Sets the URL of the public key to be used by cosign validation
      --date                             Adds a date suffix into the generated ISO file
  -h, --help                             help for build-iso
      --label string                     Label of the ISO volume
      --local                            Use an image from local cache
  -n, --name string                      Basename of the generated ISO file
  -o, --output string                    Output directory (defaults to current directory)
      --overlay-iso string               Path of the overlayed iso data
      --overlay-rootfs string            Path of the overlayed rootfs data
      --overlay-uefi string              Path of the overlayed uefi data
      --platform string                  Platform to build the image for (default "linux/amd64")
  -x, --squash-compression stringArray   cmd options for compression to pass to mksquashfs. Full cmd including --comp as the whole values will be passed to mksquashfs. For a full list of options please check mksquashfs manual. (default value: '-comp xz -Xbcj ARCH')
      --squash-no-compression            Disable squashfs compression. Overrides any values on squash-compression
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

