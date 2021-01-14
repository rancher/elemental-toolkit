## Requirements

- Luet installed locally (You can install it with `curl https://get.mocaccino.org/luet/get_luet_root.sh | sudo sh` )
- Luet-extensions (It comes pre-installed with the above command, or add the [official luet-repo](https://github.com/Luet-lab/luet-repo) and install it with `luet install -y system/luet-extensions)
- Docker/or img for building packages locally
- squashfs/xorriso/dosfstools for building ISO
- yq (`luet install -y repository/mocaccino-extra-stable && luet install -y utils/yq`)

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

## Testing ISO changes

To test changes against a specific set of packages, you can for example:

```bash

SUDO="sudo -E" PACKAGES="live/init kernel/default-minimal" make rebuild local-iso

```

SUDO is used because we want to keep permissions on the output packages (not really required for experimenting).
