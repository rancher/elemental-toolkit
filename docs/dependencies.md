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
- `yq` ([version `3.x`](https://github.com/mikefarah/yq/releases/tag/3.4.1))  (optional)
- [`jq`](https://stedolan.github.io/jq) (optional)

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
$> luet install -y utils/jq utils/yq
```

_Note_: `yq` and `jq` are just used to generate the list of packages to build, and you don't need to have them installed if you manually specify the packages to be compiled.
