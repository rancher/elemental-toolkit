Welcome!

The cOS Distribution is entirely built over GitHub. You can check the pipelines in the `.github` folder to see how the process looks like.

## Repository layout

- `packages`: contain packages definition for luet
- `values`: interpolation files, needed only for multi-arch and flavor-specific build
- `assets`: static files needed by the iso generation process
- `packer`: Packer templates
- `tests`: cOS test suites
- `manifest.yaml`: Is the manifest needed used to generate the ISO and additional packages to build

## Forking and test on your own

By forking the `cOS` repository, you already have the Github Action workflow configured to start building and pushing your own `cOS` fork.
The only changes required to keep in mind for pushing images:
- set `DOCKER_PASSWORD` and `DOCKER_USERNAME` as Github secrets, which are needed to push the resulting docker images from the pipeline. 
- Tweak or set the `Makefile`'s `REPO_CACHE` and `FINAL_REPO` accordingly. Those are used respectively for an image used for cache, and for the final image reference.

Those are not required for building - you can disable image push (`--push`) from the `Makefile` or just by specifying e.g. `BUILD_ARGS=--pull` when calling the `make` targets.

## Building locally

cOS has a docker image which can be used to build cOS locally in order to generate the cOS packages and the cOS iso from your checkout.

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

### Requirements for local build

To get requirements installed locally, run:

```bash
$> make deps
```

or you need:

- [luet](https://github.com/mudler/luet)
- [luet-makeiso](https://github.com/mudler/luet-makeiso)
- `squashfs-tools`
- `xorriso`
- `yq` (version `3.x`)  (optional)
- `jq` (optional)

_Note_: Running `make` deps will install only `luet`, `luet-makeiso`, `yq` and `jq`. `squashfs-tools` and `xorriso` needs to be provided by the OS.

### Manually install dependencies

To install luet locally, you can also run as root:
```bash
$> curl https://get.mocaccino.org/luet/get_luet_root.sh | sh
```
or either build from source (see [luet](https://github.com/mudler/luet)).

The Luet official repository that are being installed by the script above are:
- [official Luet repository](https://github.com/Luet-lab/luet-repo)
- [mocaccino-extra repository](https://github.com/mocaccinoOS/mocaccino-extra) (installable afterwards also with `luet install -y repository/mocaccino-extra-stable`) that contains the `yq` and `jq` versions that are used by the CI. 


#### luet-makeiso

Available in the [official Luet repository](https://github.com/Luet-lab/luet-repo). After installing `luet` with the curl command above, is sufficient to:

```bash
$> luet install -y extension/makeiso
```

to install it locally, otherwise grab the binary from [luet-makeiso](https://github.com/mudler/luet-makeiso) releases.

#### yq and jq
`yq` (version `3.x`) and `jq` are used to retrieve the list of packages to build in order to produce the final ISOs. Those are not strictly required, see the Note above. 

Install the `mocaccino-extra` repository:

```bash
$> luet install -y repository/mocaccino-extra-stable
```

They are installable with:

```bash
$> luet install -y utils/yq utils/yq
```

_Note_: `yq` and `jq` are just used to generate the list of packages to build, and you don't need to have them installed if you manually specify the packages to be compiled.

### Build all packages locally

```
$> make build
```

To clean from previous runs, run `make clean`.

_Note_: The makefile uses `yq` and `jq` to retrieve the packages to build from the iso specfile. If you don't have `jq` and `yq` installed, you must pass by the packages manually with `PACKAGES` (e.g. `PACKAGES="system/cos live/systemd-boot live/boot live/syslinux`).

You might want to build packages running as `root` or `sudo -E` if you intend to preserve file permissions in the resulting packages (mainly for `xattrs`, and so on).

### Build ISO

If using opensuse, first install the required deps:

```
$> zypper in -y squashfs xorriso dosfstools
```

and then, simply run

```
$> make local-iso
```

### Testing ISO changes

To test changes against a specific set of packages, you can for example:

```bash

$> make PACKAGES="live/init"  build local-iso

```

root is required because we want to keep permissions on the output packages (not really required for experimenting).

### Run with qemu

After you have the iso locally, run

```bash

$> QEMU=qemu-system-x86_64 make run-qemu

```

### Run tests

Requires: Virtualbox or libvirt, vagrant, packer

We have a test suite which runs over SSH.

To create the vagrant image:

```bash

$> PACKER_ARGS="-var='feature=vagrant' -only virtualbox-iso" make packer

```

To run the tests:

```bash

$> make test

```
