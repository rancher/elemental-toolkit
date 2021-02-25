## Requirements

- Luet installed locally (You can install it with `curl https://get.mocaccino.org/luet/get_luet_root.sh | sudo sh` )

### Building packages

- Docker/or img for building packages locally

### Building ISO

- Luet-devkit (If you manually installed luet add the [official luet-repo](https://github.com/Luet-lab/luet-repo) first. Install it with `luet install -y system/luet-devkit`)
- squashfs/xorriso/dosfstools for building ISO ( from your OS )
- yq and jq (`luet install -y repository/mocaccino-extra-stable && luet install -y utils/yq utils/yq`)

## Repository layout

- `packages`: contain packages definition for luet
- `iso`: yaml spec files for development iso generation
- `values`: interpolation files, needed only for multi-arch build
- `assets`: static files needed by the iso generation process

## Build all packages locally

```
make build
```

To clean from previous runs, run `make clean`.

You might want to build packages running as `root` or `sudo -E` if you intend to preserve file permissions in the resulting packages (mainly for `xattrs`, and so on).

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

make PACKAGES="live/init"  build local-iso

```

root is required because we want to keep permissions on the output packages (not really required for experimenting).

Note: Remind to bump `definition.yaml` files where necessary, otherwise it would generate packages from existing images
