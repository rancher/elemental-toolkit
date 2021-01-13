## Requirements

- Luet installed locally
- Docker/or img for building packages locally
- squashfs/xorriso/dosfstools for building ISO

## Build all packages locally

```
make build-full
```

## Build ISO

If using opensuse, first install the required deps:

```
zypper in -y squashfs xorriso dosfstools
```

and then, simply run

```
make local-iso
```