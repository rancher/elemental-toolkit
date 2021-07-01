### Installing required dependencies for local build

To get requirements installed locally, run:

```bash
$> make deps
```

or you need:

- [`luet`](https://github.com/mudler/luet)
- [`luet-makeiso`](https://github.com/mudler/luet-makeiso)
- [`squashfs-tools`](https://github.com/plougher/squashfs-tools)
  - `zypper in squashfs` on SLES or openSUSE
- [`xorriso`](https://dev.lovelyhq.com/libburnia/web/wiki/Xorriso)
  - `zypper in xorriso` on SLES or openSUSE
- `yq` ([version `3.x`](https://github.com/mikefarah/yq/releases/tag/3.4.1)), installed via [packages/toolchain/yq](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packages/toolchain/yq) (optional)
- [`jq`](https://stedolan.github.io/jq), installed via [packages/utils/jq](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packages/utils/jq) (optional)

_Note_: Running `make` deps will install only `luet`, `luet-makeiso`, `yq` and `jq`. `squashfs-tools` and `xorriso` needs to be provided by the OS.

### Manually install dependencies

To install luet locally, you can also run as root:
```bash
# curl https://raw.githubusercontent.com/rancher-sandbox/cOS-toolkit/master/scripts/get_luet.sh | sh
```
or build [luet from source](https://github.com/mudler/luet)).

You can find more luet components in the official [Luet repository](https://github.com/Luet-lab/luet-repo).


#### luet-makeiso

`luet-makeiso` comes [with cOS-toolkit](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packages/toolchain/luet-makeiso)
and can be installed with `luet` locally:

```bash
$> luet install -y toolchain/luet-makeiso
```

You can also grab the binary from [luet-makeiso](https://github.com/mudler/luet-makeiso) releases.


#### yq and jq
`yq` (version `3.x`) and `jq` are used to retrieve the list of
packages to build in order to produce the final ISOs. Those are not
strictly required, see the Note below. 


They are installable with:

```bash
$> luet install -y utils/jq toolchain/yq
```

_Note_: `yq` and `jq` are just used to generate the list of packages to build, and you don't need to have them installed if you manually specify the packages to be compiled.
