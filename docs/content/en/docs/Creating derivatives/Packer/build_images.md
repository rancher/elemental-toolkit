---
title: "Build QCOW, VirtualBox and Vagrant images"
linkTitle: "Build QCOW, VirtualBox and Vagrant images"
weight: 4
date: 2017-01-05
description: >
  This section documents the procedure to build a custom QCOW, VirtualBox and a Vagrant images with the cOS packer templates
---

![](https://docs.google.com/drawings/d/e/2PACX-1vT-ZugVPCUCffRbfko-tOoTyRIpqjtgvQgQn74lckTZCjMLIakEJKRPwyjFL7tGEmKE8DDMVSZBEZ9u/pub?w=1223&h=691)

Requirements:

* Packer
* Either qemu or VirtualBox functioning in the build host
* a cOS or a [custom ISO](../../build_iso)
* [Packer templates](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packer)

The suggested approach is based on using [Packer templates](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packer) to customize the
deployment and automate creation of QCOW, Virtualbox and Vagrant images of cOS derivatives or cOS itself. For all the details
and possibilties of Packer check the [official documentation](https://www.packer.io/guides/hcl).

## Run the build with Packer

To build QCOW and VirtualBox images an ISO file is required. You can either [Download](../../../getting-started/download) a cOS ISO or [build your own from a container image](../../build_iso).

### QCOW2

Consider:

```bash
# From the root of a cOS-toolkit repository checkout

> cd packer
> packer build -var "iso=/path/to/image.iso" -only qemu.cos .
```

The process should end up by building a `.box` file (vagrant image) and a `.tar.gz` file (containing the raw disk) in the same folder:

```
...
==> qemu.cos (compress): Using pgzip compression with 8 cores for cOS_green_dev_amd64.tar.gz
==> qemu.cos (compress): Tarring cOS_green_dev_amd64.tar.gz with pgzip
==> qemu.cos (compress): Archive cOS_green_dev_amd64.tar.gz completed
Build 'qemu.cos' finished after 4 minutes 34 seconds.

==> Wait completed after 4 minutes 34 seconds

==> Builds finished. The artifacts of successful builds are:
--> qemu.cos: 'libvirt' provider box: cOS_green_dev_amd64.box
--> qemu.cos: compressed artifacts in: cOS_green_dev_amd64.tar.gz

> ubuntu@jumpbox:~/cOS-toolkit/packer$ ls -liah
total 2.3G
 516552 drwxrwxr-x  4 ubuntu ubuntu 4.0K Jul 30 07:54 .
 516207 drwxrwxr-x 14 ubuntu ubuntu 4.0K Jul 30 07:41 ..
 516362 -rw-r--r--  1 root   root   1.2G Jul 30 07:54 cOS_green_dev_amd64.box
 516385 -rw-r--r--  1 root   root   1.2G Jul 30 07:54 cOS_green_dev_amd64.tar.gz
 516553 -rw-rw-r--  1 ubuntu ubuntu  389 Jun 28 08:07 config.yaml
 516339 -rw-rw-r--  1 ubuntu ubuntu 6.5K Jul 30 07:41 images.json.pkr.hcl
1818463 drwxrwxr-x  3 ubuntu ubuntu 4.0K Jul 30 07:49 packer_cache
1818400 drwxrwxr-x  2 ubuntu ubuntu 4.0K Jul 30 07:41 user-data
 516555 -rw-rw-r--  1 ubuntu ubuntu  606 Jun 28 08:07 vagrant.yaml
 516340 -rw-rw-r--  1 ubuntu ubuntu 6.1K Jul 30 07:41 variables.pkr.hcl
ubuntu@jumpbox:~/cOS-toolkit/packer$ 
```

The `-only qemu.cos` flag is just to tell packer which of the sources
to make use for the build. Note the `packer/images.json.pkr.hcl` file defines
few other sources such as `amazon-ebs` and `virtualbox-iso`.

### Virtualbox

Similarly, to build OVA images we run:

```bash
# From the root of a cOS-toolkit repository checkout

> cd packer
> packer build -var "iso=/path/to/image.iso" -only virtualbox-iso.cos .
```

### Vagrant

To build vagrant images, we enable the `vagrant` feature, which allows the `vagrant` user to login afterwards:

```bash
# From the root of a cOS-toolkit repository checkout

> cd packer
> packer build -var "iso=/path/to/image.iso" -var "feature=vagrant" -only virtualbox-iso.cos .
```

## Customize the build with a variables file

The packer template can be customized with the variables defined in
`packer/variables.pkr.hcl`. These are the variables that can be set on run
time using the `-var key=value` or `-var-file=path` flags. The variable file
can be a json file including desired varibles. Consider the following example:

```bash
# From the packer folder of the cOS-toolkit repository checkout

> cat << EOF > test.json
{
    "root_username": "root",
    "root_password": "cos"
}
EOF

> packer build -only qemu.cos -var "iso=/path/to/image.iso" -var-file=test.json .
```

The above example runs the build by logging with a different username/password to run the installation.

### Default cloud-init

In the packer folder there is present a `config.yaml` file which can be used to customize the image. The file is in [cloud-init](../../../references/cloud-init) style and will be automatically installed by default when running `elemental install`.

### Available variables for customization

All the customizable variables are listed in `packer/variables.pkr.hcl`, 
these are some of the relevant ones:

* `build`,`flavor`,`arch`: This to personalize the artifact output name. The artifact output format is
  `"cOS_${var.flavor}_${var.build}_${var.arch}.tar.gz"`

* `disk_size`: This sets the disk size to be used for the image. It is 50000 by default, and expressed in MB.

* `memory`: RAM to allocate to the VM in MB.

* `cpu`: Number of processors of the VM

* `accelerator`: Accelerator type, see the [official docs](https://www.packer.io/docs/builders/qemu#accelerator)

* `root_username`: Username used to login via ssh, it needs root permissions to be able to run the installation process and call `elemental install`

* `root_password`: Password for the user specified in `root_username`

* `feature`: Enable/Disables specific `cOS` features.
