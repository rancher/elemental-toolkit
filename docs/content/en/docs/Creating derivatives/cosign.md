
---
title: "Cosign"
linkTitle: "Cosign"
weight: 2
date: 2020-11-02
description: >
  How we use cosign in cos-toolkit
---

[Cosign](https://github.com/sigstore/cosign) is a project that signs and verifies containers and stores the signatures on OCI registries.

You can check the cosign [github repo](https://github.com/sigstore/cosign) for more information.

In cos-toolkit we sign every container that we generate as part of our publish process so the signature can be verified during package installation with luet or during deploy/upgrades from a deployed system to verify that the containers have not been altered in any way since their build.

Currently cosign provides 2 methods for signing and verifying.

 - private/public key
 - keyless

We use keyless signatures based on OIDC Identity tokens provided by github, so nobody has access to any private keys and can use them. (For more info about keyless signing/verification check [here](https://github.com/sigstore/cosign/blob/main/KEYLESS.md))

This signature generation is provided by [luet-cosign](https://github.com/rancher-sandbox/luet-cosign) which is a luet plugin that generates the signatures on image push when building, and verifies them on package unpack when installing/upgrading/deploying.

The process is completely transparent to the end user when upgrading/deploying a running system and using our published artifacts.

When using luet-cosign as part of `luet install` you need to set `COSIGN_REPOSITORY=raccos/releases-green` and `COSIGN_EXPERIMENTAL=1` so it can find the proper signatures and use keyless verification


{{% alert title="Note" %}}
Currently setting `COSIGN_REPOSITORY` value is due to quay.io not supporting OCI artifacts. It may be removed in the future and signatures stored along the artifacts.
{{% /alert %}}


## Derivatives

If building a derivative, you can also sign and verify you final artifacts with the use of [luet-cosign](https://github.com/rancher-sandbox/luet-cosign).

As keyless is only possible to do in an CI environment (as it needs an OIDC token) you would need to set up private/public signature and verification.

{{% alert title="Note" %}}
If you are building and publishing your derivatives with luet on github, you can see an example on how we generate and push the keyless signatures ourselves on [this workflow](https://github.com/rancher-sandbox/cOS-toolkit/blob/master/.github/workflows/build-master-green-x86_64.yaml#L445)
{{% /alert %}}


### Verify cos-toolkit artifacts as part of derivative building

If you consume cos-toolkit artifacts in your Dockerfile as part of building a derivative you can verify the signatures of the artifacts by setting:

```dockerfile
ENV COSIGN_REPOSITORY=raccos/releases-green
ENV COSIGN_EXPERIMENTAL=1
RUN luet install -y meta/cos-verify # install dependencies for signature checking
```

{{% alert title="Note" %}}
The {{<package package="meta/cos-verify" >}} is a meta package that will pull {{<package package="toolchain/cosign" >}} and {{<package package="toolchain/luet-cosign" >}} .
{{% /alert %}}


And then making sure you call luet with `--plugin luet-cosign`. You can see an example of this in our [standard Dockerfile example](https://github.com/rancher-sandbox/cOS-toolkit/tree/master/examples/standard) 

That would verify the artifacts coming from our repository.


For signing resulting containers with a private/public key, please refer to the [cosign](https://github.com/sigstore/cosign) documents.

For verifying with a private/public key, the only thing you need is to set the env var `COSIGN_PUBLIC_KEY_LOCATION` to point to the public key that signed and enable the luet-cosign plugin.

{{% alert title="Note" %}}
Currently there is an issue in which if there is more than one repo and one of those repos is not signed the whole install will fail due to cosign failing to verify the unsigned repo.

If you are using luet with one or more unsigned repos, it's not possible to use cosign to verify the chain.

Please follow up in https://github.com/rancher-sandbox/luet-cosign/issues/6 for more info.
{{% /alert %}}
