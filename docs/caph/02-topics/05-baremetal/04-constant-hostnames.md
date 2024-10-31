---
title: Using constant hostnames
metatitle: Enable Fixed Hostnames for Bare Metal Servers in Hetzner Clusters
sidebar: Using constant hostnames
description: Utilize fixed node names for bare metal servers, useful for local storage persistence across reprovisionings.
---

## Constant hostnames for baremetal servers

In some cases it has advantages to fix the hostname and with it the names of nodes in your clusters. For cloud servers not so much as for bare metal servers, where there are storage integrations that allow you to use the storage of the bare metal servers and that work with fixed node names.

Therefore, there is the possibility to create a cluster that uses fixed node names for bare metal servers. Please note: this only applies to the bare metal servers and not to Hetzner Cloud servers.

You can trigger this feature by creating a `Cluster` or `HetznerBareMetalMachine` (you can choose) with the annotation `"capi.syself.com/constant-bare-metal-hostname": "true"`. Of course, `HetznerBareMetalMachines` are not created by the user. However, if you use the `ClusterClass`, then you can add the annotation to a `MachineDeployment`, so that all machines are created with this annotation.

This is still an experimental feature but it should be safe to use and to also update existing clusters with this annotation. All new machines will be created with this constant hostname.
