---
title: "Building images with Packer"
linkTitle: "Building images with Packer"
weight: 8
date: 2017-01-05
description: >
  Building VBox, OVA, QEMU, etc. images of your derivative with Packer
---

{{<image_center image="https://docs.google.com/drawings/d/e/2PACX-1vTOn0fYAI589csRoONln7aCttWWNyUc2DvEUt9Dw6tys4nzyearrIUaCXYpg6lknli16z_Kz1N-ugxK/pub?w=507&h=217">}}

For Vagrant Boxes, OVA and QEMU images at the moment we are relying on [packer templates](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/packer). 

The cOS vanilla images can be used as input to Packer to deploy pristine images of the user container image with [elemental reset](../../getting-started/deploy). 
