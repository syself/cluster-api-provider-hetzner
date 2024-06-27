---
title: Prerequisites
---

This guide goes through all the necessary steps to create a cluster on Hetzner infrastructure (on HCloud).

{% callout %}

The cluster templates used in the repository and in this guide for creating clusters are for development purposes only. These templates are not advised to be used in the production environment. However, the software is production-ready and users use it in their production environment. Make your clusters production-ready with the help of Syself Autopilot. For more information, contact <contact@syself.com>.

{% /callout %}

There are certain prerequisites that you need to comply with before getting started with this guide.

## Installing Helm

Helm is a package manager that facilitates the installation and management of applications in a Kubernetes cluster. Refer to the [official docs](https://helm.sh/docs/intro/install/) for installation.

## Understanding Cluster API and clusterctl

Cluster API Provider Hetzner uses Cluster API to create a cluster in provider Hetzner. So, it is essential to understand Cluster API before getting started with the cluster creation on Hetzner infrastructure. It is a subproject of Kubernetes focused on providing declarative APIs and tooling to simplify provisioning, upgrading, and operating multiple Kubernetes clusters. Know more about Cluster API from its [official documentation](https://cluster-api.sigs.k8s.io/introduction).

`clusterctl` is the command-line tool used for managing the lifecycle of a Cluster API management cluster. Learn more about `clusterctl` and its commands from the official documentation of Cluster API [here](https://cluster-api.sigs.k8s.io/clusterctl/overview).
