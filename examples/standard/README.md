# From official images example

Build:

```bash
$ docker build -t $IMAGE .
```

Push to some registry and upgrade your `cOS` vm to it:

```bash
cos-vm $ elemental upgrade --system.uri $IMAGE
```
