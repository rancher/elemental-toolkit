source "amazon-ebs" "cos" {
  access_key    = var.aws_access_key
  ami_name      = "cos-${var.cos_version}-${formatdate("DDMMYYYY", timestamp())}"
  ami_description = "cos-${var.cos_version}-${formatdate("DDMMYYYY", timestamp())}"
  instance_type = "t3.small"
  region       = var.aws_region
  secret_key   = var.aws_secret_key
  ssh_password = "cos"
  ssh_username = "root"
  source_ami_filter {
    filters = {
      name                = "*cos*recovery*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["self"]
  }
  launch_block_device_mappings {
    device_name = "/dev/sda1"
    volume_size = 15
  }
  user_data_file = "aws/setup-disk.yaml"
  tags = {
    Name    = "cOS"
    Version = var.cos_version
    Base_AMI_ID = "{{ .SourceAMI }}"
    Base_AMI_Name = "{{ .SourceAMIName }}"
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

# a build block invokes sources and runs provisioning steps on them. The
# documentation for build blocks can be found here:
# https://www.packer.io/docs/templates/hcl_templates/blocks/build
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
