## elemental upgrade-recovery

Upgrade the Recovery system

```
elemental upgrade-recovery [flags]
```

### Options

```
  -h, --help                             help for upgrade-recovery
      --poweroff                         Shutdown the system after install
      --reboot                           Reboot the system after install
      --recovery-system.uri string       Sets the recovery image source and its type (e.g. 'docker:registry.org/image:tag')
      --snapshot-labels stringToString   Add labels to the to the system (ex. --snapshot-labels my-label=foo,my-other-label=bar) (default [])
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
