---
title: "Build AMI machines for AWS"
linkTitle: "Build AWS images"
weight: 4
date: 2017-01-05
description: >
  This section documents the procedure to deploy cOS (or derivatives) images
  in AWS public cloud provider by using the cOS Vanilla image.
---

![](https://docs.google.com/drawings/d/e/2PACX-1vSqJWcFThP7K2HS551LCqs73l4ZncXElLjlbCvxY96Ga2Jbjnq79j-DEjaccUZvYEQyphWiDQc9flxk/pub?w=1223&h=691)

Requirements:

* Packer
* AWS access keys with the appropriate roles and permissions
* A Vanilla AMI
* [Packer templates](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packer)

The suggested approach is based on using Packer templates to customize the
deployment and automate the upload and publish to AWS of cOS derivatives or cOS itself. For all the details
and possibilties of Packer check the [official documentation](https://www.packer.io/guides/hcl).

## Run the build with Packer

Publishing an AMI image in AWS based on top of the latest cOS Vanilla image is
fairly simple. In fact, it is only needed to set the AWS credentials
and run a `packer build` process to trigger the deployment and register the
resulting snapshot as an AMI. In such case the lastest cOS image will be
deployed and configured with pure defaults. Consider:

```bash
# From the root of a cOS-toolkit repository checkout

> export AWS_ACCESS_KEY_ID=<your_aws_access_key> 
> export AWS_SECRET_ACCESS_KEY=<your_aws_secret_access_key> 
> export AWS_DEFAULT_REGION=<your_aws_default_region>

> cd packer
> packer build -only amazon-ebs.cos .
```

AWS keys can be passed as environment variables as it is above or packer
picks them from aws-cli configuration files (`~/.aws`) if any. Alternatively,
one can define them in the variables file.

The `-only amazon-ebs.cos` flag is just to tell packer which of the sources
to make use for the build. Note the `packer/images.json.pkr.hcl` file defines
few other sources such as `qemu` and `virtualbox`.

## Customize the build with a variables file

The packer template can be customized with the variables defined in
`packer/variables.pkr.hcl`. These are the variables that can be set on run
time using the `-var key=value` or `-var-file=path` flags. The variable file
can be a json file including desired varibles. Consider the following example:

```bash
# From the packer folder of the cOS-toolkit repository checkout

> cat << EOF > test.json
{
    "aws_cos_deploy_args": "elemental reset",
    "aws_launch_volume_size": 16,
    "name": "MyTest"
}
EOF

> packer build -only amazon-ebs.cos -var-file=test.json .
```

The above example runs the AMI Vanilla image on a 16GiB disk and calls the
command `elemental reset` to deploy the main OS, once deployed an snapshot is
created and an AMI out this snapshot is registered in EC2. The created
AMI artifact will be called `MyTest`, the name has no impact in the underlaying
OS.

### Available variables for customization

All the customizable variables are listed in `packer/variables.pkr.hcl`, 
variables with the  `aws_` prefix are the ones related to the AWS Packer
template. These are some of the relevant ones:

* `aws_cos_deploy_args`: This the command that will be executed once the
  Vanilla image booted. In this stage it is expected that user sets a command
  to install the desired cOS or derivative image. By default it is set to
  `elemental reset` which will deploy the cOS image from the recovery partition.
  To deploy custom derivatives something like
  `elemental reset --docker-image <my-derivative-img-ref>` should be sufficient.

* `aws_launch_volume_size`: This sets the disk size of the VM that Packer
  launches for the build. During Vanilla image first boot the system will
  expand to the disk geometry. The layout is configurable with the user-data.

* `aws_user_data_file`: This sets the user-data file that will be used for the
  aws instance during the build process. It defaults to `aws/setup-disk.yaml` and
  the defauklt file basically includes the disk expansion configuration. It
  adds a `COS_STATE` partition that should be big enough to store about three times
  the size of the image to deploy. Then it also creates a `COS_PERSISTENT`
  partition with all the rest of the available space in disk.

* `aws_source_ami_filter_name`: This a filter to choose the AMI image for the
  build process. It defaults to `*cOS*Vanilla*` pattern to pick the latest cOS
  Vanilla image available.

* `aws_temporary_security_group_source_cidr`: A IPv4 CIDR to be authorized access to the instance,
  when packer is creating a temporary security group. Defaults to "0.0.0.0/0".
