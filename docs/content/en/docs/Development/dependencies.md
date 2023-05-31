
---
title: "Build requirements"
linkTitle: "Build requirements"
weight: 6
date: 2023-05-31
description: >
  Building prerequisites
---

### Installing required dependencies for local build

To get requirements installed locally, run:

```bash
$> make deps
```

or you need:

- [`elemental-cli`](https://github.com/rancher/elemental-toolkit)
- [`squashfs-tools`](https://github.com/plougher/squashfs-tools)
  - `zypper in squashfs` on SLES or openSUSE
- [`xorriso`](https://dev.lovelyhq.com/libburnia/web/wiki/Xorriso)
  - `zypper in xorriso` on SLES or openSUSE
- [`mtools`](https://www.gnu.org/software/mtools/)
  - `zypper in mtools` on SLES or openSUSE
- `yq` ([version `4.x`](https://github.com/mikefarah/yq/releases)), installed via [packages/toolchain/yq](https://github.com/rancher/elemental-toolkit/tree/main/packages/toolchain/yq) (optional)
- [`jq`](https://stedolan.github.io/jq), installed via [packages/utils/jq](https://github.com/rancher/elemental-toolkit/tree/main/packages/utils/jq) (optional)

#### elemental

`elemental` comes [with Elemental-toolkit](https://github.com/rancher/elemental-toolkit)

You can grab the binary from [elemental](https://github.com/rancher/elemental-toolkit) releases.


#### yq and jq
`yq` (version `4.x`) and `jq` are used to retrieve the list of
packages to build in order to produce the final ISOs. Those are not
strictly required, see the Note below. 

_Note_: `yq` and `jq` are just used to generate the list of packages to build, and you don't need to have them installed if you manually specify the packages to be compiled.
