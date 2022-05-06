---
title: "Creating derivatives"
linkTitle: "Creating derivatives"
weight: 3
date: 2017-01-05
description: >
  This document summarize references to create derivatives with `cos-toolkit` by using the `luet` toolchain.

---

`cos-toolkit` is a manifest to share a common abstract layer between derivatives inheriting the same featureset. 

`cos` is a [Luet tree](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/#specfiles) and derivatives can be expressed as Luet trees as well that inherit part of the compilation specs from `cos`.

Those trees are then post-processed and converted to Dockerfiles when building packages, that in turn are used to build docker images and final artefacts.

## High level workflow

The building workflow can be resumed in the following steps:

- Build packages from container images. This step generates build metadata (`luet build` / `docker build` / `buildah` ..)
- Add repository metadata and create a repository from the build phase (`luet create-repo`)
- (otherwise, optionally) publish the repository and the artefacts along (`luet create-repo --push-images`)

While on the client side, the upgrade workflow is:
- `luet install` (when upgrading from release channels) latest cos on a pristine image file
- or `luet util unpack` (when upgrading from specific docker images)

*Note*: The manual build steps are not stable and will likely change until [we build a single CLI](https://github.com/rancher-sandbox/cOS-toolkit/issues/108) to encompass the `cos-toolkit` components, rather use `source .envrc && cos-build` for the moment being while iterating locally.

## Single image OS

Derivatives are composed by a combination of specs to form a final package that is consumed as a single image OS.

The container image during installation and upgrade, is converted to an image file with a backing ext2 fs. 

Packages in luet have `runtime` and `buildtime` specifications into `definition.yaml` and `build.yaml` respectively, and in the buildtime we set:

```yaml
requires:
- category: "system"
  name: "cos"
  version: ">=0"
- category: "app"
  name: "sampleOSService"
  version: ">=0"

```

This instruct `luet` to compose a new image from the results of the compilation of the specified packages, without any version constraints, and use it to run any `steps` and `prelude` on top of it.

We are interested in the dependencies final images, and not the containers used to build them, so we enable `requires_final_images`:

```
requires_final_images: true
```

We later run arbitrary steps to tweak the image:

```yaml
steps:
- ...
```

And we instruct luet to compose the final artifact as a `squash` of the resulting container image, composed of all the files:

```yaml
unpack: true
```

A detailed explaination of all the keywords available [is in the luet docs](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/#keywords) along with the [supported build strategies](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/#building-strategies).

We exclude then a bunch of file that we don't want to be in the final package (regexp supported):

```yaml
excludes:
- ..
```

## Templating

The package `build` definition supports [templating](https://luet-lab.github.io/docs/docs/concepts/packages/templates/), and global interpolation of build files with multiple values files.

Values file can be specified during build time in luet with the ```--values``` flag (also multiple files are allowed) and, if you are familiar with `helm` it using the same engine under the hood, so all the functions are available as well.

`cos-toolkit` itself uses [default values files](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/values) for every supported distributions.

Templates uses cases are for: resharing common pieces between flavors, building for different platforms and architectures, ...


## Build ISO

To build an iso for a derivative image `elemental build-iso` command can be used:

```bash
elemental build-iso -n $NAME $IMAGE
```

Where `$NAME` is the name of the ISO and `$IMAGE` is the reference to the container image we are building the ISO for. See also [building ISOs](../../creating-derivatives/build_iso)
