## elemental reset

Reset OS

```
elemental reset [flags]
```

### Options

```
      --cosign              Enable cosign verification (requires images with signatures)
      --cosign-key string   Sets the URL of the public key to be used by cosign validation
  -h, --help                help for reset
      --poweroff            Shutdown the system after install
      --reboot              Reboot the system after install
      --reset-oem           Clear OEM partitions
      --reset-persistent    Clear persistent partitions
      --strict              Enable strict check of hooks (They need to exit with 0)
      --system.uri string   Sets the system image source and its type (e.g. 'docker:registry.org/image:tag')
      --tty                 Add named tty to grub
      --verify              Enable mtree checksum verification (requires images manifests generated with mtree separately)
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

