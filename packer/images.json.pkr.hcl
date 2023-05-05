source "amazon-ebs" "elemental" {
  access_key      = var.aws_access_key
  ami_name        = "${var.name}-${var.flavor}"
  ami_description = "${var.name}-${var.flavor}"
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
    Flavor        = var.flavor
    Git_SHA       = var.git_sha  # use full sha here
    Base_AMI_ID   = "{{ .SourceAMI }}"  # This info comes from the build process directly
    Base_AMI_Name = "{{ .SourceAMIName }}"  # This info comes from the build process directly
  }
}

source "azure-arm" "elemental" {
  client_id = var.azure_client_id
  tenant_id = var.azure_tenant_id
  client_secret = var.azure_client_secret
  subscription_id = var.azure_subscription_id
  custom_managed_image_resource_group_name = var.azure_custom_managed_image_resource_group_name
  custom_managed_image_name = var.azure_custom_managed_image_name
  managed_image_name = "${var.name}-${formatdate("DDMMYYYY", timestamp())}-${var.flavor}-${var.arch}"
  managed_image_resource_group_name = var.azure_managed_image_resource_group_name
  user_data_file = var.azure_user_data_file
  location = var.azure_location
  os_type = "Linux"
  os_disk_size_gb = var.azure_os_disk_size_gb
  vm_size = var.azure_vm_size
  communicator = "ssh"
  # root username is not allowed!
  ssh_username = "packer"
  ssh_password = "elemental"
  azure_tags = {
    name = var.name
  }
}

source "googlecompute" "elemental" {
  project_id                = var.gcp_project_id
  source_image_family       = var.gcp_source_image_family
  ssh_password              = var.root_password
  ssh_username              = var.root_username
  zone                      = var.gcp_location
  disk_size                 = var.gcp_disk_size
  enable_secure_boot        = false
  image_name                = "${lower(var.name)}-${formatdate("DDMMYYYY", timestamp())}-${substr(var.git_sha, 0, 7)}-${var.flavor}-${var.arch}"
  image_description         = "${var.name}-${formatdate("DDMMYYYY", timestamp())}-${substr(var.git_sha, 0, 7)}-${var.flavor}-${var.arch}"
  image_labels = {
    name          = "${lower(var.name)}"
    flavor        = var.flavor
    git_sha       = var.git_sha  # use full sha here
  }
  image_storage_locations   = [var.gcp_image_storage_location]
  machine_type              = var.gcp_machine_type
  metadata_files = {
    user-data = var.gcp_user_data_file
  }
}

source "qemu" "elemental-x86_64" {
  qemu_binary            = "qemu-system-x86_64"
  accelerator            = "${var.accelerator}"
  machine_type           = "${var.machine_type}"
  cpu_model              = "host"
  boot_wait              = "${var.sleep}"
  cpus                   = "${var.cpus}"
  memory                 = "${var.memory}"
  firmware               = "${var.firmware}"
  disk_interface         = "virtio-scsi"
  disk_size              = "${var.disk_size}"
  format                 = "qcow2"
  headless               = true
  iso_checksum           = "${var.iso_checksum}"
  iso_url                = "${var.iso}"
  output_directory       = "build"
  shutdown_command       = "shutdown -hP now"
  ssh_handshake_attempts = "20"
  ssh_password           = "${var.root_password}"
  ssh_timeout            = "5m"
  ssh_username           = "${var.root_username}"
  vm_name                = "${var.name}-${var.flavor}.${var.arch}.qcow2"
  qemuargs               = [
    ["-serial", "file:serial.log"],
    ["-drive", "if=pflash,format=raw,readonly=on,file=${var.firmware}"],
    ["-drive", "if=none,file=build/elemental-${var.flavor}.${var.arch}.qcow2,id=drive0,cache=writeback,discard=ignore,format=qcow2"],
    ["-drive", "file=${var.iso},media=cdrom"],
  ]
}

source "qemu" "elemental-arm64" {
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
  iso_checksum           = "${var.iso_checksum}"
  iso_url                = "${var.iso}"
  qemuargs               = [
    ["-boot", "menu=on,strict=on"], # Override the default packer -boot flag which is not valid on UEFI
    [ "-device", "virtio-scsi-pci" ], # Add virtio scsi device
    [ "-device", "scsi-cd,drive=cdrom0,bootindex=0" ], # Set the boot index to the cdrom, otherwise UEFI wont boot from CD
    [ "-device", "scsi-hd,drive=drive0,bootindex=1" ], # Set the boot index to the cdrom, otherwise UEFI wont boot from CD
    [ "-drive", "if=none,file=${var.iso},id=cdrom0,media=cdrom" ], # attach the iso image
    [ "-drive", "if=none,file=build/elemental-${var.flavor}.${var.arch}.qcow2,id=drive0,cache=writeback,discard=ignore,format=qcow2"],
    ["-cpu", "cortex-a57"],
    ["-serial", "file:serial.log"],
  ]
  shutdown_command       = "shutdown -hP now"
  ssh_handshake_attempts = "20"
  ssh_password           = "${var.root_password}"
  ssh_timeout            = "5m"
  ssh_username           = "${var.root_username}"
  vm_name                = "${var.name}-${var.flavor}.${var.arch}.qcow2"
}

source "virtualbox-iso" "elemental" {
  boot_wait              = "${var.sleep}"
  cpus                   = "${var.cpus}"
  disk_size              = "${var.disk_size}"
  format                 = "ova"
  guest_additions_mode   = "disable"
  guest_os_type          = "Elemental"
  headless               = true
  iso_checksum           = "${var.iso_checksum}"
  iso_url                = "${var.iso}"
  memory                 = "${var.memory}"
  shutdown_command       = "shutdown -hP now"
  ssh_handshake_attempts = "20"
  ssh_password           = "${var.root_password}"
  ssh_timeout            = "5m"
  ssh_username           = "${var.root_username}"
  vm_name                = "${var.name}"
  vboxmanage = [
    ["modifyvm", "{{.Name}}", "--recording", "${var.enable_video_capture}", "--recordingscreens", "0","--recordingfile", "../capture.webm"],
  ]
}

build {
  description = "elemental"

  sources = ["source.amazon-ebs.elemental", "source.qemu.elemental-x86_64", "source.qemu.elemental-arm64", "source.virtualbox-iso.elemental", "source.azure-arm.elemental", "source.googlecompute.elemental"]

  source "source.qemu.elemental-x86_64" {
    name = "elemental-x86_64-squashfs"
  }

  source "source.qemu.elemental-arm64" {
    name = "elemental-arm64-squashfs"
  }

  source "source.virtualbox-iso.elemental" {
    name = "elemental-squashfs"
  }

  provisioner "file" {
    only = ["virtualbox-iso.elemental-squashfs", "qemu.elemental-x86_64-squashfs", "qemu.elemental-arm64-squashfs"]
    destination = "/etc/elemental/config.d/squashed_recovery.yaml"
    source      = "squashed_recovery.yaml"
  }

  provisioner "file" {
    except = ["amazon-ebs.elemental", "azure-arm.elemental", "googlecompute.elemental"]
    destination = "/90_custom.yaml"
    source      = "config.yaml"
  }

  provisioner "file" {
    except = ["amazon-ebs.elemental", "azure-arm.elemental", "googlecompute.elemental"]
    destination = "/testusr.yaml"
    source      = "testusr.yaml"
  }

  provisioner "shell" {
    except = ["amazon-ebs.elemental", "azure-arm.elemental", "googlecompute.elemental"]
    inline = ["elemental install --debug --cloud-init /90_custom.yaml,/testusr.yaml /dev/sda"]
    pause_after = "30s"
  }

  provisioner "shell" {
    only = ["amazon-ebs.elemental"]
    inline = [
      "${var.aws_elemental_deploy_args}",
      "sync"
    ]
    pause_after = "30s"
  }

  provisioner "shell" {
    only = ["googlecompute.elemental"]
    inline = [
      "${var.gcp_elemental_deploy_args}",
      "sync"
    ]
    pause_after = "30s"
  }

  provisioner "shell" {
    only = ["azure-arm.elemental"]
    inline = [
      "${var.azure_elemental_deploy_args}",
      "sync"
    ]
    pause_after = "30s"
  }
}
