source "amazon-ebs" "cos" {
  access_key      = var.aws_access_key
  ami_name        = "${var.name}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${var.flavor}"
  ami_description = "${var.name}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${var.flavor}"
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
    Base_AMI_ID   = "{{ .SourceAMI }}"  # This info comes from the build process directly
    Base_AMI_Name = "{{ .SourceAMIName }}"  # This info comes from the build process directly
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

  sources = ["source.amazon-ebs.cos", "source.qemu.cos", "source.virtualbox-iso.cos"]

  provisioner "file" {
    except = ["amazon-ebs.cos"]
    destination = "/90_custom.yaml"
    source      = "config.yaml"
  }

  provisioner "file" {
    except = ["amazon-ebs.cos"]
    destination = "/vagrant.yaml"
    source      = "vagrant.yaml"
  }

  provisioner "shell" {
    except = ["amazon-ebs.cos"]
    inline = ["INTERACTIVE=false cos-installer --config /90_custom.yaml /dev/sda",
      "if [ -n \"${var.feature}\" ]; then mount /dev/disk/by-label/COS_OEM /oem; cos-feature enable ${var.feature}; fi"
    ]
    pause_after = "30s"
  }

  provisioner "shell" {
    only = ["amazon-ebs.cos"]
    inline = [
      "${var.aws_cos_install_args}",
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
