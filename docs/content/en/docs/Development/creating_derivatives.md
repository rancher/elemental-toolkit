---
title: "Creating derivatives"
linkTitle: "Creating derivatives"
weight: 3
date: 2017-01-05
description: >
  This document summarize references to create derivatives with `elemental-toolkit`.

---

`elemental-toolkit` is a manifest to share a common abstract layer between derivatives inheriting the same featureset. 

## High level workflow

The building workflow can be resumed in the following steps:

- Build a container image (`docker build` / `nerdctl build` / `buildah` ..)
- Publish the image (`docker push` / `nerdctl push` )
- Build an ISO (`elemental build-iso`)
- Boot a machine using the ISO and run installation (`elemental install`)
- Reboot into the installed system

While on the client side, the upgrade workflow is:
- `elemental upgrade --system.uri=oci:<image:version>`

## Single image OS

Derivatives are composed by a combination of specs to form a single image OS.

The container image during installation and upgrade, is converted to an image file with a backing ext2 fs. 

## Build ISO

To build an iso for a derivative image `elemental build-iso` command can be used:

```bash
elemental build-iso -n $NAME $SOURCE
```

Where `$NAME` is the name of the ISO and `$SOURCE` might be the reference to the directory, file, container image or chaneel we are building the ISO for. `$SOURCE` should be provided as uri in following format <sourceType>:<sourceName>, where:
    * <sourceType> - might be ["dir", "file", "oci", "docker"], as default is taken "oci"
    * <sourceName> - is path to file or directory, channel or image name with tag version (if tag was not provided then "latest" is used)

Some examples for $SOURCE argument "dir:/cOS/system", "oci:quay.io/repository/costoolkit/releases-green:cos-system-0.8.14-10"

See also [building ISOs](../../creating-derivatives/build_iso)
