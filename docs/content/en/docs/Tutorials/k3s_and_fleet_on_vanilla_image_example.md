
---
title: "K3s + Fleet"
linkTitle: "K3s and Fleet"
weight: 3
date: 2021-01-05
description: >
  Running k3s and Fleet on a cOS vanilla raw image
---

This is a work in progress example of how to deploy K3S + Fleet + System Uprade Controller over a cOS vanilla image only
by using cloud-init yaml configuration files. The config file reproduced here is meant to be included
as a user-data in a cloud provider (aws, gcp, azure, etc) or as part of a cdrom (cOS-Recovery will try to fetch `/userdata` file
from a cdrom device).

A vanilla image is an image that only provides the cOS-Recovery system on a `COS_RECOVERY` partition. It does not include any other
system and it is meant to be dumped to a bigger disk and deploy a cOS system or a derivative system over the free space in disk.
COS vanilla images are build as part of the CI workflow, see CI artifacts to download one of those.

The configuration file of this example has two purposes: first it deploys cOS, second in reboots on the deployed OS and deploys
K3S + Fleet + System Upgrades Controller.

On first boot it will fail to boot cOS grub menu entry and fallback
to cOS-Recovery system. From there it will partition the vanilla image to create the main system partition (`COS_STATE`)
and add an extra partition for persistent data (`COS_PERSISTENT`). It will use the full disk, a disk of at least 20GiB
is recommended. After partitioning it will deploy the main system on `COS_STATE` and reboot to it.

On consequent boots it will simply boot from `COS_STATE`, there it prepares the persistent areas of the system (arranges few bind
mounts inside `COS_PERSISTENT`) and then it runs an standard installation of K3s, Fleet and System Upgrade Controller. After few
minutes after the system is up the K3s cluster is up and running.

Note this setup similar to the [derivative example](https://github.com/rancher-sandbox/cos-fleet-upgrades-sample) using Fleet.
The main difference is that this example does not require to build any image, it is pure cloud-init configuration based.

### User data configuration file
```yaml
name: "Default deployment"
stages:
   rootfs.after:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Repart image"
       layout:
         # It will partition a device including the given filesystem label or part label (filesystem label matches first)
         device:
           label: COS_RECOVERY
         add_partitions:
           - fsLabel: COS_STATE
             # 15Gb for COS_STATE, so the disk should have, at least, 20Gb
             size: 15360
             pLabel: state
           - fsLabel: COS_PERSISTENT
             # unset size or 0 size means all available space
             pLabel: persistent
     - if: '[ ! -f "/run/cos/recovery_mode" ]'
       name: "Persistent state"
       environment_file: /run/cos/cos-layout.env
       environment:
         VOLUMES: "LABEL=COS_OEM:/oem LABEL=COS_PERSISTENT:/usr/local"
         OVERLAY: "tmpfs:25%"
         RW_PATHS: "/var /etc /srv"
         PERSISTENT_STATE_PATHS: "/root /opt /home /var/lib/rancher /var/lib/kubelet /etc/systemd /etc/rancher /etc/ssh"
   network.before:
     - name: "Setup SSH keys"
       authorized_keys:
         root:
         # It can download ssh key from remote places, such as github user keys (e.g. `github:my_user`)
         - my_custom_ssh_key
     - if: '[ ! -f "/run/cos/recovery_mode" ]'
       name: "Fleet deployment"
       files:
       - path: /etc/k3s/manifests/fleet-config.yaml
         content: |
              apiVersion: helm.cattle.io/v1
              kind: HelmChart
              metadata:
                name: fleet-crd
                namespace: kube-system
              spec:
                chart: https://github.com/rancher/fleet/releases/download/v0.3.3/fleet-crd-0.3.3.tgz
              ---
              apiVersion: helm.cattle.io/v1
              kind: HelmChart
              metadata:
                name: fleet
                namespace: kube-system
              spec:
                chart: https://github.com/rancher/fleet/releases/download/v0.3.3/fleet-0.3.3.tgz
   network:
     - if: '[ -f "/run/cos/recovery_mode" ]'
       name: "Deploy cos-system"
       commands:
         # Deploys the recovery image.
         # use --docker-image to deploy a custom image
         # e.g. `elemental reset --docker-image quay.io/my_custom_repo:my_image`
         - elemental reset --reboot
     - if: '[ ! -f "/run/cos/recovery_mode" ]'
       name: "Setup k3s"
       directories:
       - path: "/usr/local/bin"
         permissions: 0755
         owner: 0
         group: 0
       commands:
       - |
            curl -sfL https://get.k3s.io | \
            INSTALL_K3S_VERSION="v1.20.4+k3s1" \
            INSTALL_K3S_EXEC="--tls-san {{.Values.node.hostname}}" \
            INSTALL_K3S_SELINUX_WARN="true" \
            sh -
            # Install fleet 
            kubectl apply -f /etc/k3s/manifests/fleet-config.yaml
            # Install system-upgrade-controller
            kubectl apply -f https://raw.githubusercontent.com/rancher/system-upgrade-controller/v0.6.2/manifests/system-upgrade-controller.yaml
```
