variable "accelerator" {
  type    = string
  default = "kvm"
}

variable "arch" {
  type    = string
  default = "amd64"
}

variable "aws_access_key" {
  type = string
  default = env("AWS_ACCESS_KEY_ID")
  sensitive = true
}

variable "aws_secret_key" {
  type    = string
  default = env("AWS_SECRET_ACCESS_KEY")
  sensitive = true
}

variable "aws_region" {
  type    = string
  default = env("AWS_DEFAULT_REGION")
}

variable "aws_ami_groups" {
  type = list(string)
  default = ["all"]
  sensitive = true
  description = "A list of groups that have access to launch the resulting AMI(s). By default no groups have permission to launch the AMI. all will make the AMI publicly accessible."
}

variable "aws_instance_type" {
  type = string
  default = "t3.small"
  description = "Instance type to build the AMI on"
}

variable "aws_source_ami_filter_name" {
  type = string
  default = "*cOS*Vanilla*"
  description = "Name to search for a base ami to build upon the new AMI. Accepts regex and will default to the latest AMI found with that name"
}

variable "aws_source_ami_filter_owners" {
  type = list(string)
  default = ["self"]
  description = "Filters the AMIs by their owner. You may specify one or more AWS account IDs or 'self' (which will use the account whose credentials you are using to run Packer)"
}

variable "aws_source_ami_filter_root-device-type" {
  type = string
  default = "ebs"
  description = "Type of root device type to filter the search for the base AMI"
}

variable "aws_source_ami_filter_virtualization-type" {
  type = string
  default = "hvm"
  description = "Type of virtualization type to filter the search for the base AMI"
}


variable "aws_cos_install_args" {
  type = string
  default = "cos-deploy"
  description = "Arguments to execute while provisioning the aws ami with packer. This will use the shell provisioner"
}

variable "aws_launch_volume_name" {
  type = string
  default = "/dev/sda1"
  description = "Launch block device number to configure. Usually /dev/sda1. Check https://www.packer.io/docs/builders/amazon/ebs#block-devices-configuration for full details"
}

variable "aws_launch_volume_size" {
  type = number
  default = 15
  description = "Size for the launch block device. This has be be at least 2Gb (recovery) + the size of COS_STATE partition set in the user data"
}

variable "aws_user_data_file" {
  type = string
  default = "aws/setup-disk.yaml"
  description = "Path to the user-data file to boot the base AMI with"
}

variable "cos_version" {
  type    = string
  default = "latest"
}

variable "build" {
  type    = string
  default = "dev"
}

variable "cpus" {
  type    = string
  default = "3"
}

variable "disk_size" {
  type    = string
  default = "50000"
}

variable "feature" {
  type    = string
  default = ""
}

variable "flavor" {
  type    = string
  default = "opensuse"
}

variable "iso" {
  type    = string
  default = ""
}

variable "memory" {
  type    = string
  default = "8192"
}

variable "root_password" {
  type    = string
  default = "cos"
}

variable "root_username" {
  type    = string
  default = "root"
}

variable "sleep" {
  type    = string
  default = "30s"
}

variable "name" {
  type = string
  default = "cOS"
  description = "Name of the product being built. Only used for naming artifacts."
}
