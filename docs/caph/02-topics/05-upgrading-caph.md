---
title: Upgrading the Kubernetes Cluster API Provider Hetzner
---

This guide explains how to upgrade Cluster API and Cluster API Provider Hetzner (aka CAPH). Additionally, it also references [upgrading your kubernetes version](#external-cluster-api-reference) as part of this guide.

## Set matching kubeconfig

Connect `kubectl` to the management cluster.

Check, that you are connected to the correct cluster:

```shell
❯ k config current-context
mgm-cluster-admin@mgm-cluster
```

OK, looks good.

## Update clusterctl

Is clusterctl still up to date?

```shell
$ clusterctl version -oyaml
clusterctl:
  buildDate: "2024-04-09T17:23:12Z"
  compiler: gc
  gitCommit: c9136af030eaba5deed7be5ca9f6c3f5e6d69334
  gitTreeState: clean
  gitVersion: v1.7.0-rc.1
  goVersion: go1.21.9
  major: "1"
  minor: "7"
  platform: linux/amd64
```

You can see the current version here:

[https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl)

If your clusterctl is outdated, then upgrade it. See the above URL for details.

## clusterctl upgrade plan

Have a look at what could get upgraded:

```shell
$ clusterctl upgrade plan
Checking cert-manager version...
Cert-Manager is already up to date

Checking new release availability...

Latest release available for the v1beta1 API Version of Cluster API (contract):

NAME                     NAMESPACE                           TYPE                     CURRENT VERSION   NEXT VERSION
bootstrap-kubeadm        capi-kubeadm-bootstrap-system       BootstrapProvider        v1.6.0            v1.6.3
control-plane-kubeadm    capi-kubeadm-control-plane-system   ControlPlaneProvider     v1.6.0            v1.6.3
cluster-api              capi-system                         CoreProvider             v1.6.0            v1.6.3
infrastructure-hetzner   caph-system                         InfrastructureProvider   v1.0.0-beta.30    Already up to date

You can now apply the upgrade by executing the following command:

clusterctl upgrade apply --contract v1beta1
```

Docs: [clusterctl upgrade plan](https://cluster-api.sigs.k8s.io/clusterctl/commands/upgrade.html)

You might be surprised that for `infrastructure-hetzner`, you see the "Already up to date" message below "NEXT VERSION".

{% callout %}

`clusterctl upgrade plan` does not display pre-release versions by default.

{% /callout %}

## Upgrade cluster-API

We will upgrade cluster API core components to v1.6.3 version.
Use the command, which you saw in the plan:

```shell
$ clusterctl upgrade apply --contract v1beta1
Checking cert-manager version...
Cert-manager is already up to date
Performing upgrade...
Scaling down Provider="cluster-api" Version="v1.6.0" Namespace="capi-system"
Scaling down Provider="bootstrap-kubeadm" Version="v1.6.0" Namespace="capi-kubeadm-bootstrap-system"
Scaling down Provider="control-plane-kubeadm" Version="v1.6.0" Namespace="capi-kubeadm-control-plane-system"
Deleting Provider="cluster-api" Version="v1.6.0" Namespace="capi-system"
Installing Provider="cluster-api" Version="v1.6.3" TargetNamespace="capi-system"
Deleting Provider="bootstrap-kubeadm" Version="v1.6.0" Namespace="capi-kubeadm-bootstrap-system"
Installing Provider="bootstrap-kubeadm" Version="v1.6.3" TargetNamespace="capi-kubeadm-bootstrap-system"
Deleting Provider="control-plane-kubeadm" Version="v1.6.0" Namespace="capi-kubeadm-control-plane-system"
Installing Provider="control-plane-kubeadm" Version="v1.6.3" TargetNamespace="capi-kubeadm-control-plane-system"
```

Great, cluster-API was upgraded.

{% callout %}

If you want to update only one components or update components one by one then there are flags for that under `clusterctl upgrade apply` subcommand like `--bootstrap`, `--control-plane` and `--core`.

{% /callout %}

## Upgrade CAPH

You can find the latest version of CAPH here:

https://github.com/syself/cluster-api-provider-hetzner/tags

```shell
$ clusterctl upgrade apply --infrastructure=hetzner:v1.0.0-beta.33
Checking cert-manager version...
Cert-manager is already up to date
Performing upgrade...
Scaling down Provider="infrastructure-hetzner" Version="" Namespace="caph-system"
Deleting Provider="infrastructure-hetzner" Version="" Namespace="caph-system"
Installing Provider="infrastructure-hetzner" Version="v1.0.0-beta.33" TargetNamespace="caph-system"
```

After the upgrade, you'll notice the new pod spinning up the `caph-system` namespace.

```shell
$ kubectl get pods -n caph-system
NAME                                       READY   STATUS    RESTARTS   AGE
caph-controller-manager-85fcb6ffcb-4sj6d   1/1     Running   0          79s
```

{% callout %}

Please note that `clusterctl` doesn't support pre-release of GitHub by default so if you want to use a pre-release, you'll have to specify the version such as `hetzner:v1.0.0-beta.33`

{% /callout %}

## Check your cluster

Check the health of your workload cluster with your preferred tools and ensure that all components are healthy especially apiserver and etcd pods.

## External Cluster API Reference

After upgrading cluster API, you may want to update the Kubernetes version of your controlplane and worker nodes. Those details can be found in the [Cluster API documentation](https://cluster-api.sigs.k8s.io/tasks/upgrading-clusters).

{% callout %}

The update can be done on either management cluster or workload cluster separately as well.

{% /callout %}

You should upgrade your kubernetes version after considering that a Cluster API minor release supports (when it’s initially created):

- 4 Kubernetes minor releases for the management cluster (N - N-3)
- 6 Kubernetes minor releases for the workload cluster (N - N-5)
