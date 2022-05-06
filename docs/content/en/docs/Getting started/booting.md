
---
title: "Booting"
linkTitle: "Booting"
weight: 2
date: 2017-01-05
description: >
  Documents various methods for booting cOS vanilla images
---

{{<image_right image="https://docs.google.com/drawings/d/e/2PACX-1vQXQFmc4MnmRsPCLR1_hCElykMWbcye6TY-zDWZhVyFbIqFEXyVsLgPIKVqUCZaQTkAE00uAK66Mt-D/pub?w=507&h=217">}}

Each cOS release contains a variety of assets:

- ISOs `cOS-Seed-green-$VERSION-$ARCH.iso.tar.xz`
- QCOW `cOS-Packer-green_$VERSION-QEMU-$ARCH.tar.gz.tar.xz`
- OVA `cOS-Packer-green_$VERSION-vbox-$ARCH.tar.gz.tar.xz`
- Vagrant box (vbox provider) `cOS-Packer-green-$VERSION-vbox-$ARCH.box.tar.xz`
- Vagrant box (qemu provider) `cOS-Packer-green-$VERSION-QEMU-$ARCH.box.tar.xz`
- RAW Disk `cOS-Vanilla-RAW-$VERSION-$ARCH.raw.tar.xz`
- VHD `cOS-Vanilla-AZURE-$VERSION-$ARCH.vhd.tar.xz`
- GCE `cOS-Vanilla-GCE-green-$VERSION-$ARCH.raw.tar.xz`

here we try to summarize and document how they are meant to be consumed.

## ISO

ISO images (e.g. ``cOS-Seed-green-$VERSION-$ARCH.iso.tar.xz` ) are shipping a cOS vanilla image and they feature an installer to perform an automated installation. They can be used to burn USB sticks or CD/DVD used to boot baremetals. Once booted, you can install cOS with:

```bash
elemental install $DEVICE
```

See also [../install] for installation options.

After the first boot you can also switch to a derivative by:

```bash
elemental upgrade --docker-image --no-verify $IMAGE
```

## Booting from network

You can boot a cOS squashfs by using a native iPXE implementation on your
system or by inserting a custom build iPXE iso by using your either your servers
management web-interface (ILO, iDrac, ...) or using a
[redfish](https://www.dmtf.org/standards/redfish) library or its vendor specific implementation (e.g. [ilorest](https://hewlettpackard.github.io/python-redfish-utility/)). 

To boot from IPXE you need to extract the squashfs from
[images](https://quay.io/repository/costoolkit/releases-green?tab=tags) with the `cos-img-recovery-` prefix.
By pulling the image, saving it using docker save, then unpack the single layer
in the file to an extra directory and then run 

```bash
$> mksquashfs <path_to_folder> <output_path>/root.squashfs
```

**Note:**
The squashfs file can also be extracted using the following `docker` command:

```bash
$> docker run -v $PWD:/cOS --entrypoint /usr/bin/luet -ti --rm quay.io/costoolkit/toolchain util unpack quay.io/costoolkit/releases-green:cos-img-recovery-<version> .
```

Then copy the `boot` directory and the squashfs file to your webserver and use
the following script to boot.

```bash
#!ipxe

ifconf
kernel http://<web_server_ip>/boot/vmlinuz ip=dhcp rd.cos.disable rd.noverifyssl
root=live:http://<web_server_ip>/root.squashfs console=ttyS0 console=tty1 cos.setup=http://<web_server_ip>/<path_to_cloud_config.yaml>
initrd http://<web_server_ip>/boot/initrd
boot
```

To build a custom iPXE image clone the source from
[https://github.com/ipxe/ipxe](https://github.com/ipxe/ipxe), then traverse into
the `src` directory and run `make EMBED=</path/to/your/script.ipxe>`. The
resulting iso can be found in `src/bin/ipxe.iso` and should be `~1MB` in size.

## Virtual machines

For booting into Virtual machines we offer QCOW2, OVA, and raw disk recovery images which can be used to bootstrap your booting container.

### QCOW2

QCOW2 images ( e.g. `cOS-Packer-green-$VERSION-QEMU-$ARCH.tar.gz.tar.xz` ) contains a pre-installed cOS system which can be booted via QEMU, e.g:

```bash
qemu-system-x86_64 -m 2048 -hda cOS -nographic
```

### OVA

Ova images ( e.g. `cOS-Packer-green-$VERSION-vbox-$ARCH.tar.gz.tar.xz` ) contains a pre-installed cOS system which can be booted via vbox.
Please check the virtuabox docs on how to create a new VM with an existing disk.

### Vagrant

Download the vagrant box artifact ( e.g. `cOS-Packer-green-$VERSION-{vbox, QEMU}-$ARCH.box.tar.xz` ), extract it and run:

```bash
vagrant box add cos <cos-box-image>
vagrant init cos
vagrant up
```

### RAW disk images

RAW disk images ( e.g. `cOS-Vanilla-RAW-green-$VERSION-$ARCH.raw.tar.xz` ) contains only the `cOS` recovery system. Those are typically used when creating derivatives images based on top of `cOS`.

They can be run with QEMU with:

```bash
qemu-system-x86_64 -m 2048 -hda <cos-disk-image>.raw -bios /usr/share/qemu/ovmf-x86_64.bin
```

## Cloud Images

Cloud images are `vanilla` images that boots into [recovery mode](../recovery) and can be used to deploy
whatever image you want to the VM. Then you can snapshot that VM into a VM image ready to deploy with the default cOS
system or your derivative.

At the moment we support Azure and AWS images among our artifacts. We publish AWS images that can also be re-used in packer templates for creating customized AMI images. 

The RAW image can then be used into packer templates to generate custom Images, or used as-is with a userdata to deploy a container image of choice with an input user-data.

### Import an AWS image manually

{{% pageinfo %}}
You can also use RAW images ( e.g. `cOS-Vanilla-RAW-green-$VERSION-$ARCH.raw.tar.xz` ) manually when importing AMIs images and use them to generate images with Packer. See [build AMI with Packer](../../creating-derivatives/packer/build_ami)
{{% /pageinfo %}}

1. Upload the raw image to an S3 bucket
```
aws s3 cp <cos-raw-image> s3://<your_s3_bucket>
```

2. Created the disk container JSON (`container.json` file) as:

```
{
  "Description": "cOS Testing image in RAW format",
  "Format": "raw",
  "UserBucket": {
    "S3Bucket": "<your_s3_bucket>",
    "S3Key": "<cos-raw-image>"
  }
}
```

3. Import the disk as snapshot

```
aws ec2 import-snapshot --description "cOS PoC" --disk-container file://container.json
```

4. Followed the procedure described in [AWS docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/creating-an-ami-ebs.html#creating-launching-ami-from-snapshot) to register an AMI from snapshot. Used all default settings unless for the firmware, set to force to UEFI boot.

5. Launch instance with this simple userdata with at least a 16Gb boot disk:
```
name: "Default deployment"
stages:
   rootfs.after:
     - name: "Repart image"
       layout:
         # It will partition a device including the given filesystem label or part label (filesystem label matches first)
         device:
           label: COS_RECOVERY
         add_partitions:
           - fsLabel: COS_STATE
             # 10Gb for COS_STATE, so the disk should have at least 16Gb
             size: 10240
             pLabel: state
           - fsLabel: COS_PERSISTENT
             # unset size or 0 size means all available space
             pLabel: persistent
   initramfs:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Set sshd to wait for deployment"
       files:
       - path: "/etc/systemd/system/sshd.service.d/override.conf"
         content: |
             [Unit]
             After=cos-setup-network.service
   network:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Deploy cos-system"
       commands:
         - |
             # Use `elemental reset --docker-image <img-ref>` to deploy a custom image
             # By default the recovery cOS gets deployed
             elemental reset --reboot

```


### Importing a Google Cloud image manually

{{% pageinfo %}}
You need to use the GCE images ( e.g. `cOS-Vanilla-GCE-green-$VERSION-$ARCH.raw.tar.xz` ) manually when importing GCE images and use them to generate images with Packer. See [build GCE with Packer](../../creating-derivatives/packer/build_gcp)
{{% /pageinfo %}}

1. Upload the Google Cloud compressed disk to your bucket

```bash
gsutil cp <cos-gce-image> gs://<your_bucket>/
```

2. Import the disk as an image

```bash
gcloud compute images create <new_image_name> --source-uri=<your_bucket>/<cos-gce-image> --guest-os-features=UEFI_COMPATIBLE
```

3. Launch instance with this simple userdata with at least a 16Gb boot disk:

{{% info %}}See [here](https://cloud.google.com/container-optimized-os/docs/how-to/create-configure-instance#using_cloud-init_with_the_cloud_config_format) on how to add user-data to an instance{{% /info %}}

```yaml
name: "Default deployment"
stages:
   rootfs.after:
     - name: "Repart image"
       layout:
         # It will partition a device including the given filesystem label or part label (filesystem label matches first)
         device:
           label: COS_RECOVERY
         add_partitions:
           - fsLabel: COS_STATE
             # 10Gb for COS_STATE, so the disk should have at least 16Gb
             size: 10240
             pLabel: state
           - fsLabel: COS_PERSISTENT
             # unset size or 0 size means all available space
             pLabel: persistent
   initramfs:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Set sshd to wait for deployment"
       files:
       - path: "/etc/systemd/system/sshd.service.d/override.conf"
         content: |
             [Unit]
             After=cos-setup-network.service
   network:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Deploy cos-system"
       commands:
         - |
             # Use `elemental reset --docker-image <img-ref>` to deploy a custom image
             # By default recovery cOS gets deployed
             elemental reset --reboot
```


### Importing an Azure image manually

{{% info %}}
You need to use the AZURE images ( e.g. `cOS-Vanilla-AZURE-green-$VERSION-$ARCH.raw.tar.xz` ) manually when importing Azure images.
{{% /info %}}

1. Upload the Azure Cloud VHD disk in `.vhda` format ( extract e.g. `cOS-Vanilla-AZURE-green-0.6.0-g7d04f1db-x86_64.vhd.tar.xz` ) to your bucket

```bash
az storage copy --source <cos-azure-image> --destination https://<account>.blob.core.windows.net/<container>/<destination-cos-azure-image>

```

2. Import the disk as an image

```bash
az image create --resource-group <resource-group> --source https://<account>.blob.core.windows.net/<container>/<cos-azure-image> --os-type linux --hyper-v-generation v2 --name <image-name>
```

3. Launch instance with this simple userdata with at least a 16Gb boot disk:

Hint: There is currently no way of altering the boot disk of an Azure VM via GUI, use the azure-cli to launch the VM with an expanded OS disk

```yaml
name: "Default deployment"
stages:
   rootfs.after:
     - name: "Repart image"
       layout:
         # It will partition a device including the given filesystem label or part label (filesystem label matches first)
         device:
           label: COS_RECOVERY
         add_partitions:
           - fsLabel: COS_STATE
             # 10Gb for COS_STATE, so the disk should have at least 16Gb
             size: 10240
             pLabel: state
           - fsLabel: COS_PERSISTENT
             # unset size or 0 size means all available space
             pLabel: persistent
   initramfs:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Set sshd to wait for deployment"
       files:
       - path: "/etc/systemd/system/sshd.service.d/override.conf"
         content: |
             [Unit]
             After=cos-setup-network.service
   network:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Deploy cos-system"
       commands:
         - |
             # Use `elemental reset --docker-image <img-ref>` to deploy a custom image
             # By default recovery cOS gets deployed
             elemental reset --reboot
```

## Login

By default you can login with the user `root` and password `cos`.

See the [customization section](../../customizing/login) for examples on how to persist username and password changes after installation.
