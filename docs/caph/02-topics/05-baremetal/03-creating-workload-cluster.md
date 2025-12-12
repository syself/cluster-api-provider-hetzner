---
title: Creating a workload cluster with bare metal servers
metatitle: Provisioning a Kubernetes Workload Cluster with Bare Metal Servers
sidebar: Creating a workload cluster with bare metal servers
description: Create workload clusters on Hetzner using bare metal servers as nodes in an automated way, using CAPI custom resources.
---

## Creating Workload Cluster

{% callout %}

Secrets as of now are hardcoded given we are using a flavor which is essentially a template. If you want to use your own naming convention for secrets then you'll have to update the templates. Please make sure that you pay attention to the sshkey name.

{% /callout %}

Since we have already created secret in hetzner robot, hcloud and ssh-keys as secret in management cluster, we can create a workload cluster. Generate the manifest by using `clusterctl generate`:

```shell
clusterctl generate cluster my-cluster --flavor hetzner-hcloud-control-planes > my-cluster.yaml
```

And apply it:

```shell
kubectl apply -f my-cluster.yaml
```

```shell
$ kubectl apply -f my-cluster.yaml
kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/my-cluster-md-0 created
kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/my-cluster-md-1 created
cluster.cluster.x-k8s.io/my-cluster created
machinedeployment.cluster.x-k8s.io/my-cluster-md-0 created
machinedeployment.cluster.x-k8s.io/my-cluster-md-1 created
machinehealthcheck.cluster.x-k8s.io/my-cluster-control-plane-unhealthy-5m created
machinehealthcheck.cluster.x-k8s.io/my-cluster-md-0-unhealthy-5m created
machinehealthcheck.cluster.x-k8s.io/my-cluster-md-1-unhealthy-5m created
kubeadmcontrolplane.controlplane.cluster.x-k8s.io/my-cluster-control-plane created
hcloudmachinetemplate.infrastructure.cluster.x-k8s.io/my-cluster-control-plane created
hcloudmachinetemplate.infrastructure.cluster.x-k8s.io/my-cluster-md-0 created
hcloudremediationtemplate.infrastructure.cluster.x-k8s.io/control-plane-remediation-request created
hcloudremediationtemplate.infrastructure.cluster.x-k8s.io/worker-remediation-request created
hetznerbaremetalmachinetemplate.infrastructure.cluster.x-k8s.io/my-cluster-md-1 created
hetznercluster.infrastructure.cluster.x-k8s.io/my-cluster created
```

## Getting the kubeconfig of workload cluster

After a while, our first controlplane should be up and running. You can verify it using the output of `kubectl get kcp` followed by `kubectl get machines`

Once it's up and running, you can get the kubeconfig of the workload cluster using the following command:

```shell
clusterctl get kubeconfig my-cluster > workload-kubeconfig
chmod go-r workload-kubeconfig # required to avoid helm warning
```

## Deploy Cluster Addons

{% callout %}

This is important for the functioning of the cluster otherwise the cluster won't work.

{% /callout %}

### Deploying the Hetzner Cloud Controller Manager

{% callout %}

This requires a secret containing access credentials to both Hetzner Robot and HCloud.

{% /callout %}

If you have configured your secret correctly in the previous step then you already have the secret in your cluster.
Let's deploy the hetzner CCM helm chart.

```shell
helm repo add syself https://charts.syself.com
helm repo update syself

$ helm upgrade --install ccm syself/ccm-hetzner --version 2.0.1 \
              --namespace kube-system \
              --kubeconfig workload-kubeconfig
Release "ccm" does not exist. Installing it now.
NAME: ccm
LAST DEPLOYED: Thu Apr  4 21:09:25 2024
NAMESPACE: kube-system
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

### Installing CNI

For CNI, let's deploy cilium in the workload cluster that will facilitate the networking in the cluster.

```shell
$ helm install cilium cilium/cilium --kubeconfig workload-kubeconfig
NAME: cilium
LAST DEPLOYED: Thu Apr  4 21:11:13 2024
NAMESPACE: default
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
You have successfully installed Cilium with Hubble.

For any further help, visit https://docs.cilium.io/en/v1.15/gettinghelp
```

### Verifying the cluster

Now, the cluster should be up and you can verify it by running the following commands:

```shell
$ kubectl get clusters -A
NAMESPACE   NAME         CLUSTERCLASS   PHASE         AGE   VERSION
default     my-cluster                  Provisioned   10h
$ kubectl get machines -A
NAMESPACE   NAME                             CLUSTER      NODENAME                         PROVIDERID            PHASE          AGE   VERSION
default     my-cluster-control-plane-6m6zf   my-cluster   my-cluster-control-plane-84hsn   hcloud://45443706     Running        10h   v1.31.6
default     my-cluster-control-plane-m6frm   my-cluster   my-cluster-control-plane-hvl5d   hcloud://45443651     Running        10h   v1.31.6
default     my-cluster-control-plane-qwsq6   my-cluster   my-cluster-control-plane-ss9kc   hcloud://45443746     Running        10h   v1.31.6
default     my-cluster-md-0-2xgj5-c5bhc      my-cluster   my-cluster-md-0-6xttr            hcloud://45443694     Running        10h   v1.31.6
default     my-cluster-md-0-2xgj5-rbnbw      my-cluster   my-cluster-md-0-fdq9l            hcloud://45443693     Running        10h   v1.31.6
default     my-cluster-md-0-2xgj5-tl2jr      my-cluster   my-cluster-md-0-59cgw            hcloud://45443692     Running        10h   v1.31.6
default     my-cluster-md-1-cp2fd-7nld7      my-cluster   bm-my-cluster-md-1-d7526         hrobot://2317525   Running        9h    v1.31.6
default     my-cluster-md-1-cp2fd-n74sm      my-cluster   bm-my-cluster-md-1-l5dnr         hrobot://2105469   Running        10h   v1.31.6
```

Please note that hcloud servers are prefixed with `hcloud://` and baremetal servers are prefixed with `hrobot://`.
