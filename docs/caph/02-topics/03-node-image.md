---
title: Node Images
metatitle: Custom Node Images for Nodes in Clusters Managed by CAPH
sidebar: Node Images
description: Build custom Node Images for CAPH using Packer on Hetzner Cloud. Create, configure, and snapshot servers with all necessary components for Kubernetes.
---

## What are node-images?

Node-images are pre-configured operating system (OS) images for setting up nodes within a Kubernetes cluster. A Kubernetes cluster consists of multiple nodes that are physical or virtual machines. To run the necessary Kubernetes components for a fully functional cluster, each node runs an operating system that hosts these components. These OS images should be compatible with Kubernetes and ensure that the node has the required environment to join and perform in the cluster. The images often comes with necessary components that are pre-installed or can be easy installable facilitating a smooth setup process for Kubernetes nodes.

## Node-images in CAPH

Node-image is necessary for using CAPH in production and as a user, there can be some specific needs as per your requirements that needs to be in the linux instance. The popular linux distributions might not contain all of those specifics. In such cases, the user need to build a node-image. These images can be uploaded to the Hetzner cloud as a snapshot and then the user can use these node-images for cluster creation.

## Creating a Node Image

For using cluster-API with the bootstrap provider kubeadm, we need a server with all the necessary components for running Kubernetes.
There are several ways to achieve this. In the quick-start guide, we use `pre-kubeadm` commands in the `KubeadmControlPlane` and `KubeadmConfigTemplate` objects. These are propagated from the bootstrap-provider-kubeadm and the control-plane-provider-kubeadm to the node as cloud-init commands. This way is usable universally also in other infrastructure providers.

For Hcloud, there is an alternative way of doing this my using Hcloud Snapshots. Snapshots are images you can boot from. This makes it easier to version the images, and creating new nodes using this image is faster. The same is possible for Hetzner BareMetal, as we could use installimage and a prepared tarball, which then gets installed as the OS for your nodes.

To use CAPH in production, it needs a node image. In Hetzner Cloud, it is not possible to upload your own images directly. However, a server can be created, configured, and then snapshotted.

For this, [Hashicorp Packer](https://github.com/hashicorp/packer) could be used, which already has support for Hetzner Cloud. But a simple Bash script with some `curl` commands to the Hcloud API could be used to create snapshots, too.

Then set `template.spec.imageName` in HCloudMachineTemplate to the name of your Hcloud snapshot. See [HCloudMachineTemplate Reference](../03-reference/03-hcloud-machine-template.md)
