# From scratch image example

Build:

```bash
$ docker build -t scratch .
```

Push to some registry and upgrade your `cOS` vm to it:

```bash
cos-vm $ cos-upgrade --docker-image --no-verify $IMAGE
```