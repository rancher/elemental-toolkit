---
title: "Build disk images with Elemental"
linkTitle: "Build disk images with Elemental"
weight: 4
date: 2024-08-20
description: >
  This section documents the procedure to build disk images using elemental
---

In order to build a RAW disk image we rely on `elemental build-disk` command. It accepts a YAML file denoting the sources to bundle in the RAW image. In addition it can also overlay custom files or use container images from a registry as packages.

To build a RAW image, just run:

```bash
docker run --rm -ti -v $(pwd):/build ghcr.io/rancher/elemental-toolkit/elemental-cli:latest --debug build-disk --expandable --unprivileged --squash-no-compression -o /build $SOURCE
```

Argument `$SOURCE` might be the reference to the directory, file, container image or channel we are building the ISO for, it should be provided as uri in following format <sourceType>:<sourceName>, where:
    * <sourceType> - might be ["dir", "file", "oci", "docker"], as default is taken "docker"
    * <sourceName> - is path to file or directory, channel or image name with tag version (if tag was not provided then "latest" is used)

`elemental build-disk` command also supports reading a configuration `manifest.yaml` file. It is loaded form the directory specified by `--config-dir` elemental's flag.

An example of a yaml file using the bootloader from the contained image:

```yaml
# Representing the RAW disk final image name without including the '.raw'
name: "Elemental-0"
# Indicates if the output image name has to contain the date
date: true
# Folder destination of the built artifacts. It attempts to create if it doesn't exist.
output: /output/dir
# Snapshotter configuration
snapshotter:
  type: btrfs
  maxSnaps: 2
```

### Usage

```text
Usage:
  elemental build-disk image [flags]

Flags:
  -c, --cloud-init strings               Cloud-init config files to include in disk
      --cloud-init-paths strings         Cloud-init config files to run during build
      --cosign                           Enable cosign verification (requires images with signatures)
      --cosign-key string                Sets the URL of the public key to be used by cosign validation
      --date                             Adds a date suffix into the generated disk file
      --deploy-command strings           Deployment command for expandable images (default [elemental,--debug,reset,--reboot])
      --expandable                       Creates an expandable image including only the recovery image
  -h, --help                             help for build-disk
      --local                            Use an image from local cache
  -n, --name string                      Basename of the generated disk file
  -o, --output string                    Output directory (defaults to current directory)
      --platform string                  Platform to build the image for (default "linux/amd64")
  -x, --squash-compression stringArray   cmd options for compression to pass to mksquashfs. Full cmd including --comp as the whole values will be passed to mksquashfs. For a full list of options please check mksquashfs manual. (default value: '-comp xz -Xbcj ARCH')
      --squash-no-compression            Disable squashfs compression. Overrides any values on squash-compression
  -t, --type string                      Type of image to create (default "raw")
      --unprivileged                     Makes a build runnable within a non-privileged container, avoids mounting filesystems (experimental)

Global Flags:
      --config-dir string   Set config dir
      --debug               Enable debug output
      --logfile string      Set logfile
      --quiet               Do not output to stdout
```
