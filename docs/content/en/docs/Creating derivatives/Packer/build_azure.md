---
title: "Build Azure images"
linkTitle: "Build Azure images"
weight: 4
date: 2017-01-05
description: >
  This section documents the procedure to deploy cOS (or derivatives) images
  in Azure public cloud provider by using the cOS Vanilla image.
---

![](https://docs.google.com/drawings/d/e/2PACX-1vSqJWcFThP7K2HS551LCqs73l4ZncXElLjlbCvxY96Ga2Jbjnq79j-DEjaccUZvYEQyphWiDQc9flxk/pub?w=1223&h=691)

Requirements:

* Packer
* Azure access keys with the appropriate roles and permissions
* [A Vanilla image](../../../getting-started/booting/#importing-an-azure-image-manually)
* [Packer templates](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packer)

The suggested approach is based on using Packer templates to customize the
deployment and automate the upload and publish to Azure of cOS derivatives or cOS itself. For all the details
and possibilities of Packer check the [official documentation](https://www.packer.io/guides/hcl).

## Run the build with Packer

Publishing an image in Azure based on top of the latest cOS Vanilla image is
fairly simple. In fact, it is only needed to set the Azure credentials
and run a `packer build` process to trigger the deployment and register the
resulting snapshot as an image. In such case the latest cOS image will be
deployed and configured with pure defaults. Consider:

```bash
# From the root of a cOS-toolkit repository checkout

> export AZURE_CLIENT_ID=<your_azure_client_id> 
> export AZURE_TENANT_ID=<your_azure_tenant_id> 
> export AZURE_CLIENT_SECRET=<your_azure_client_secret>
> export AZURE_SUBSCRIPTION_ID=<your_azure_subscription_id>

> cd packer
> packer build -only azure-arm.cos .
```

The `-only azure-arm.cos` flag is just to tell packer which of the sources
to make use for the build. Note the `packer/images.json.pkr.hcl` file defines
few other sources such as `qemu` and `virtualbox`.

## Customize the build with a variables file

The packer template can be customized with the variables defined in
`packer/variables.pkr.hcl`. These are the variables that can be set on run
time using the `-var key=value` or `-var-file=path` flags. The variable file
can be a json file including desired variables. Consider the following example:

```bash
# From the packer folder of the cOS-toolkit repository checkout

> cat << EOF > test.json
{
    "azure_location": "westeurope",
    "azure_os_disk_size_gb": 16,
    "azure_vm_size": "Standard_B2s"
}
EOF

> packer build -only azure-arm.cos -var-file=test.json .
```

The above example runs the Vanilla image on a 16GiB disk and calls the
command `elemental reset` to deploy the main OS and once deployed an image
is created from the running instance.

### Available variables for customization

All the customizable variables are listed in `packer/variables.pkr.hcl`, 
variables with the  `azure_` prefix are the ones related to the Azure Packer
template. These are some of the relevant ones:

* `azure_cos_deploy_args`: This the command that will be executed once the
  Vanilla image booted. In this stage it is expected that user sets a command
  to install the desired cOS or derivative image. By default it is set to
  `elemental reset` which will deploy the cOS image from the recovery partition.
  To deploy custom derivatives something like
  `elemental reset --docker-image <my-derivative-img-ref>` should be sufficient.
  
* `azure_custom_managed_image_name`: Name of a custom managed image to use for your 
  base image.
  
* `azure_custom_managed_image_resource_group_name`: Name of a custom managed image's 
  resource group to use for your base image.

* `azure_os_disk_size_gb`: This sets the disk size of the VM that Packer
  launches for the build. During Vanilla image first boot the system will
  expand to the disk geometry. The layout is configurable with the user-data.

* `azure_vm_size`: Sets the size of the instance being launched. Defaults to Standard_B2s

* `azure_user_data_file`: This sets the user-data file that will be used for the
  azure instance during the build process. It defaults to `user-data/azure.yaml` and
  the default file basically includes the disk expansion configuration. It
  adds a `COS_STATE` partition that should be big enough to store about three times
  the size of the image to deploy. Then it also creates a `COS_PERSISTENT`
  partition with all the rest of the available space in disk.

{{% pageinfo %}}
The current version of packer (1.7.4) doesn't have any support for user-data so currently is
not possible to automate the deployment with packer correctly.
Have a look at [packer changelog](https://github.com/hashicorp/packer/blob/master/CHANGELOG.md) to be informed when
user-data is supported.
{{% /pageinfo %}}
