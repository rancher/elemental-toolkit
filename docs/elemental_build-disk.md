## elemental build-disk

Build a raw recovery image

```
elemental build-disk [flags]
```

### Options

```
  -a, --arch string             Arch to build the image for (default "x86_64")
      --cosign                  Enable cosign verification (requires images with signatures)
      --cosign-key string       Sets the URL of the public key to be used by cosign validation
  -h, --help                    help for build-disk
      --oem_label string        Oem partition label (default "COS_OEM")
  -o, --output string           Output file (Extension auto changes based of the image type) (default "disk.raw")
      --recovery_label string   Recovery partition label (default "COS_RECOVERY")
  -t, --type string             Type of image to create (default "raw")
```

### Options inherited from parent commands

```
      --config-dir string   Set config dir (default is /etc/elemental) (default "/etc/elemental")
      --debug               Enable debug output
      --logfile string      Set logfile
      --quiet               Do not output to stdout
```

### SEE ALSO

* [elemental](elemental.md)	 - Elemental

