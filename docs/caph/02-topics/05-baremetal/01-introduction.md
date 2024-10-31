---
title: Hetzner baremetal
metatitle: "Introduction to Using Hetzner Bare Metal Servers as Nodes with CAPH"
sidebar: Hetzner bare metal
description: Explanation of the Hetzner offerings, and the available cluster flavors with bare metal servers.
---

Hetzner have two offerings primarily:

1. `Hetzner Cloud`/`Hcloud` for virtualized servers
2. `Hetzner Dedicated`/`Robot` for bare metal servers

In this guide, we will focus on creating a cluster from baremetal servers.

## Flavors of Hetzner Bare Metal

Now, there are different ways you can use baremetal servers, you can use them as controlplanes or as worker nodes or both. Based on that we have created some templates and those templates are released as flavors in GitHub releases.

These flavors can be consumed using [clusterctl](https://main.cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl) tool:

To use bare metal servers for your deployment, you can choose one of the following flavors:

| Flavor                                         | What it does                                                                                                  |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `hetzner-baremetal-control-planes-remediation` | Uses bare metal servers for the control plane nodes - with custom remediation (try to reboot machines first)  |
| `hetzner-baremetal-control-planes`             | Uses bare metal servers for the control plane nodes - with normal remediation (unprovision/recreate machines) |
| `hetzner-hcloud-control-planes`                | Uses the hcloud servers for the control plane nodes and the bare metal servers for the worker nodes           |

{% callout %}

These flavors are only for demonstration purposes and should not be used in production.

{% /callout %}

## Purchasing Bare Metal Servers

If you want to create a cluster with bare metal servers, you will also need to set up the robot credentials. For setting robot credentials, as described in the [reference](/docs/caph/03-reference/06-hetzner-bare-metal-machine-template.md), you need to purchase bare metal servers beforehand manually.
