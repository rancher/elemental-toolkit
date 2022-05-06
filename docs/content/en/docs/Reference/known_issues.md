---
title: "Known issues"
linkTitle: "Known issues"
weight: 3
date: 2017-01-05
description: >
  This document encompasses known issues while building cOS and cOS derivatives.
---

When building cOS or a cOS derivative, you could face different issues, this section provides a description of the most known ones, and way to workaround them.

### Building SELinux fails

`cOS` by default has SELinux enabled in permissive mode. If you are building parts of cOS or cOS itself from scratch, you might encounter issues while building the SELinux module, like so:

```
Step 12/13 : RUN checkmodule -M -m -o cOS.mod cOS.te && semodule_package -o cOS.pp -m cOS.mod
  ---> Using cache
 ---> 1be520969ead
Step 13/13 : RUN semodule -i cOS.pp
  ---> Running in c5bfa5ae92e2
 libsemanage.semanage_commit_sandbox: Error while renaming /var/lib/selinux/targeted/active to /var/lib/selinux/targeted/previous. (Invalid cross-device link).
semodule:  Failed!
 The command '/bin/sh -c semodule -i cOS.pp' returned a non-zero code: 1
 Error: while resolving join images: failed building join image: Failed compiling system/selinux-policies-0.0.6+3: failed building package image: Could not push image: raccos/sampleos:ffc8618ecbfbffc11cc3bca301cc49867eb7dccb623f951dd92caa10ced29b68 selinux-policies-system-0.0.6+3.dockerfile: Could not build image: raccos/sampleos:ffc8618ecbfbffc11cc3bca301cc49867eb7dccb623f951dd92caa10ced29b68 selinux-policies-system-0.0.6+3.dockerfile: Failed running command: : exit status 1
 Bailing out
make: *** [Makefile:45: build] Error 1
```

The issue is possibly caused by https://github.com/docker/for-linux/issues/480 . A workaround is to switch the storage driver of Docker. Check if your storage driver is overlay2, and switch it to `devicemapper`

### Multi-stage copy build fails

While processing images with several stage copy, you could face the following:


```
 🐋  Building image raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d done
 📦  8/8 system/cos-0.5.3+1 ⤑ 🔨  build system/selinux-policies-0.0.6+3 ✅  Done
 🚀  All dependencies are satisfied, building package requested by the user system/cos-0.5.3+1
 📦  system/cos-0.5.3+1  Using image:  raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d
 📦  system/cos-0.5.3+1 🐋  Generating 'builder' image from raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d as raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed with prelude steps
🚧  warning Failed to download 'raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed'. Will keep going and build the image unless you use --fatal
🚧  warning Failed pulling image: Error response from daemon: manifest for raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed not found: manifest unknown: manifest unknown
: exit status 1
 🐋  Building image raccos/sampleos:builder-8533d659df2505a518860bd010b7a8ed
 Sending build context to Docker daemon  9.728kB
 Step 1/10 : FROM raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d
  ---> f1122e79b17e
Step 2/10 : COPY . /luetbuild
  ---> 4ff3e202951b
 Step 3/10 : WORKDIR /luetbuild
  ---> Running in 7ec571b96c6f
 Removing intermediate container 7ec571b96c6f
  ---> 9e05366f830a
Step 4/10 : ENV PACKAGE_NAME=cos
  ---> Running in 30297dbd21a3
 Removing intermediate container 30297dbd21a3
  ---> 4c4838b629f4
 Step 5/10 : ENV PACKAGE_VERSION=0.5.3+1
  ---> Running in 36361b617252
 Removing intermediate container 36361b617252
  ---> 6ac0d3a2ff9a
Step 6/10 : ENV PACKAGE_CATEGORY=system
  ---> Running in f20c2cf3cf34
 Removing intermediate container f20c2cf3cf34
  ---> a902ff95d273
 Step 7/10 : COPY --from=quay.io/costoolkit/build-cache:f3a333095d9915dc17d7f0f5629a638a7571a01dcf84886b48c7b2e5289a668a /usr/bin/yip /usr/bin/yip
  ---> 42fa00d9c990
 Step 8/10 : COPY --from=quay.io/costoolkit/build-cache:e3bbe48c6d57b93599e592c5540ee4ca7916158461773916ce71ef72f30abdd1 /usr/bin/luet /usr/bin/luet
 e3bbe48c6d57b93599e592c5540ee4ca7916158461773916ce71ef72f30abdd1: Pulling from costoolkit/build-cache
 3599716b36e7:  Already exists
 24a39c0e5d06: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 378615c429f5: Already exists
 c28da22d3dfd:  Already exists
 ddb4dd5c81b0: Already exists
 92db41c0c9ab: Already exists
 4f4fb700ef54: Already exists
 6e0ca71a6514: Already exists
 47debb886c7d: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 4f4fb700ef54: Already exists
 d0c9d0f8ddb6: Already exists
 e5a48f1f72ad:  Pulling fs layer
 4f4fb700ef54:  Pulling fs layer
 7d603b2e4a37:  Pulling fs layer
 64c4d787e344:  Pulling fs layer
 f8835d2e60d1:  Pulling fs layer
 64c4d787e344:  Waiting
 f8835d2e60d1:  Waiting
 e5a48f1f72ad:  Download complete
 e5a48f1f72ad:  Pull complete
 4f4fb700ef54:  Verifying Checksum
 4f4fb700ef54:  Download complete
 4f4fb700ef54:  Pull complete
 7d603b2e4a37: Verifying Checksum
7d603b2e4a37: Download complete
 64c4d787e344: Verifying Checksum
64c4d787e344: Download complete
 7d603b2e4a37: Pull complete
 64c4d787e344: Pull complete
 f8835d2e60d1:  Verifying Checksum
 f8835d2e60d1:  Download complete
 f8835d2e60d1: Pull complete
 Digest: sha256:9b58bed47ff53f2d6cc517a21449cae686db387d171099a4a3145c8a47e6a1e0
 Status: Downloaded newer image for quay.io/costoolkit/build-cache:e3bbe48c6d57b93599e592c5540ee4ca7916158461773916ce71ef72f30abdd1
 failed to export image: failed to create image: failed to get layer sha256:118537d8997a08750ab1ac3d8e8575e40fe60e8337e02633b0d8a1287117fe78: layer does not exist
 Error: while resolving join images: failed building join image: failed building package image: Could not push image: raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d cos-system-0.5.3+1-builder.dockerfile: Could not build image: raccos/sampleos:cc0aee4ff6c194f920a945c45ebcb487c3e22c5ab40e2634ea70c064dfab206d cos-system-0.5.3+1-builder.dockerfile: Failed running command: : exit status 1
 Bailing out
make: *** [Makefile:45: build] Error 1
```

There is a issue open [upstream](https://github.com/moby/moby/issues/37965) about it. A workaround is to enable Docker buildkit with `DOCKER_BUILDKIT=1`.