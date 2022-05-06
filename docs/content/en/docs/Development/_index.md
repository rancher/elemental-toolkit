
---
title: "Development"
linkTitle: "Development"
weight: 8
date: 2017-01-05
description: >
  How to build cOS?
---

Welcome!

The cOS (containerized OS) distribution is entirely built over GitHub. You can check the pipelines in the `.github` folder to see how the process looks like.

## Repository layout

- `packages`: contain packages definition for luet
- `values`: interpolation files, needed only for multi-arch and flavor-specific build
- `assets`: static files needed by the iso generation process
- `packer`: Packer templates
- `tests`: cOS test suites
- `manifest.yaml`: Is the manifest needed used to generate the ISO and additional packages to build

## Forking and test on your own

By forking the `cOS-toolkit` repository, you already have the Github Action workflow configured to start building and pushing your own `cOS` fork.

The only changes required to keep in mind for pushing images:
- set `DOCKER_PASSWORD` and `DOCKER_USERNAME` as Github secrets, which are needed to push the resulting container images from the pipeline. 
- Tweak or set the `Makefile`'s `REPO_CACHE` and `FINAL_REPO` accordingly. Those are used respectively for an image used for cache, and for the final image reference.

Those are not required for building - you can disable image push (`--push`) from the `Makefile` or just by specifying e.g. `BUILD_ARGS=--pull` when calling the `make` targets.

## Building locally

cOS has a container image which can be used to build cOS locally in order to generate the cOS packages and the cOS iso from your checkout.

From your git folder:

```bash
$> docker build -t cos-builder .
$> docker run --privileged=true --rm -v /var/run/docker.sock:/var/run/docker.sock -v $PWD:/cOS cos-builder
```

or use the `.envrc` file:

```bash
$> source .envrc
$> cos-build
```

### Build all packages locally

Building locally has a [set of dependencies](dependencies.md) that
should be satisfied.

Then you can run
```
# make build
```
as root


To clean from previous runs, run `make clean`.

_Note_: The makefile uses [`yq` and `jq`](dev.md#yq-and-jq) to
retrieve the packages to build from the iso specfile.

If you don't have `jq` and `yq` installed, you must pass by the packages manually with `PACKAGES` (e.g. `PACKAGES="system/cos live/systemd-boot live/boot live/syslinux"`).

You might want to build packages running as `root` or `sudo -E` if you intend to preserve file permissions in the resulting packages (mainly for `xattrs`, and so on).

### Build ISO

If using SLES or openSUSE, first install the required deps:

```
# zypper in -y squashfs xorriso dosfstools
```

and then, simply run

```
# make local-iso
```

### Testing ISO changes

To test changes against a specific set of packages, you can for example:

```
# make PACKAGES="toolchain/yq"  build local-iso
```

root is required because we want to keep permissions on the output packages (not really required for experimenting).

### Run with qemu

After you have the iso locally, run

```

$> QEMU=qemu-system-x86_64 make run-qemu

```

This will create a disk image at `.qemu/drive.img` and boot from the ISO.

>
> If the image already exists, it will NOT be overwritten.
>
> You need to run an explicit `make clean_run` to wipe the image and
> start over.
>

#### Installing

With a fresh `drive.img`, `make run-qemu` will boot from ISO. You can then log in as `root` with password `cos` and install cOS on
the disk image with:

```
# elemental install /dev/sda
```

#### Running

After a successful installation of cOS on `drive.img`, you can boot
the resulting sytem with

```

$> QEMU_ARGS="-boot c" make run-qemu

```


### Run tests

Requires: Virtualbox or libvirt, vagrant, packer

We have a test suite which runs over SSH.

To create the vagrant image:

```

$> PACKER_ARGS="-var='feature=vagrant' -only virtualbox-iso.cos" make packer

```

To run the tests:

```

$> make test

```
