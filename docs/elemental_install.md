## elemental install

elemental installer

```
elemental install DEVICE [flags]
```

### Options

```
  -c, --cloud-init string         Cloud-init config file
      --cosign                    Enable cosign verification (requires images with signatures)
      --cosign-key string         Sets the URL of the public key to be used by cosign validation
      --directory string          Use directory as source to install from
  -d, --docker-image string       Install a specified container image
      --eject-cd                  Try to eject the cd on reboot, only valid if booting from iso
      --force                     Force install
      --force-efi                 Forces an EFI installation
      --force-gpt                 Forces a GPT partition table
  -h, --help                      help for install
  -i, --iso string                Performs an installation from the ISO url
      --no-format                 Don’t format disks. It is implied that COS_STATE, COS_RECOVERY, COS_PERSISTENT, COS_OEM are already existing
      --no-verify                 Disable mtree checksum verification (requires images manifests generated with mtree separately)
  -p, --partition-layout string   Partitioning layout file
      --poweroff                  Shutdown the system after install
      --reboot                    Reboot the system after install
      --strict                    Enable strict check of hooks (They need to exit with 0)
      --tty                       Add named tty to grub
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

