## elemental convert-disk

converts between a raw disk and a cloud operator disk image (azure,gce)

```
elemental convert-disk RAW_DISK [flags]
```

### Options

```
  -h, --help          help for convert-disk
      --keep-source   Keep the source image, otherwise it will delete it once transformed.
  -t, --type string   Type of image to create (default "azure")
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

