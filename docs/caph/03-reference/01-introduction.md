---
title: Object Reference
metatitle: Naming Conventions for Custom Resources
sidebar: Object Reference
description: "Overview of the naming conventions of objects specific to the CAPH integration."
---

In this object reference, we introduce all objects that are specific for this provider integration. The naming of objects, servers, machines, etc. can be confusing. Without claiming to be consistent throughout these docs, we would like to give an overview of how we name things here.

First, there are some important counterparts of our objects and CAPI objects.

- `HetznerCluster` has CAPI's `Cluster` object.
- CAPI's `Machine` object is the counterpart of both `HCloudMachine` and `HetznerBareMetalMachine`.

These two are objects of the provider integration that are reconciled by the `HCloudMachineController` and the `HetznerBareMetalMachineController` respectively.

The `HCloudMachineController` checks whether there is a server in the HCloud API already and if not, buys/creates one that corresponds to a `HCloudMachine` object.

The `HetznerBareMetalMachineController` does not buy new bare metal machines, but instead consumes a host of the inventory of `HetznerBareMetalHosts`, which have a one-to-one relationship to Hetzner dedicated/root/bare metal servers that have been bought manually by the user.

Therefore, there is an important difference between the `HCloudMachine` object and a server in the HCloud API. For bare metal, we have even three terms: the `HetznerBareMetalMachine` object, the `HetznerBareMetalHost` object, and the actual bare metal server that can be accessed through Hetzner's robot API.
