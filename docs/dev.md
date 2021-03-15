Welcome!

The cOS Distribution is entirely built over GitHub. You can check the pipelines in the `.github` folder to see how the process looks like.

## Repository layout

- `packages`: contain packages definition for luet
- `iso`: yaml spec files for development iso generation
- `values`: interpolation files, needed only for multi-arch build
- `assets`: static files needed by the iso generation process

## Forking and test on your own

By forking the `cOS` repository, you already have the Github Action workflow configured to start building and pushing your own `cOS` fork.
The only changes required to keep in mind for pushing images:
- set `DOCKER_PASSWORD` and `DOCKER_USERNAME` as Github secrets, which are needed to push the resulting docker images from the pipeline. 
- Tweak or set the `Makefile`'s `REPO_CACHE` and `FINAL_REPO` accordingly. Those are used respectively for an image used for cache, and for the final image reference.

Those are not required for building - you can disable image push (`--push`) from the `Makefile` or just by specifying e.g. `BUILD_ARGS=--pull` when calling the `make` targets.

## Building locally

### With docker

cOS has a docker image which can be used to build cOS locally.

From your local checkout of cOS:

```bash
$> docker build -t cos-builder .
$> docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $PWD:/cOS cos-builder
```

### Requirements

- Luet installed locally (You can install it with `curl https://get.mocaccino.org/luet/get_luet_root.sh | sudo sh` )

#### Building packages

- Docker/or img for building packages locally

#### Building ISO

- Luet-devkit (If you manually installed luet add the [official luet-repo](https://github.com/Luet-lab/luet-repo) first. Install it with `luet install -y system/luet-devkit`)
- squashfs/xorriso/dosfstools for building ISO ( from your OS )
- yq and jq (`luet install -y repository/mocaccino-extra-stable && luet install -y utils/yq utils/yq`)

### Build all packages locally

```
make build
```

To clean from previous runs, run `make clean`.

You might want to build packages running as `root` or `sudo -E` if you intend to preserve file permissions in the resulting packages (mainly for `xattrs`, and so on).

### Build ISO

If using opensuse, first install the required deps:

```
zypper in -y squashfs xorriso dosfstools
```

and then, simply run

```
make local-iso
```

### Testing ISO changes

To test changes against a specific set of packages, you can for example:

```bash

make PACKAGES="live/init"  build local-iso

```

root is required because we want to keep permissions on the output packages (not really required for experimenting).

Note: Remind to bump `definition.yaml` files where necessary, otherwise it would generate packages from existing images
