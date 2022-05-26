source "amazon-ebs" "cos" {
  access_key      = var.aws_access_key
  ami_name        = "${var.name}-${var.cos_version}-${var.flavor}"
  ami_description = "${var.name}-${var.cos_version}-${var.flavor}"
  ami_groups      = var.aws_ami_groups
  instance_type   = var.aws_instance_type
  region          = var.aws_region
  secret_key      = var.aws_secret_key
  ssh_password    = var.root_password
  ssh_username    = var.root_username
  temporary_security_group_source_cidrs = [var.aws_temporary_security_group_source_cidr ]
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
    delete_on_termination = var.aws_launch_volume_delete_on_terminate
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
  custom_managed_image_resource_group_name = var.azure_custom_managed_image_resource_group_name
  custom_managed_image_name = var.azure_custom_managed_image_name
  managed_image_name = "${var.name}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${var.flavor}-${var.arch}"
  managed_image_resource_group_name = var.azure_managed_image_resource_group_name
  user_data_file = var.azure_user_data_file
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

source "googlecompute" "cos" {
  project_id                = var.gcp_project_id
  source_image_family       = var.gcp_source_image_family
  ssh_password              = var.root_password
  ssh_username              = var.root_username
  zone                      = var.gcp_location
  disk_size                 = var.gcp_disk_size
  enable_secure_boot        = false
  image_name                = "${lower(var.name)}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${substr(var.git_sha, 0, 7)}-${var.flavor}-${var.arch}"
  image_description         = "${var.name}-${replace(var.cos_version, "+", "-")}-${formatdate("DDMMYYYY", timestamp())}-${substr(var.git_sha, 0, 7)}-${var.flavor}-${var.arch}"
  image_labels = {
    name          = "${lower(var.name)}"
    version       = var.cos_version
    flavor        = var.flavor
    git_sha       = var.git_sha  # use full sha here
  }
  image_storage_locations   = [var.gcp_image_storage_location]
  machine_type              = var.gcp_machine_type
  metadata_files = {
    user-data = var.gcp_user_data_file
  }
}

source "qemu" "cos" {
  accelerator            = "${var.accelerator}"
  boot_wait              = "${var.sleep}"
  cpus                   = "${var.cpus}"
  firmware               = "${var.firmware}"
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

source "qemu" "cos-arm64" {
  qemu_binary            = "qemu-system-aarch64"
  machine_type           = "virt"
  accelerator            = "${var.accelerator}"
  boot_wait              = "${var.sleep}"
  cpus                   = "${var.cpus}"
  memory                 = "${var.memory}"
  disk_interface         = "virtio-scsi"
  firmware               = "${var.firmware}"
  cdrom_interface        = "virtio-scsi"
  disk_size              = "${var.disk_size}"
  format                 = "qcow2"
  headless               = true
  iso_checksum           = "none"
  iso_url                = "${var.iso}"
  qemuargs               = [
    ["-boot", "menu=on,strict=on"], # Override the default packer -boot flag which is not valid on UEFI
    [ "-device", "virtio-scsi-pci" ], # Add virtio scsi device
    [ "-device", "scsi-cd,drive=cdrom0,bootindex=0" ], # Set the boot index to the cdrom, otherwise UEFI wont boot from CD
    [ "-device", "scsi-hd,drive=drive0,bootindex=1" ], # Set the boot index to the cdrom, otherwise UEFI wont boot from CD
    [ "-drive", "if=none,file=${var.iso},id=cdrom0,media=cdrom" ], # attach the iso image
    [ "-drive", "if=none,file=output-cos-arm64/${var.name},id=drive0,cache=writeback,discard=ignore,format=qcow2" ], # attach the destination disk
    ["-cpu", "cortex-a57"],
  ]
  shutdown_command       = "shutdown -hP now"
  ssh_handshake_attempts = "20"
  ssh_password           = "${var.root_password}"
  ssh_timeout            = "5m"
  ssh_username           = "${var.root_username}"
  vm_name                = "${var.name}"
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
  vboxmanage = [
    ["modifyvm", "{{.Name}}", "--recording", "${var.enable_video_capture}", "--recordingscreens", "0","--recordingfile", "../capture.webm"],
  ]
}

build {
  description = "cOS"

  sources = ["source.amazon-ebs.cos", "source.qemu.cos", "source.qemu.cos-arm64", "source.virtualbox-iso.cos", "source.azure-arm.cos", "source.googlecompute.cos"]

  source "source.qemu.cos" {
    name = "cos-squashfs"
  }

  source "source.qemu.cos-arm64" {
    name = "cos-arm64-squashfs"
  }

  source "source.virtualbox-iso.cos" {
    name = "cos-squashfs"
  }

  provisioner "file" {
    only = ["virtualbox-iso.cos-squashfs", "qemu.cos-squashfs", "qemu.cos-arm64-squashfs"]
    destination = "/etc/elemental/config.d/squashed_recovery.yaml"
    source      = "squashed_recovery.yaml"
  }

  provisioner "file" {
    except = ["amazon-ebs.cos", "azure-arm.cos", "googlecompute.cos"]
    destination = "/90_custom.yaml"
    source      = "config.yaml"
  }

  provisioner "file" {
    except = ["amazon-ebs.cos", "azure-arm.cos", "googlecompute.cos"]
    destination = "/vagrant.yaml"
    source      = "vagrant.yaml"
  }

  provisioner "shell" {
    except = ["amazon-ebs.cos", "azure-arm.cos", "googlecompute.cos"]
    inline = ["INTERACTIVE=false elemental install --cloud-init /90_custom.yaml /dev/sda",
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
    only = ["googlecompute.cos"]
    inline = [
      "${var.gcp_cos_deploy_args}",
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
    only   = ["virtualbox-iso.cos", "virtualbox-iso.cos-squashfs"]
    output = "cOS-Packer-${var.flavor}-${var.build}-vbox-${var.arch}.box"
  }

  post-processor "vagrant" {
    only   = ["qemu.cos", "qemu.cos-arm64", "qemu.cos-squashfs", "qemu.cos-arm64-squashfs"]
    output = "cOS-Packer-${var.flavor}-${var.build}-QEMU-${var.arch}.box"
  }

  post-processor "compress" {
    only   = ["virtualbox-iso.cos", "virtualbox-iso.cos-squashfs"]
    output = "cOS-Packer-${var.flavor}-${var.build}-vbox-${var.arch}.tar.gz"
  }

  post-processor "compress" {
    only   = ["qemu.cos", "qemu.cos-arm64", "qemu.cos-squashfs", "qemu.cos-arm64-squashfs"]
    output = "cOS-Packer-${var.flavor}-${var.build}-QEMU-${var.arch}.tar.gz"
  }
}
