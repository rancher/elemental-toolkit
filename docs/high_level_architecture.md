# cOS toolkit High level Architecture

This page tries to encompass the [`cos-toolkit`](https://github.com/rancher-sandbox/cOS-toolkit) structure and the high level architecture, along with all the involved components.


## Design goals

- Blueprints to build immutable Linux derivatives from container images
- A workflow to maintain, support and deliver custom-OS and upgrades to end systems
- Derivatives have the same “foundation” manifest - easy to customize on top, add packages: `systemd`, `dracut` and `grub` as a foundation stack.
- Upgrades delivered with container registry images ( also workflow with `docker run` && `docker commit` supported! )
<br/>The content of the container image is the system which is booted.


## High level overview

cOS-Toolkit encompasses several components required for building and distributing OS images. [This issue](https://github.com/rancher-sandbox/cOS-toolkit/issues/108) summarize the current state, and how we plan to integrate them in a single CLI to improve the user experience.

cOS-Toolkit is also a manifest, which includes package definitions of how the underlying OS is composed. It forms an abstraction layer, which is then translated to Dockerfiles and built by our CI (optionally) for re-usal. A derivative can be built by parts of the manifest, or reusing it entirely, container images included.
 
![High level overview](https://docs.google.com/drawings/d/e/2PACX-1vQQJOaISPbMxMYU44UT-M3ou9uGYOrzbXCRXMLPU8m7_ie3ke_08xCsyRLkFZJRB4VnzIeobPciEoQv/pub?w=942&h=532)

The fundamental phases can be summarized in the following steps:

- Build packages from container images (and optionally keep build caches)
- Extract artefacts from containers
- Add metadata(s) and create a repository
- (optionally) publish the repository and the artefacts

The developer of the derivative applies a customization layer during build, which is an augmentation layer in the same form of `cos-toolkit` itself. [An example repository is provided](https://github.com/rancher-sandbox/cos-toolkit-sample-repo) that shows how to build a customOS that can be maintained with a container image registry.

## Distribution

The OS delivery mechanism is done via container registries. The developer that wants to provide upgrades for the custom OS will push the resulting container images to the container registry. It will then be used by the installed system to pull upgrades from.

![](https://docs.google.com/drawings/d/e/2PACX-1vQrTArCYgu-iscf29v1sl1sEn2J81AqBpi9D5xpwGKr9uxR2QywoSqCmsSaJLxRRacoRr0Kq40a7jPF/pub?w=969&h=464)

## Upgrade mechanism

There are two different upgrade mechanisms available that can be used from a maintainer perspective: (a) release channels or (b) providing a container image reference ( `e.g. my.registry.com/image:tag` ) [that can be tweaked in the customization phases](https://github.com/rancher-sandbox/cOS-toolkit#default-oem) to achieve the desired effect. 

<!-- WIP -->