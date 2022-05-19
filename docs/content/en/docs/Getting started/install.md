
---
title: "Installing"
linkTitle: "Installing"
weight: 2
date: 2021-11-04
description: >
  Installing cOS or a derivative locally
---


cOS (or any cOS derivative built with cos-toolkit) can be installed with `elemental install`:

```bash
elemental install [options] <device>
```

| Option                       | Description                                                                                                  |
|------------------------------|--------------------------------------------------------------------------------------------------------------|
| --cloud-init string          | Cloud-init config file                                                                                       |
| --cosign                     | Enable cosign verification (requires images with signatures)                                                 |
| --cosign-key string          | Sets the URL of the public key to be used by cosign validation                                               |
| --eject-cd                   | Try to eject the cd on reboot, only valid if booting from iso                                                |
| --firmware string            | Firmware to install for ('esp' or 'bios') (default "efi")                                                    |
| --force                      | Force install                                                                                                |
| --help                       | help for install                                                                                             |
| --iso string                 | Performs an installation from the ISO url                                                                    |
| --no-format                  | Don’t format disks. It is implied that COS_STATE, COS_RECOVERY, COS_PERSISTENT, COS_OEM are already existing |
| --verify                     | Enable mtree checksum verification (requires images manifests generated with mtree separately)               |
| --part-table string          | Partition table type to use (default "gpt")                                                                  |
| --poweroff                   | Shutdown the system after install                                                                            |
| --reboot                     | Reboot the system after install                                                                              |
| --recovery-system.uri string | Sets the recovery image source and its type (e.g. 'docker:registry.org/image:tag')                           |
| --system.uri string          | Sets the system image source and its type (e.g. 'docker:registry.org/image:tag')                             |
| --strict                     | Enable strict check of hooks (They need to exit with 0)                                                      |
| --tty string                 | Add named tty to grub                                                                                        |


### Custom OEM configuration

During installation it can be specified a [cloud-init config file](../../reference/cloud_init), that will be installed and persist in the system after installation:

```bash
elemental install --cloud-init [url|path] <device>
```

### Custom partitioning

When installing it's possible to specify a custom partition sizes via the configuration file (`/etc/elemental/config.yaml` by default).

```yaml
install
   partitions:
     state:
       # All sizes are in MiB
       size: 8192
     recovery:
       size: 4096
     oem:
       size: 64
     persistent:
       # zero size tells parted to use all the available
       # disk, note this is only functional for the last partition
       size: 0
```

Refer to the [config file docs](../../customizing/general_configuration) for further details about all partitioning options.

In order to create additional partitions please consider the layout section on [cloud-init config file reference](../../reference/cloud_init)

### Installation from 3rd party LiveCD or rescue mediums

The installer can be used to perform installations also from outside the cOS or standard derivative ISOs.

For instance, it is possible to install cOS (or any derivative) with the installer from another bootable medium, or a rescue mode which is booting from RAM, given there is enough free RAM available.

#### With Docker

If in the rescue system, or LiveCD you have docker available, it can be used to perform an installation

```bash
docker run --privileged -v /dev/:/dev/ -ti quay.io/costoolkit/elemental:latest install --system.uri $IMAGE $DEVICE
```

Where `$IMAGE` is the container image that we want to install (e.g. `docker:quay.io/costoolkit/releases-teal:cos-system-0.8.10-3` ), elemental identifies the type of source by the URI scheme (`docker`, `channel`, `dir` or `file`). `$DEVICE` is the device where to perform the installation to (e.g. `/dev/sda`).


Note, we used the `quay.io/costoolkit/elemental:latest` image which contains the latest stable installer and the dependencies.
You can see all the versions at [quay](https://quay.io/repository/costoolkit/elemental?tab=tags).


#### By using manually the Elemental installer

Similarly, the same mechanism can be used without docker. Download elemental from [github releases](https://github.com/rancher-sandbox/elemental/releases/latest) and run the follow as root:

```bash
elemental install --system.uri $IMAGE $DEVICE
```
