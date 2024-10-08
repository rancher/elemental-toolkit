## elemental reset

Reset OS

```
elemental reset [flags]
```

### Options

```
  -c, --cloud-init strings               Cloud-init config files
      --cloud-init-paths strings         Cloud-init config files to run during reset
      --cosign                           Enable cosign verification (requires images with signatures)
      --cosign-key string                Sets the URL of the public key to be used by cosign validation
      --disable-boot-entry               Dont create an EFI entry for the system install.
  -h, --help                             help for reset
      --poweroff                         Shutdown the system after install
      --reboot                           Reboot the system after install
      --reset-oem                        Clear OEM partitions
      --reset-persistent                 Clear persistent partitions
      --snapshot-labels stringToString   Add labels to the to the system (ex. --snapshot-labels my-label=foo,my-other-label=bar) (default [])
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

