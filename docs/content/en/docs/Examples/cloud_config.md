---
title: "Cloud config examples"
linkTitle: "Cloud config examples"
weight: 3
date: 2017-01-05
description: >
  Useful copy-paste cloud-config settings
---

You can find here examples on how to tweak a system via cloud-config various aspects of an Elemental-toolkit derivative.

The examples are meant to be placed as yaml files under `/oem` ore either `/usr/local/cloud-config`. They can be also given as input cloud-config while calling `elemental install`.

## Networking with NetworkManager

By default all interfaces will get automatically an IP address, however, there are situations where a static IP is desired, or custom configuration to be specified, here you can find some network settings with NetworkManager.

### Additional NIC

Set static IP to an additional NIC:

```yaml
name: "Default network configuration"
stages:
   boot:
     - commands:
       - nmcli dev up eth1
     - name: "Setup network"
       files:
       - path: /etc/sysconfig/network/ifcfg-eth1
         content: |
            BOOTPROTO='static'
            IPADDR='192.168.1.2/24'
         permissions: 0600
         owner: 0
         group: 0
```

### Static IP

Set static IP to default interface:

```yaml
name: "Default network configuration"
stages:
   boot:
     - commands:
       - nmcli dev up eth0
   initramfs:
     - name: "Setup network"
       files:
       - path: /etc/sysconfig/network/ifcfg-eth0
         content: |
            BOOTPROTO='static'
            IPADDR='192.168.1.2/24'
         permissions: 0600
         owner: 0
         group: 0
```

### DHCP

```yaml
name: "Default network configuration"
stages:
   boot:
     - commands:
       - nmcli dev up eth1
   initramfs:
     - name: "Setup network"
       files:
       - path: /etc/sysconfig/network/ifcfg-eth1
         content: |
                  BOOTPROTO='dhcp'
                  STARTMODE='onboot'
         permissions: 0600
         owner: 0
         group: 0
```

## Additional files

### K3s manifests

Add k3s manifests:

```yaml
name: "k3s"
stages:
    network:
     - if: '[ ! -f "/run/cos/recovery_mode" ]'
       name: "Fleet deployment"
       files:
       - path: /var/lib/rancher/k3s/server/manifests/fleet-config.yaml
         content: |
              apiVersion: v1
              kind: Namespace
              metadata:
                name: cattle-system
              ---
              apiVersion: helm.cattle.io/v1
              kind: HelmChart
              metadata:
                name: fleet-crd
                namespace: cattle-system
              spec:
                chart: https://github.com/rancher/fleet/releases/download/v0.3.8/fleet-crd-0.3.8.tgz
              ---
              apiVersion: helm.cattle.io/v1
              kind: HelmChart
              metadata:
                name: fleet
                namespace: cattle-system
              spec:
                chart: https://github.com/rancher/fleet/releases/download/v0.3.8/fleet-0.3.8.tgz
```
