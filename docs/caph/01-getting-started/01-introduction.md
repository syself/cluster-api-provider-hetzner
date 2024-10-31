---
title: Introduction
metatitle: "What is the Cluster API Provider Hetzner and Compatible Versions"
sidebar: Introduction
description: Cluster API Provider Hetzner automates the lifecycle and provisioning of Kubernetes clusters on cloud or bare metal.
---

Welcome to the official documentation for the Cluster API Provider Hetzner (CAPH). If you are new to it, you can keep reading the Getting Started section, the Quickstart guide will walk you through your first cluster setup.

## What is the Cluster API Provider Hetzner

CAPH is a Kubernetes Cluster API provider that facilitates the deployment and management of self-managed Kubernetes clusters on Hetzner infrastructure. The provider supports both cloud and bare-metal instances for consistent, scalable, and production-ready cluster operations.

It is recommended that you have at least a basic understanding of Cluster API before getting started with CAPH. You can refer to the Cluster API Quick Start Guide from its [official documentation](https://cluster-api.sigs.k8s.io).

## Compatibility with Cluster API and Kubernetes Versions

This provider's versions are compatible with the following versions of Cluster API:

|                                      | CAPI `v1beta1` (`v1.7.x`) | CAPI `v1beta1` (`v1.8.x`) |
| ------------------------------------ | ------------------------- | ------------------------- |
| Hetzner Provider `v1.0.0-beta.34-43` | ✅                        | ❌                        |
| Hetzner Provider `v1.0.0`            | ✅                        | ✅                        |

This provider's versions can install and manage the following versions of Kubernetes:

|                   | Hetzner Provider `v1.0.x` |
| ----------------- | ------------------------- |
| Kubernetes 1.23.x | ✅                       |
| Kubernetes 1.24.x | ✅                       |
| Kubernetes 1.25.x | ✅                       |
| Kubernetes 1.26.x | ✅                       |
| Kubernetes 1.27.x | ✅                       |
| Kubernetes 1.28.x | ✅                       |
| Kubernetes 1.29.x | ✅                       |
| Kubernetes 1.30.x | ✅                       |

Test status:

- ✅ tested
- ❔ should work, but we weren't able to test it

Each version of Cluster API for Hetzner will attempt to support at least two Kubernetes versions.

{% callout %}

As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

{% /callout %}
