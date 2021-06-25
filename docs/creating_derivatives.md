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
    - [Customizing GRUB boot cmdline](#customizing-grub-boot-cmdline)
    - [Separate image recovery](#separate-image-recovery)
    - [Building ISOs, Vagrant Boxes, OVA](#building-isos-vagrant-boxes-ova)
    - [Known issues](#known-issues)
        - [Building SELinux fails](#building-selinux-fails)
        - [Multi-stage copy build fails](#multi-stage-copy-build-fails)

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

## Known issues

When building cOS or a cOS derivative, you could face different issues, this section provides a description of the most known ones, and way to workaround them.

### Building SELinux fails

`cOS` by default has SELinux enabled in permissive mode. If you are building parts of cOS or cOS itself from scratch, you might encounter issues while building the SELinux module, like so:

```
Step 12/13 : RUN checkmodule -M -m -o cOS.mod cOS.te && semodule_package -o cOS.pp -m cOS.mod
  ---> Using cache
 ---> 1be520969ead
Step 13/13 : RUN semodule -i cOS.pp
  ---> Running in c5bfa5ae92e2
 libsemanage.semanage_commit_sandbox: Error while renaming /var/lib/selinux/targeted/active to /var/lib/selinux/targeted/previous. (Invalid cross-device link).
semodule:  Failed!
 The command '/bin/sh -c semodule -i cOS.pp' returned a non-zero code: 1
 Error: while resolving join images: failed building join image: Failed compiling system/selinux-policies-0.0.6+3: failed building package image: Could not push image: raccos/sampleos:ffc8618ecbfbffc11cc3bca301cc49867eb7dccb623f951dd92caa10ced29b68 selinux-policies-system-0.0.6+3.dockerfile: Could not build image: raccos/sampleos:ffc8618ecbfbffc11cc3bca301cc49867eb7dccb623f951dd92caa10ced29b68 selinux-policies-system-0.0.6+3.dockerfile: Failed running command: : exit status 1
 Bailing out
make: *** [Makefile:45: build] Error 1
```

The issue is possibly caused by https://github.com/docker/for-linux/issues/480 . A workaround is to switch the storage driver of Docker. Check if your storage driver is overlay2, and switch it to `devicemapper`

### Multi-stage copy build fails

While processing images with several stage copy, you could face the following:


```
 🐋  Building image raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d done
 📦  8/8 system/cos-0.5.3+1 ⤑ 🔨  build system/selinux-policies-0.0.6+3 ✅  Done
 🚀  All dependencies are satisfied, building package requested by the user system/cos-0.5.3+1
 📦  system/cos-0.5.3+1  Using image:  raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d
 📦  system/cos-0.5.3+1 🐋  Generating 'builder' image from raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d as raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed with prelude steps
🚧  warning Failed to download 'raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed'. Will keep going and build the image unless you use --fatal
🚧  warning Failed pulling image: Error response from daemon: manifest for raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed not found: manifest unknown: manifest unknown
: exit status 1
 🐋  Building image raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed
 Sending build context to Docker daemon  9.728kB
 Step 1/10 : FROM raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d
  ---> f1122e79b17e
Step 2/10 : COPY . /luetbuild
  ---> 4ff3e202951b
 Step 3/10 : WORKDIR /luetbuild
  ---> Running in 7ec571b96c6f
 Removing intermediate container 7ec571b96c6f
  ---> 9e05366f830a
Step 4/10 : ENV PACKAGE_NAME=cos
  ---> Running in 30297dbd21a3
 Removing intermediate container 30297dbd21a3
  ---> 4c4838b629f4
 Step 5/10 : ENV PACKAGE_VERSION=0.5.3+1
  ---> Running in 36361b617252
 Removing intermediate container 36361b617252
  ---> 6ac0d3a2ff9a
Step 6/10 : ENV PACKAGE_CATEGORY=system
  ---> Running in f20c2cf3cf34
 Removing intermediate container f20c2cf3cf34
  ---> a902ff95d273
 Step 7/10 : COPY --from=quay.io/costoolkit/build-cache:f3a333095d9915dc17d7f0f5629a638a7571a01dcf84886b48c7b2e5289a668a /usr/bin/yip /usr/bin/yip
  ---> 42fa00d9c990
 Step 8/10 : COPY --from=quay.io/costoolkit/build-cache:e3bbe48c6d57b93599e592c5540ee4ca7916158461773916ce71ef72f30abdd1 /usr/bin/luet /usr/bin/luet
 e3bbe48c6d57b93599e592c5540ee4ca7916158461773916ce71ef72f30abdd1: Pulling from costoolkit/build-cache
 3599716b36e7:  Already exists
 24a39c0e5d06: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 378615c429f5: Already exists
 c28da22d3dfd:  Already exists
 ddb4dd5c81b0: Already exists
 92db41c0c9ab: Already exists
 4f4fb700ef54: Already exists
 6e0ca71a6514: Already exists
 47debb886c7d: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 d0c9d0f8ddb6: Already exists
 e5a48f1f72ad:  Pulling fs layer
 4f4fb700ef54:  Pulling fs layer
 7d603b2e4a37:  Pulling fs layer
 64c4d787e344:  Pulling fs layer
 f8835d2e60d1:  Pulling fs layer
 64c4d787e344:  Waiting
 f8835d2e60d1:  Waiting
 e5a48f1f72ad:  Download complete
 e5a48f1f72ad:  Pull complete
 4f4fb700ef54:  Verifying Checksum
 4f4fb700ef54:  Download complete
 4f4fb700ef54:  Pull complete
 7d603b2e4a37: Verifying Checksum
7d603b2e4a37: Download complete
 64c4d787e344: Verifying Checksum
64c4d787e344: Download complete
 7d603b2e4a37: Pull complete
 64c4d787e344: Pull complete
 f8835d2e60d1:  Verifying Checksum
 f8835d2e60d1:  Download complete
 f8835d2e60d1: Pull complete
 Digest: sha256:9b58bed47ff53f2d6cc517a21449cae686db387d171099a4a3145c8a47e6a1e0
 Status: Downloaded newer image for quay.io/costoolkit/build-cache:e3bbe48c6d57b93599e592c5540ee4ca7916158461773916ce71ef72f30abdd1
 failed to export image: failed to create image: failed to get layer sha256:118537d8997a08750ab1ac3d8e8575e40fe60e8337e02633b0d8a1287117fe78: layer does not exist
 Error: while resolving join images: failed building join image: failed building package image: Could not push image: raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d cos-system-0.5.3+1-builder.dockerfile: Could not build image: raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d cos-system-0.5.3+1-builder.dockerfile: Failed running command: : exit status 1
 Bailing out
make: *** [Makefile:45: build] Error 1
```

There is a issue open [upstream](https://github.com/moby/moby/issues/37965) about it. A workaround is to enable Docker buildkit with `DOCKER_BUILDKIT=1`.