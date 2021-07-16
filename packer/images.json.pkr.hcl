source "amazon-ebs" "cos" {
  access_key      = var.aws_access_key
  ami_name        = "${var.name}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${substr(var.git_sha, 0, 7)}-${var.flavor}"
  ami_description = "${var.name}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${substr(var.git_sha, 0, 7)}-${var.flavor}"
  ami_groups      = var.aws_ami_groups
  instance_type   = var.aws_instance_type
  region          = var.aws_region
  secret_key      = var.aws_secret_key
  ssh_password    = var.root_password
  ssh_username    = var.root_username
  source_ami_filter {
    filters = {
      name                = var.aws_source_ami_filter_name
      root-device-type    = var.aws_source_ami_filter_root-device-type
      virtualization-type = var.aws_source_ami_filter_virtualization-type
    }
    most_recent = true
    owners      = var.aws_source_ami_filter_owners
  }
  launch_block_device_mappings {
    device_name = var.aws_launch_volume_name
    volume_size = var.aws_launch_volume_size
  }
  user_data_file = var.aws_user_data_file
  tags = {
    Name          = var.name
    Version       = var.cos_version
    Flavor        = var.flavor
    Git_SHA       = var.git_sha  # use full sha here
    Base_AMI_ID   = "{{ .SourceAMI }}"  # This info comes from the build process directly
    Base_AMI_Name = "{{ .SourceAMIName }}"  # This info comes from the build process directly
  }
}

source "azure-arm" "cos" {
  client_id = var.azure_client_id
  tenant_id = var.azure_tenant_id
  client_secret = var.azure_client_secret
  subscription_id = var.azure_subscription_id
  shared_image_gallery {
    subscription = var.azure_shared_image_gallery_subscription
    resource_group = var.azure_shared_image_gallery_resource_group
    gallery_name = var.azure_shared_image_gallery_gallery_name
    image_name = var.azure_shared_image_gallery_image_name
    image_version = var.azure_shared_image_gallery_image_version
  }
  managed_image_name = "${var.name}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${var.flavor}"
  managed_image_resource_group_name = var.azure_managed_image_resource_group_name
  # User data not supported in the current released version of packer azure plugin
  # also, no user-data -> no deployment
  //user_data_file = var.azure_user_data_file
  location = var.azure_location
  os_type = "Linux"
  os_disk_size_gb = var.azure_os_disk_size_gb
  vm_size = var.azure_vm_size
  communicator = "ssh"
  # root username is not allowed!
  ssh_username = "packer"
  ssh_password = "cos"
  azure_tags = {
    name = var.name
    version = var.cos_version
  }
}

source "qemu" "cos" {
  accelerator            = "${var.accelerator}"
  boot_wait              = "${var.sleep}"
  cpus                   = "${var.cpus}"
  disk_interface         = "ide"
  disk_size              = "${var.disk_size}"
  format                 = "qcow2"
  headless               = true
  iso_checksum           = "none"
  iso_url                = "${var.iso}"
  qemuargs               = [["-m", "${var.memory}M"]]
  shutdown_command       = "shutdown -hP now"
  ssh_handshake_attempts = "20"
  ssh_password           = "${var.root_password}"
  ssh_timeout            = "5m"
  ssh_username           = "${var.root_username}"
  vm_name                = "cOS"
}

source "virtualbox-iso" "cos" {
  boot_wait              = "${var.sleep}"
  cpus                   = "${var.cpus}"
  disk_size              = "${var.disk_size}"
  format                 = "ova"
  guest_additions_mode   = "disable"
  guest_os_type          = "cOS"
  headless               = true
  iso_checksum           = "none"
  iso_url                = "${var.iso}"
  memory                 = "${var.memory}"
  shutdown_command       = "shutdown -hP now"
  ssh_handshake_attempts = "20"
  ssh_password           = "${var.root_password}"
  ssh_timeout            = "5m"
  ssh_username           = "${var.root_username}"
  vm_name                = "cOS"
}

build {
  description = "cOS"

  sources = ["source.amazon-ebs.cos", "source.qemu.cos", "source.virtualbox-iso.cos", "source.azure-arm.cos"]

  provisioner "file" {
    except = ["amazon-ebs.cos", "azure-arm.cos"]
    destination = "/90_custom.yaml"
    source      = "config.yaml"
  }

  provisioner "file" {
    except = ["amazon-ebs.cos", "azure-arm.cos"]
    destination = "/vagrant.yaml"
    source      = "vagrant.yaml"
  }

  provisioner "shell" {
    except = ["amazon-ebs.cos", "azure-arm.cos"]
    inline = ["INTERACTIVE=false cos-installer --config /90_custom.yaml /dev/sda",
      "if [ -n \"${var.feature}\" ]; then mount /dev/disk/by-label/COS_OEM /oem; cos-feature enable ${var.feature}; fi"
    ]
    pause_after = "30s"
  }

  provisioner "shell" {
    only = ["amazon-ebs.cos"]
    inline = [
      "${var.aws_cos_deploy_args}",
      "sync"
    ]
    pause_after = "30s"
  }

  provisioner "shell" {
    only = ["azure-arm.cos"]
    inline = [
      "${var.azure_cos_deploy_args}",
      "sync"
    ]
    pause_after = "30s"
  }

  post-processor "vagrant" {
    only   = ["virtualbox-iso.cos", "qemu.cos"]
    output = "cOS_${var.build}_${var.arch}_${var.flavor}.box"
  }
  post-processor "compress" {
    only   = ["virtualbox-iso.cos", "qemu.cos"]
    output = "cOS_${var.build}_${var.arch}_${var.flavor}.tar.gz"
  }
}
