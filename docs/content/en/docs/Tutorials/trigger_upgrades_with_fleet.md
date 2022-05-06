
---
title: "Trigger upgrades with K3s and Fleet"
linkTitle: "Trigger upgrades with K3s and Fleet"
weight: 3
date: 2021-01-05
description: >
  Using fleet to trigger upgradeson cOS based derivatives
---

![](https://docs.google.com/drawings/d/e/2PACX-1vQPv9TI3D95vocG7oCHmVmNuuvBYuc2_0kaxAc6xnCBM9mFTnUTFIIDkzZKUFFP-xyw2Hg4q9XhxLD8/pub?w=1185&h=712)

In this tutorial we will:

1) Build a custom OS image to deploy in our cluster
2) Setup a cluster with cOS, k3s and fleet
3) Upgrade the cluster to our custom OS image with fleet

[This repository](https://github.com/rancher-sandbox/cos-fleet-upgrades-sample/) contains the full example code.

## 1) Build the OS image

```bash
# IMAGE=quay.io/costoolkit/test-images:fleet-sample
# cd os
# docker build -t $IMAGE .
```

## 2) Push the docker image


```bash
# docker push $IMAGE
```

## 3) Prepare a cOS VM

Download an ISO, or a qcow image from the Github artifacts of cOS. Or generate an iso of the image (check [here](https://github.com/mudler/os2) for another example). 

If deploying on AWS/openstack/Cloud, use the `fleet-cloud-init.yaml` file as userdata. If deploying on baremetal/VMs, place `fleet-cloud-init.yaml` in `/oem` after install (or run the installer with `elemental install --cloud-init https://raw.githubusercontent.com/rancher-sandbox/cos-fleet-upgrades-sample/main/fleet-cloud-init.yaml $DEVICE`).

Reboot, after some bootstraping time (check until all pods are running with `watch kubectl get pods -A`), you should have a k3s cluster with fleet and [system-upgrade-controller](https://github.com/rancher/system-upgrade-controller) deployed. 

## 4) Upgrade with fleet

Add your fleet repository to the fleet cluster:

```bash
cat > example.yaml << "EOF"
apiVersion: fleet.cattle.io/v1alpha1
kind: GitRepo
metadata:
  name: upgrade
  # This namespace is special and auto-wired to deploy to the local cluster
  namespace: fleet-local
spec:
  # Everything from this repo will be ran in this cluster. You trust me right?
  repo: "https://github.com/rancher-sandbox/cos-fleet-upgrades-sample"
  branch: "main"
  paths:
  - manifests
EOF

kubectl apply -f example.yaml
```

An example of how to trigger an upgrade with fleet is in `manifests/upgrade.yaml`. Edit the image with the one generated in the previous steps, and commit it to your **fleet repository**, At this point you should see the upgrade job to kick-in, the system will reboot afterwards.

