
---
title: "Cosign"
linkTitle: "Cosign"
weight: 2
date: 2023-05-11
description: >
  How we use cosign in elemental-toolkit
---

[Cosign](https://github.com/sigstore/cosign) is a project that signs and verifies containers and stores the signatures on OCI registries.

You can check the cosign [github repo](https://github.com/sigstore/cosign) for more information.

In elemental-toolkit we sign every container that we generate as part of our publish process so the signature can be verified during package installation or during deploy/upgrades from a deployed system to verify that the containers have not been altered in any way since their build.

Currently cosign provides 2 methods for signing and verifying.

 - private/public key
 - keyless

We use keyless signatures based on OIDC Identity tokens provided by github, so nobody has access to any private keys and can use them. (For more info about keyless signing/verification check [here](https://github.com/sigstore/cosign/blob/main/KEYLESS.md))

The process is completely transparent to the end user when upgrading/deploying a running system and using our published artifacts.

## Derivatives

If building a derivative, you can also sign and verify you final artifacts with the use of cosign.

As keyless is only possible to do in an CI environment (as it needs an OIDC token) you would need to set up private/public signature and verification.
