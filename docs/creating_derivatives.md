# Creating derivatives

This document summarize references to create Immutable derivatives with `cos-toolkit`.

`cos-toolkit` is a manifest to share a common abstract layer between derivatives inheriting the same [featureset](/docs/derivatives_featureset.md). 

`cos` is a [Luet tree](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/#specfiles) and derivatives are Luet trees as well that inherit part of the compilation specs from `cos`.

Those trees are then post-processed and converted to Dockerfiles when building packages, that in turn are used to build docker images and final artefacts.

<!-- TOC -->

- [Creating derivatives](#creating-derivatives)
    - [High level workflow](#high-level-workflow)
    - [Example](#example)
    - [Single image OS](#single-image-os)
        - [Building](#building)
    - [Additional packages](#additional-packages)
    - [Templating](#templating)
    - [Upgrades](#upgrades)
    - [OEM Customizations](#oem-customizations)
    - [Building ISOs, Vagrant Boxes, OVA](#building-isos-vagrant-boxes-ova)

<!-- /TOC -->

## High level workflow

The building workflow can be resumed in the following steps:

- Build packages from container images. This step generates build metadata (`luet build`)
- Add repository metadata and create a repository from the build phase (`luet create-repo`)
- (otherwise, optionally) publish the repository and the artefacts along (`luet create-repo --push-images`)

While on the client side, the upgrade workflow is:
- `luet install` (when upgrading from release channels) latest cos on a pristine image file
- or `luet util unpack` (when upgrading from specific docker images)

*Note*: The manual build steps are not stable and will likely change until [we build a single CLI](https://github.com/rancher-sandbox/cOS-toolkit/issues/108) to encompass the `cos-toolkit` components, rather use `source .envrc && cos-build` for the moment being while iterating locally.

## Example

[The sample repository](https://github.com/rancher-sandbox/cos-toolkit-sample-repo) has the following layout:

```
├── Dockerfile
├── .envrc
├── .github
│   └── workflows
│       ├── build.yaml
│       └── test.yaml
├── .gitignore
├── iso.yaml
├── LICENSE
├── .luet.yaml
├── Makefile
├── packages
│   ├── sampleOS
│   │   ├── 02_upgrades.yaml
│   │   ├── 03_branding.yaml
│   │   ├── 04_accounting.yaml
│   │   ├── build.yaml
│   │   ├── definition.yaml
│   │   └── setup.yaml
│   └── sampleOSService
│       ├── 10_sampleOSService.yaml
│       ├── build.yaml
│       ├── definition.yaml
│       └── main.go
└── README.md
```

In the detail:
- the `packages` directory is the sample Luet tree that contains the [package definitions](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/#build-specs) [1] which composes the derivative. 
For an overview of the package syntax and build process, see the [official luet documentation](https://luet-lab.github.io/docs/docs/concepts/packages/)
- `.luet.yaml` contains a configuration file for `luet` pointing to the `cos` repositories, used to fetch packages required in order to build the iso [2] **and** to fetch definitions from [3].
- `Makefile` and `.envrc` are just wrappers around `luet build` and `luet create-repo`
- `iso.yaml` a YAML file that describes what packages to embed in the final ISO

*Note*: There is nothing special in the layout, and neither the `packages` folder naming is special. By convention we have chosen to put the compilation specs in the `packages` folder, the `Makefile` is just calling `luet` with a set of default parameters according to this setup.

The `.envrc` is provided as an example to automatize the build process: it will build a docker image with the required dependencies, check the [development docs](/docs/dev.md) about the local requirements if you plan to build outside of docker.

**[1]** _In the sample above we just declare two packages: `sampleOS` and `sampleOSService`. Their metadata are respectively in `packages/sampleOS/definition.yaml` and `packages/sampleOSService/definition.yaml`_

**[2]** _We consume `live/systemd-boot` and `live/syslinux` from `cos` instead of building them from the sample repository_

**[3]** _see also [using git submodules](https://github.com/rancher-sandbox/epinio-appliance-demo-sample#main-difference-with-cos-toolkit-sample-repo) instead_

## Single image OS

Derivatives are composed by a combination of specs to form a final package that is consumed as a single image OS.

The container image during installation and upgrade, is converted to an image file with a backing ext2 fs. 

In the sample repository [we have defined `system/sampleOS`](https://github.com/rancher-sandbox/cos-toolkit-sample-repo/blob/master/packages/sampleOS/definition.yaml) as our package, that will later on will be converted to image.

Packages in luet have `runtime` and `buildtime` specifications into `definition.yaml` and `build.yaml` respectively, and in the buildtime we set:

```yaml
join:
- category: "system"
  name: "cos"
  version: ">=0"
- category: "app"
  name: "sampleOSService"
  version: ">=0"
```

This instruct `luet` to compose a new image from the results of the compilation of the specified packages, without any version constraints, and use it to run any `steps` and `prelude` on top of it.

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

__Note__: In the [EpinioOS sample](https://github.com/rancher-sandbox/epinio-appliance-demo-sample/blob/19c530ea53ad577e60adbae1d419781fcea808f5/packages/epinioOS/build.yaml#L1), we use `requires` instead of `join`:

```yaml
requires:
- category: "system"
  name: "cos"
  version: ">=0"
- name: "k3s"
  category: "app"
  version: ">=0"
- name: "policy"
  category: "selinux"
  version: ">=0"
```

The difference is that with `requires` we use the _building_ container that was used to build the packages instead of creating a new image from their results: we are not consuming their artifacts in this case, but the environment used to build them. See also [the luet docs](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/#package-source-image) for more details. 

### Building

Refering to the `sampleOS` example, we set the [Makefile](https://github.com/rancher-sandbox/cos-toolkit-sample-repo/blob/8ed369c6ca76f1fc69e49d8001c689c8d0371d30/Makefile#L13) accordingly to compile the system package.

With luet installed locally and docker running, in your git checkout you can build it also by running `luet build --tree packages system/sampleOS`. This will produce an artifact of `system/sampleOS`. Similary, we could also build separately the sample application with `luet build --tree packages app/sampleOSService`.

The build process by default results in a `build` folder containing the package and the compilation metadata in order to generate a repository.

_Note on reproducibility_: See [the difference between our two samples repositories](https://github.com/rancher-sandbox/epinio-appliance-demo-sample#main-difference-with-cos-toolkit-sample-repo) for an explanation of what are the implications of using a `.luet.yaml` file for building instead of a git submodule.

## Additional packages

In our sample repo we have split the logic of a separate application in `app/sampleOSService`. 

`sampleOSService` is just an HTTP server that we would like to have permanently in the system and on boot.

Thus we define it as a dependency in the `system/sampleOS`'s `requires` section:

```yaml
requires:
...
- category: "app"
  name: "sampleOSService"
  version: ">=0"
```

_Note_ If you are wondering about copying just single files, there is [an upstream open issue](https://github.com/mudler/luet/issues/190) about it.

In this way, when building our `sampleOS` package, `luet` will automatically apply the compilation spec of our package on top.

## Templating

The package `build` definition supports [templating](https://luet-lab.github.io/docs/docs/concepts/packages/templates/), and global interpolation of build files with multiple values files.

Values file can be specified during build time in luet with the ```--values``` flag (also multiple files are allowed) and, if you are familiar with `helm` it using the same engine under the hood, so all the functions are available as well.

`cos-toolkit` itself uses [default values files](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/values) for every supported distributions.

For a more complex example involving values file, [see the epinio appliance example](https://github.com/rancher-sandbox/epinio-appliance-demo-sample).

Templates uses cases are for: resharing common pieces between flavors, building for different platforms and architectures, ...

## Upgrades

In order for the derivative to upgrade, it needs to be configured in order to download upgrades from a source.

By default, `cos` derivatives if not specified will point to latest `cos-toolkit`. To override, you need to or overwrite the content of `/system/oem/02_upgrades.yaml` or supply an additional one, e.g. `/system/oem/03_upgrades.yaml` in the final image, see [an example here](https://github.com/rancher-sandbox/epinio-appliance-demo-sample/blob/master/packages/epinioOS/02_upgrades.yaml).

The configuration need to point to a specific docker image or an upgrade channel, [a complete example and documentation is here](https://github.com/rancher-sandbox/epinio-appliance-demo-sample#images).

## OEM Customizations

There are several way to customize a cos-toolkit derivative:

- declaratively in runtime with cloud-config file (by overriding, or extending)
- stateful, via build definition when running `luet build`.

For runtime persistence configuration, the only supported way is with cloud-config files, [see the relevant docs](https://github.com/rancher-sandbox/cOS-toolkit/blob/master/docs/derivatives_featureset.md#persistent-changes).

A derivative automatically loads and executes cloud-config files which are hooking into system stages.

In this way the cloud-config mechanism works also as an emitter event pattern - running services or programs can emit new custom `stages` in runtime by running `cos-setup stage_name`.

For an extensive list of the default OEM files that can be reused or replaced [see here](https://github.com/rancher-sandbox/cOS-toolkit/blob/master/docs/derivatives_featureset.md#oem-customizations).

## Customizing GRUB boot cmdline

Each bootable image have a default boot arguments which are defined in `/etc/cos/bootargs.cfg`. This file is used by GRUB to parse the cmdline used to boot the image. 

For example:
```
set kernel=/boot/vmlinuz
if [ -n "$recoverylabel" ]; then
    # Boot arguments when the image is used as recovery
    set kernelcmd="console=tty1 root=live:CDLABEL=$recoverylabel rd.live.dir=/ rd.live.squashimg=$img panic=5"
else
    # Boot arguments when the image is used as active/passive
    set kernelcmd="console=tty1 root=LABEL=$label iso-scan/filename=$img panic=5 security=selinux selinux=1"
fi

set initramfs=/boot/initrd
```

You can tweak that file to suit your needs if you need to specify persistent boot arguments.

## Separate image recovery

A separate image recovery can be used during upgrades. 

To set a default recovery image or a package, set `RECOVERY_IMAGE` into /etc/cos-upgrade-image. It allows to override the default image/package used during upgrades.

To make an ISO with a separate recovery image as squashfs, you can either use the default from `cOS`, by adding it in the iso yaml file:


```yaml
packages:
  rootfs:
  ..
  uefi:
  ..
  isoimage:
  ...
  - recovery/cos-img
```

The installer will detect the squashfs file in the iso, and will use it when installing the system. You can customize the recovery image as well by providing your own: see the `recovery/cos-img` package definition as a reference.

## Building ISOs, Vagrant Boxes, OVA

In order to build an iso at the moment of writing, we first rely on [luet-makeiso](https://github.com/mudler/luet-makeiso). It accepts a YAML file denoting the packages to bundle in an ISO and a list of luet repositories where to download the packages from.

A sample can be found [here](https://github.com/rancher-sandbox/cos-toolkit-sample-repo/blob/master/iso.yaml). 

To build an iso from a local repository (the build process, automatically produces a repository in `build` in the local checkout):

```bash
luet-makeiso ./iso.yaml --local build
```

Where `iso.yaml` is the iso specification file, and `--local build` is an optional argument to use also the local repository in the build process.

We are then free to refer to packages in the tree in the `iso.yaml` file.

For Vagrant Boxes, OVA and QEMU images at the moment of writing we are relying on [packer templates](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packer). 