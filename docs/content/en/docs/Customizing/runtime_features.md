---
title: "Runtime features"
linkTitle: "Runtime features"
weight: 6
date: 2021-09-02
description: >
  Add features that can be enabled and disabled on runtime
---

cOS allows to (optionally) add features that can be disabled/enabled in runtime, provided by {{<package package="system/cos-features" >}}.

[Cloud-init files](../../reference/cloud_init) stored in `/system/features` are read by `cos-feature` and allow to interactively enable or disable them in a running system, for example:

```
$> cos-feature list

====================
cOS features list

To enable, run: cos-feature enable <feature>
To disable, run: cos-feature disable <feature>
====================

- vagrant (enabled)
- ...
...
```

{{% alert title="Note" %}}
`/system/features` is the default path which can be customized in the [cOS configuration file](../general_configuration) by specifying it with `COS_FEATURESDIR`.
{{% /alert %}}

By default cOS ships the `vagrant` featureset - when enabled will automatically create the default `vagrant` user which is generally used to create new Vagrant boxes. 

If you don't need `cos-features` you can avoid installing {{<package package="system/cos-features" >}}, it's optional.

## Adding or removing features

To either add or remove the available features, delete the relevant files in the `/system/features` folder of the derivative prior to build the [container image](../../creating-derivatives/creating_bootable_images).

