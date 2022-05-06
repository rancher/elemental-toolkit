---
title: "Build Images for Google Compute Platform"
linkTitle: "Build GCP images"
weight: 4
date: 2017-01-05
description: >
  This section documents the procedure to deploy cOS (or derivatives) images
  in Google Compute Platform by using the cOS Vanilla image.
---

![](https://docs.google.com/drawings/d/e/2PACX-1vSqJWcFThP7K2HS551LCqs73l4ZncXElLjlbCvxY96Ga2Jbjnq79j-DEjaccUZvYEQyphWiDQc9flxk/pub?w=1223&h=691)

Requirements:

* Packer
* Google Compute Packer [plugin](https://www.packer.io/docs/builders/googlecompute)
* [Google Cloud SDK](https://cloud.google.com/sdk/docs/install)
* A Vanilla AMI
* [Packer templates](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packer)

The suggested approach is based on using Packer templates to customize the
deployment and automate the upload and publish to GCP of cOS derivatives or cOS itself. For all the details
and possibilties of Packer check the [official documentation](https://www.packer.io/guides/hcl).

There are no cOS Vanilla images publicly available in GCP, however they can be easily
build or downloaded and published to your working GCP project. See [Build Raw Images](../../../development/build_raw_images/) and
[Importing a Google Cloud image manually](../../../getting-started/booting/#importing-a-google-cloud-image-manually) to see how to upload a Vanilla image in your project.

## Run the build with Packer

Publishing an image in GCP based on top of the latest cOS Vanilla image is
fairly simple. In fact, it is only needed to set the [User Application Default Credentials](https://www.packer.io/docs/builders/googlecompute#running-locally-on-your-workstation)
for GCP and the GCP project ID and then run a `packer build` process to
trigger the deployment and register the resulting snapshot as an image.
In such case the lastest cOS image will be deployed and configured with
pure defaults. Consider:

```bash
# From the root of a cOS-toolkit repository checkout

> export GCP_PROJECT_ID=<your_gcp_project_id>

> cd packer
> packer build -only gcp.cos .
```

Packer authenticates automatically if the
[User Application Default Credentials](https://www.packer.io/docs/builders/googlecompute#running-locally-on-your-workstation)
are properly set in the host.

The `-only gcp.cos` flag is just to tell packer which of the sources
to make use for the build. Note the `packer/images.json.pkr.hcl` file defines
few other sources such as `qemu`, `virtualbox` and `amazon-ebs`.

## Customize the build with a variables file

The packer template can be customized with the variables defined in
`packer/variables.pkr.hcl`. These are the variables that can be set on run
time using the `-var key=value` or `-var-file=path` flags. The variable file
can be a json file including desired varibles. Consider the following example:

```bash
# From the packer folder of the cOS-toolkit repository checkout

> cat << EOF > test.json
{
    "gcp_project_id": "<your_gcp_project>"
    "gcp_cos_deploy_args": "elemental reset --docker-image <my-custom-image>",
    "gcp_disk_size": 20,
    "name": "MyTest"
}
EOF

> packer build -only gcp.cos -var-file=test.json .
```

The above example runs the cOS Vanilla image on a 20GB disk and calls the
command `elemental reset` to deploy the main OS, once deployed an snapshot is
created and published as an image in Google Compute Engine. The created
artifact will be called `MyTest`, the name has no impact in the underlaying
OS.

### Available variables for customization

All the customizable variables are listed in `packer/variables.pkr.hcl`, 
variables with the  `aws_` prefix are the ones related to the AWS Packer
template. These are some of the relevant ones:

* `gcp_project_id`: The project ID that will be used to launch instances and
  store images.

* `gcp_cos_deploy_args`: This the command that will be executed once the
  Vanilla image booted. In this stage it is expected that user sets a command
  to install the desired cOS or derivative image. By default it is set to
  `elemental reset` which will deploy the cOS image from the recovery partition.
  To deploy custom derivatives something like
  `elemental reset --docker-image <my-derivative-img-ref>` should be sufficient.

* `gcp_disk_size`: This sets the disk size of the VM that Packer
  launches for the build. During Vanilla image first boot the system will
  expand to the disk geometry. The layout is configurable with the user-data.

* `gcp_user_data_file`: This sets the user-data file that will be used for the
  aws instance during the build process. It defaults to `aws/setup-disk.yaml` and
  the defauklt file basically includes the disk expansion configuration. It
  adds a `COS_STATE` partition that should be big enough to store about three times
  the size of the image to deploy. Then it also creates a `COS_PERSISTENT`
  partition with all the rest of the available space in disk.

* `gcp_source_image_family`: This the family to choose the image for the
  build process. It defaults to `cos-vanilla` to pick the latest cOS
  Vanilla image available. Note Packer tries to find the image family first
  in the given working project (`gcp_project_id`).
