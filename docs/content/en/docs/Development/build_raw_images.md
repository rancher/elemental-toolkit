---
title: "Build Raw images"
linkTitle: "Build Raw images"
weight: 4
date: 2017-01-05
description: >
  This section documents the procedure to build cOS raw images
  which are used to boot into Cloud providers.
---

Requirements:

* `elemental` binary
* elemental runtime dependencies

The suggested approach high level view is building cOS packages and generating a RAW image from
them. That would allow us to transform that RAW image in a valid Azure/Google/Amazon Cloud blob that can be transformed into a VM image ready
to be launched.

This generates a `vanilla` image that boots into [recovery mode](../../getting-started/recovery) and can be used to deploy
whatever image you want to the VM. Then you can snapshot that VM into a VM image ready to deploy with the default cOS
system or your derivative.

The RAW image can then be used into packer templates to generate custom Images, or used as-is with a userdata to deploy a container image of choice with an input user-data.

## Getting the packages

By default, elemental will use the cos-toolkit repo to obtain the required packages to build the raw image.

We can also use locally built packages instead. Just run:

```bash
sudo make build
sudo make create-repo
```

All the artifacts will be generated under the `build` directory and a local repo will be created.

Then prepare a `manifest.yaml` file with the local repo, so build-disk can read from it:

```yaml
raw_image:
  x86_64: # arch we are building for
    repositories: # list of repositories to use for getting the needed packages
      - uri: /home/user/cos-toolkit/packages/
        type: local
```

As long as elemental is launched from the directory where the `manifest.yaml` file is, it will pick up that local repo and use the local packages to build the raw image.

Note that you can do the same to override the default remote repository, in order to use your own remote repository:

```yaml
raw_image:
  x86_64: # arch we are building for
    repositories: # list of repositories to use for getting the needed packages
      - uri: https://my.custom.repo
```

## Building the RAW image

The RAW image is just a RAW disk image that contains the recovery, so once launched is ready to be used for installing
whatever cOS or derivative that you want into the VM disks. This allows us to have a barebones base image that can be
used for provisioning whatever cOS you want.

Building the RAW image is as simple as running:

```bash
sudo elemental build-disk
```

This will output a `disk.raw` file in the current directory which can be run with qemu, see [booting](../../getting-started/booting).

### AWS

No other steps are required, the raw disk can be already booted as an AMI image, see [booting](../../getting-started/booting).

### Azure

Requirements:

* Azure Cloud access keys with the appropriate roles and permissions
* [azure-cli](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)

#### Transform the RAW image into a compatible Azure Cloud blob

Currently importing images from storage blobs on Azure Cloud have a [few requirements](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/create-upload-generic):

 - only fixed VHD format is supported
 - VHD disk must have a virtual size aligned to 1 MB
 - LVM is not supported
 - no swap partition

Elemental provides a `--type` flag that can be used to output an azure image directly:

```bash
sudo elemental build-disk -t azure
```

This will create a `disk.raw.vhd` which is our final artifact

Elemental also provides a convert command that can turn a previously created RAW image into an azure image:

```bash
elemental convert-disk -t azure --keep-source disk.raw
```


#### Uploading to Azure Cloud storage and importing it as an image

The last step is to upload the blob to Azure Cloud storage and import that blob as a valid image.

With [azure-cli](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installed
and its credentials configured, you upload the blob with:

```bash
az storage copy --source <cos-azure-image> --destination https://<account>.blob.core.windows.net/<container>/<destination-cos-azure-image>
```

And import it as an image with:

```bash
az image create --resource-group <resource-group> --source https://<account>.blob.core.windows.net/<container>/<cos-azure-image> --os-type linux --hyper-v-generation v2 --name <image-name>
```

Where cos-azure-image is the blob we just uploaded, basically you can use the same value as you set in `--destionation` for the upload
   
Note that we used `--os-type linux --hyper-v-generation v2` as flags. This indicates that the image will be booted with UEFI
and its required. Otherwise, launching the image will try to boot it in legacy mode, and it will fail.


Once this is over you will have you cOS (or derivative) vanilla image ready for consumption.
You can see your new image by running:

```bash
az image show --resource-group <resource-group> --name <image-name>
```

### Google cloud

Requirements:

* Google Cloud access keys with the appropriate roles and permissions
* [gsutil](https://cloud.google.com/storage/docs/gsutil) and [gcloud](https://cloud.google.com/sdk) tools

#### Transform the RAW image into a compatible Google Cloud blob

Currently importing images from storage blobs on Google Cloud have a [few requirements](https://cloud.google.com/compute/docs/import/import-existing-image#requirements_for_the_image_file):

 - blobs have to be tar.gzipped with the flag `--format=oldgnu`
 - the disk.raw has to be rounded up to the next Gb ( so no 2.1gb images or 2.4, they need to be an exact 3Gb or 2Gb)
 - image inside the tar.gzip blob needs to be called disk.raw

Elemental provides a `--type` flag that can be used to output a GCE image directly:

```bash
sudo elemental build-disk -t gce
```

This will create a `disk.raw.tar.gz` which is our final artifact

Elemental also provides a convert command that can turn a previously created RAW image into an azure image:

```bash
elemental convert-disk -t gce --keep-source disk.raw
```


#### Uploading to Google Cloud storage and importing it as an image

The last step is to upload the blob to Google Cloud storage and import that blob as a valid image.

With [gsutil](https://cloud.google.com/storage/docs/gsutil) and [gcloud](https://cloud.google.com/sdk) tools installed
and their credentials configured, you upload the blob with:

```bash
gsutil cp disk.raw.tar.gz gs://YOURBUCKET/
```

Where YOURBUCKET is the destination bucket you want your file to end up in.

And import it as an image with:

```bash
gcloud compute images create IMAGENAME --source-uri=SOURCEURI --guest-os-features=UEFI_COMPATIBLE
```

Where:
 - IMAGENAME: The name for the final image
 - SOURCEURI: The full Google Cloud Storage URI where the disk image is stored.
   This file must be a gzip-compressed tarball whose name ends in
   .tar.gz.
   This is the full path to the blob we just uploaded.
   
Note that we used `--guest-os-features=UEFI_COMPATIBLE` as a flag. This indicates that the image will be booted with UEFI
and its required. Otherwise, launching the image will try to boot it in legacy mode and it will fail.


Once this is over you will have you cOS (or derivative) vanilla image ready for consumption.
You can see your new image by running:

```bash
gcloud compute images describe IMAGENAME
```
