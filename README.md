# containerOS toolkit

containerOS (**cOS**) is a toolkit to build, ship and maintain cloud-init driven Linux derivatives based on container images with a common featureset. 

It is designed to reduce the maintenance surface, with a flexible approach to provide upgrades from container registries. It is cloud-init driven and also designed to be adaptive-first, allowing easily to build changes on top.

<!-- TOC -->

- [containerOS toolkit](#containeros-toolkit)
    - [In a nutshell](#in-a-nutshell)
    - [Design goals](#design-goals)
    - [Quick start](#quick-start)
        - [Build cOS Locally](#build-cos-locally)
    - [First steps](#first-steps)
    - [References](#references)
    - [License](#license)

<!-- /TOC -->

## In a nutshell

cOS derivatives are built from containers, and completely hosted on image registries. The build process results in a single container image used to deliver regular upgrades in OTA approach. Each derivative built with `cos-toolkit` inherits by default the [following featuresets](/docs/derivatives_featureset.md).

cOS supports different release channels, all the final and cache images used are tagged and pushed regularly [to DockerHub](https://hub.docker.com/r/raccos/releases-amd64/) and can be pulled for inspection from the registry as well.

Those are exactly the same images used during upgrades, and can also be used to build Linux derivatives from cOS.

For example, if you want to see locally what's in cOS 0.4.30, you can:

```bash
$ docker run -ti --rm raccos/releases-opensuse:cos-system-0.4.30 /bin/bash
```

cOS Images are signed, and during upgrades Docker Content Trust is enabled.

You can inspect the images signatures for each version:

```bash
$ docker trust inspect raccos/releases-opensuse:cos-system-0.4.32
```

## Design goals

- A Manifest for container-based OS. It contains just the common bits to make a container image bootable and to be upgraded from, with few customization on top
- Immutable-first, but with a flexible layout
- Cloud-init driven
- Based on systemd
- Built and upgraded from containers - It is a [single image OS](https://hub.docker.com/r/raccos/releases-opensuse/)!
- OTA updates
- Easy to customize
- Cryptographically verified

## Quick start

cOS releases consist on container images that can be used to build derived against. 
cOS is a manifest which assembles an OS from containers, so if you want to make substantial changes to the layout you can also fork directly cOS.

Currently, the toolkit supports creating derivatives from [OpenSUSE, Fedora and Ubuntu](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/values), although it's rather simple to add support for other OS families and architecures.

The cOS CI generates ISO and images artifacts used for testing, so you can also try out cOS by downloading the 
ISO [from the Github Actions page](https://github.com/rancher-sandbox/cOS-toolkit/actions/workflows/build.yaml), to the commit you are interested into.

### Build cOS Locally

The starting point to use cos-toolkit is to see it in action with our [sample repository](https://github.com/rancher-sandbox/cos-toolkit-sample-repo).

The only requirement to build derivatives is docker installed, see [Development notes](/docs/dev.md) for more details on how to build `cos` instead.

## First steps

The [sample repository](https://github.com/rancher-sandbox/cos-toolkit-sample-repo) contains the definitions of a [SampleOS](https://github.com/rancher-sandbox/cos-toolkit-sample-repo/tree/master/packages/sampleOS) boilerplate, which results in an immutable single-image distro and a [simple HTTP service on top](https://github.com/rancher-sandbox/cos-toolkit-sample-repo/tree/master/packages/sampleOSService) that gets started on boot.

To give it a quick shot, it's as simple as cloning the [Github repository](https://github.com/rancher-sandbox/cos-toolkit-sample-repo), and running cos-build:

```bash
$ git clone https://github.com/rancher-sandbox/cos-toolkit-sample-repo
$ cd cos-toolkit-sample-repo
$ source .envrc
$ cos-build
```

This command will build a container image which contains the required dependencies to build the custom OS, and will later be used to build the OS itself. The result will be a set of container images and an ISO which you can boot with your environment of choice. 


## References

- [High Level architecture](/docs/high_level_architecture.md)
- [Github project](https://github.com/mudler/cOS/projects/1) for a short-term Roadmap
- [Development notes](/docs/dev.md)
- [Sample repository](https://github.com/rancher-sandbox/cos-toolkit-sample-repo)


## License

Copyright (c) 2020-2021 [SUSE, LLC](http://suse.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.