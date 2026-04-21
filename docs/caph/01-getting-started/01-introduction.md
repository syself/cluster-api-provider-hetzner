---
title: Introduction
metatitle: What is the Cluster API Provider Hetzner and Compatible Versions
sidebar: Introduction
description: Cluster API Provider Hetzner automates the lifecycle and provisioning of Kubernetes clusters on cloud or bare metal.
ogDescription: Hetzner's Cluster API automates Kubernetes lifecycle and setup.
ogTitle: "Cluster API Hetzner: Versions"
---

Welcome to the official documentation for the Cluster API Provider Hetzner (CAPH). If you are new to it, you can keep reading the Getting Started section, the Quickstart guide will walk you through your first cluster setup.

## What is the Cluster API Provider Hetzner

CAPH is a Kubernetes Cluster API provider that facilitates the deployment and management of self-managed Kubernetes clusters on Hetzner infrastructure. The provider supports both cloud and bare-metal instances for consistent, scalable, and production-ready cluster operations.

It is recommended that you have at least a basic understanding of Cluster API before getting started with CAPH. You can refer to the Cluster API Quick Start Guide from its [official documentation](https://cluster-api.sigs.k8s.io).

## Compatibility with Cluster API and Kubernetes Versions

This provider's versions are compatible with the following versions of Cluster API:

| CAPI Version                                                                                              | Hetzner Provider `v1.0.x` | Hetzner Provider `v1.1.x` |
| --------------------------------------------------------------------------------------------------------- | ------------------------- | ------------------------- |
| `v1.8.x`                                                                                                  | ✅                        | ❔                        |
| `v1.9.x`                                                                                                  | ✅                        | ❔                        |
| `v1.10.x`                                                                                                 | ✅                        | ✅                        |
| `v1.11.x` [start of beta2](https://cluster-api.sigs.k8s.io/developer/providers/migrations/v1.10-to-v1.11) | ❌                        | ✅                        |
| `v1.12.x`                                                                                                 | ❌                        | ✅                        |

This provider's versions can install and manage the following versions of Kubernetes:

|                   | Hetzner Provider `v1.0.x` | Hetzner Provider `v1.1.x`  |
| ----------------- | ------------------------- | -------------------------- |
| Kubernetes 1.31.x | ✅                        | ❌                         |
| Kubernetes 1.32.x | ✅                        | ✅                         |
| Kubernetes 1.33.x | ✅                        | ✅                         |
| Kubernetes 1.34.x | ✅                        | ✅ (needs CAPI >= v1.11.1) |
| Kubernetes 1.35.x | ❔                        | ✅ (needs CAPI >= v1.12.1) |

Test status:

- ✅ tested
- ❔ should work, but we weren't able to test it

Related:

- [Support matrix for the Cluster API core provider](https://cluster-api.sigs.k8s.io/reference/versions.html#core-provider-cluster-api-controller)

Compatibility notes:

- CAPH `v1.1.x` still implements the deprecated `v1beta1` contract. On CAPI `v1.11.x` through `v1.15.x`, this should keep working via the temporary compatibility layer for the deprecated `v1beta1` contract, but upstream documents limitations and does not recommend staying on `v1beta1` long term. That compatibility is planned to end when `v1beta1` stops being served in CAPI `v1.16.x`. See the upstream [version support page](https://cluster-api.sigs.k8s.io/reference/versions), the upstream [v1.10 to v1.11 migration guide](https://cluster-api.sigs.k8s.io/developer/providers/migrations/v1.10-to-v1.11), the upstream [InfraMachine contract notes](https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-machine), and the upstream [removal plan](https://github.com/kubernetes-sigs/cluster-api/issues/11920).
- CAPH `v1.2.x` will be the `v1beta2` line and aligns with CAPI `v1.11.x` and later `v1beta2` releases.

Each version of Cluster API for Hetzner will attempt to support at least two Kubernetes versions.

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future
modifications to this policy may be made to more closely align with other providers in the Cluster
API ecosystem.

{% callout %}

As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

{% /callout %}
