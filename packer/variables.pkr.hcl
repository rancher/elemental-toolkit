variable "firmware" {
  default = ""
}

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

variable "aws_launch_volume_delete_on_terminate" {
  type = bool
  default = true
  description = "Indicates whether the EBS volume is deleted on instance termination. Check https://www.packer.io/docs/builders/amazon/ebs#block-devices-configuration for full details"
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
  default = "user-data/aws.yaml"
  description = "Path to the user-data file to boot the base AMI with"
}

variable "aws_temporary_security_group_source_cidr" {
  type = string
  default = "0.0.0.0/0"
  description = "A IPv4 CIDR to be authorized access to the instance, when packer is creating a temporary security group."
}

variable "azure_client_id" {
  type = string
  default = env("AZURE_CLIENT_ID")
}

variable "azure_tenant_id" {
  type = string
  default = env("AZURE_TENANT_ID")
}

variable "azure_client_secret" {
  type = string
  default = env("AZURE_CLIENT_SECRET")
}

variable "azure_subscription_id" {
  type = string
  default = env("AZURE_SUBSCRIPTION_ID")
}

variable "azure_custom_managed_image_resource_group_name" {
  type = string
  default = "cos-testing"
}

variable "azure_custom_managed_image_name" {
  type = string
  default = "cos_0.5.7-recovery"
}

variable "azure_managed_image_resource_group_name" {
  type = string
  default = "cos-testing"
}

variable "azure_location" {
  type = string
  default = "westeurope"
}

variable "azure_os_disk_size_gb" {
  type = number
  default = 16
}

variable "azure_vm_size" {
  type = string
  default = "Standard_B2s"
}

variable "azure_user_data_file" {
  type = string
  default = "user-data/azure.yaml"
  description = "Path to the user-data file to boot the base Image with"
}

variable "azure_cos_deploy_args" {
  default = "sudo /usr/bin/elemental reset"
}

variable "aws_cos_deploy_args" {
  type = string
  default = "elemental reset"
  description = "Arguments to execute while provisioning cloud images with packer. This will use the shell provisioner"
}

variable "gcp_source_image_family" {
  type = string
  default = "cos-vanilla"
  description = "Image family to use as source. The lastest of the given family is used"
}

variable "gcp_location" {
  type = string
  default = "europe-west3-a"
  description = "Zone used for the build process"
}

variable "gcp_image_storage_location" {
  type = string
  default = "eu"
  description = "Location for the resulting image"
}

variable "gcp_disk_size" {
  type = number
  default = 16
  description = "Size in of the disk in GB"
}

variable "gcp_machine_type" {
  type = string
  default = "n1-standard-1"
  description = "Instance type to use"
}

variable "gcp_user_data_file" {
  type = string
  default = "user-data/gcp.yaml"
  description = "Path to the user-data file to boot the base instance with"
}

variable "gcp_cos_deploy_args" {
  type = string
  default = "elemental reset"
  description = "Arguments to execute while provisioning cloud images with packer. This will use the shell provisioner"
}

variable "gcp_project_id" {
  type = string
  description = "Project to look for the image family"
  default = env("GCP_PROJECT_ID")
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
  default = "teal"
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

variable "git_sha" {
  type = string
  default ="none"
  description = "Git sha of the current build, defaults to none."
}

variable "enable_video_capture" {
  type = string
  default ="off"
  description = "Enable video capture of the vm(only virtualbox)."
}
