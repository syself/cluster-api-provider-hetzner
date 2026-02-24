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

First you need to decide if you want to use the Syself CCM or the upstream HCloud CCM.

The CCM is the "Cloud Controller" which runs in the workload-cluster. Most important tasks of CCM:

- Set ProviderID on Nodes. This is important, so that CAPI in the mgt-cluster knows which capi
  machine (in mgt-cluster) is which Node (in wl-cluster).
- Creates LoadBalancers

If you are unsure, use the HCloud CCM. In the long run we (Syself) want to switch from our fork to
the upstream CCM.

The CAPH controller creates the required secrets in the workload cluster for you. You only need to
install the CCM. To allow switching the ccm, the controller creates the secret "hetzner" and
"hcloud" in the workload-cluster. These secrets contain the HCLOUD_TOKEN, so that the ccm can
connect to the HCloud API.

Important: CAPH and the CCM must both use the same ProviderID format for bare metal. Unfortunately
(for historical reasons), there are two formats:

- old: `hcloud://bm-NNNN`
- new: `hrobot://NNNN`

The Syself CCM uses the old format by default. The HCloud CCM always uses the new format.

If you use the new format, set the annotation `capi.syself.com/use-hrobot-provider-id-for-baremetal`
to `"true"` on the `HetznerCluster`. Our default templates have this annotation set.

If CAPH and the CCM do not agree on the ProviderID format, then new nodes will not be able to join
the cluster, because CAPI waits for the wrong ProviderID.

This only applies to new nodes. Once a node has a ProviderID, it will never change. Both CCMs
support both formats when the ProviderID is already set.

This applies only to bare metal. HCloud nodes always use the format `hcloud://NNNN`.

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

If you want to use the HCloud CCM:

```shell
helm repo add hcloud https://charts.hetzner.cloud
helm repo update hcloud

helm upgrade --install ccm hcloud/hcloud-cloud-controller-manager \
             --namespace kube-system \
             --kubeconfig workload-kubeconfig
```

Be sure that the HetznerCluster has not the annotation
`capi.syself.com/use-hrobot-provider-id-for-baremetal: "true"`.

---

If you want to use the Syself CCM (not recommended for new clusters):

```shell
helm repo add syself https://charts.syself.com
helm repo update syself

$ helm upgrade --install ccm syself/ccm-hetzner --version 2.0.1 \
              --namespace kube-system \
              --kubeconfig workload-kubeconfig
```

Be sure that the HetznerCluster has not the annotation
`capi.syself.com/use-hrobot-provider-id-for-baremetal`.

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
default     my-cluster-control-plane-6m6zf   my-cluster   my-cluster-control-plane-84hsn   hcloud://45443706     Running        10h   v1.33.6
default     my-cluster-control-plane-m6frm   my-cluster   my-cluster-control-plane-hvl5d   hcloud://45443651     Running        10h   v1.33.6
default     my-cluster-control-plane-qwsq6   my-cluster   my-cluster-control-plane-ss9kc   hcloud://45443746     Running        10h   v1.33.6
default     my-cluster-md-0-2xgj5-c5bhc      my-cluster   my-cluster-md-0-6xttr            hcloud://45443694     Running        10h   v1.33.6
default     my-cluster-md-0-2xgj5-rbnbw      my-cluster   my-cluster-md-0-fdq9l            hcloud://45443693     Running        10h   v1.33.6
default     my-cluster-md-0-2xgj5-tl2jr      my-cluster   my-cluster-md-0-59cgw            hcloud://45443692     Running        10h   v1.33.6
default     my-cluster-md-1-cp2fd-7nld7      my-cluster   bm-my-cluster-md-1-d7526         hcloud://bm-2317525   Running        9h    v1.33.6
default     my-cluster-md-1-cp2fd-n74sm      my-cluster   bm-my-cluster-md-1-l5dnr         hcloud://bm-2105469   Running        10h   v1.33.6
```

Please note that HCloud servers are prefixed with `hcloud://` and bare-metal servers are prefixed
with either `hcloud://bm-` or `hrobot://`, depending on your ProviderID format configuration.
