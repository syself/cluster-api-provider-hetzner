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

| CAPI Version             | Hetzner Provider `v1.0.x` | Hetzner Provider `v1.1.x` |
| ------------------------ | ------------------------- | ------------------------- |
| `v1.8.x`                 | ✅                        | ❌                        |
| `v1.9.x`                 | ✅                        | ❌                        |
| `v1.10.x`                | ✅                        | ❌                        |
| `v1.11.x` [start of beta2](https://cluster-api.sigs.k8s.io/developer/providers/migrations/v1.10-to-v1.11) | ✅                        | ✅                        |
| `v1.12.x`                | ❌                        | ✅                        |

This provider's versions can install and manage the following versions of Kubernetes:

|                   | Hetzner Provider `v1.0.x` | Hetzner Provider `v1.1.x` |
| ----------------- | ------------------------- | ------------------------- |
| Kubernetes 1.31.x | ✅                        | ❌                        |
| Kubernetes 1.32.x | ✅                        | ✅                        |
| Kubernetes 1.33.x | ✅                        | ✅                        |
| Kubernetes 1.34.x | ❔                        | ✅                        |

Test status:

- ✅ tested
- ❔ should work

Each version of Cluster API for Hetzner will attempt to support at least two Kubernetes versions.

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future
modifications to this policy may be made to more closely align with other providers in the Cluster
API ecosystem.

{% callout %}

As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

{% /callout %}
