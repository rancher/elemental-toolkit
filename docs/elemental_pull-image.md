## elemental pull-image

Pull remote image to local file

```
elemental pull-image IMAGE DESTINATION [flags]
```

### Options

```
      --auth-identity-token string   Authentication identity token
      --auth-password string         Password to authenticate to registry
      --auth-registry-token string   Authentication registry token
      --auth-server-address string   Authentication server address
      --auth-type string             Auth type
      --auth-username string         Username to authenticate to registry/notary
  -h, --help                         help for pull-image
      --local                        Use an image from local cache
      --plugin stringArray           A list of runtime plugins to load. Can be repeated to add more than one plugin
      --verify                       Verify signed images to notary before to pull
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

