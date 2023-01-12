# From scratch image example

Build:

```bash
$ docker build -t scratch .
```

Push to some registry and upgrade your `Elemental` vm to it:

```bash
cos-vm $ elemental upgrade --system.uri $IMAGE
```
