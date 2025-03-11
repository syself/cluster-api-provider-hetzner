---
title: Creating the workload cluster
metatitle: Creating Workload Clusters on Hetzner
sidebar: Creating the workload cluster
description: Provision a Kubernetes workload cluster with essential components for a secure deployment, including CNI, CCM, and optional CSI integration.
---

## Generating the cluster.yaml

The `clusterctl generate cluster` command returns a YAML template for creating a workload cluster.
It generates a YAML file named `my-cluster.yaml` with a predefined list of Cluster API objects (`Cluster`, `Machines`, `MachineDeployments`, etc.) to be deployed in the current namespace.

```shell
clusterctl generate cluster my-cluster --kubernetes-version v1.31.6 --control-plane-machine-count=3 --worker-machine-count=3  > my-cluster.yaml
```

{% callout %}

With the `--target-namespace` flag, you can specify a different target namespace.

Run the `clusterctl generate cluster --help` command for more information.

{% /callout %}

{% callout %}

Please note that ready-to-use Kubernetes configurations, production-ready node images, kubeadm configuration, cluster add-ons like CNI, and similar services need to be separately prepared or acquired to ensure a comprehensive and secure Kubernetes deployment. This is where **Syself Autopilot** comes into play, taking on these challenges to offer you a seamless, worry-free Kubernetes experience. Feel free to contact us via e-mail: <info@syself.com>.

{% /callout %}

## Applying the workload cluster

The following command applies the configuration of the workload cluster:

```shell
kubectl apply -f my-cluster.yaml
```

## Accessing the workload cluster

The cluster will now start provisioning. You can check status with:

```shell
kubectl get cluster
```

You can also view the cluster and its resources at a glance by running:

```shell
clusterctl describe cluster my-cluster
```

To verify the first control plane is up, use the following command:

```shell
kubectl get kubeadmcontrolplane
```

{% callout %}

The control plane won’t be `ready` until we install a CNI in the next step.

{% /callout %}

After the first control plane node is up and running, we can retrieve the kubeconfig of the workload cluster with:

```shell
export CAPH_WORKER_CLUSTER_KUBECONFIG=/tmp/workload-kubeconfig
clusterctl get kubeconfig my-cluster > $CAPH_WORKER_CLUSTER_KUBECONFIG
```

## Deploying the CNI solution

Cilium is used as a CNI solution in this guide. The following command deploys it to your cluster:

```shell
helm repo add cilium https://helm.cilium.io/

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install cilium cilium/cilium --version 1.14.4 \
--namespace kube-system \
-f templates/cilium/cilium.yaml
```

You can, of course, also install an alternative CNI, e.g., calico.

{% callout %}

There is a bug in Ubuntu that requires the older version of Cilium for this quickstart guide.

{% /callout %}

## Deploy the CCM

### Deploy HCloud Cloud Controller Manager - _hcloud only_

The following `make` command will install the CCM in your workload cluster:

`make install-ccm-in-wl-cluster PRIVATE_NETWORK=false`

For a cluster without a private network, use the following command:

```shell
helm repo add hcloud https://charts.hetzner.cloud
helm repo update hcloud

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install hccm hcloud/hcloud-cloud-controller-manager \
        --namespace kube-system \
        --set env.HCLOUD_TOKEN.valueFrom.secretKeyRef.name=hetzner \
        --set env.HCLOUD_TOKEN.valueFrom.secretKeyRef.key=hcloud \
        --set privateNetwork.enabled=false
```

## Deploying the CSI (optional)

```shell
cat << EOF > csi-values.yaml
storageClasses:
- name: hcloud-volumes
  defaultStorageClass: true
  reclaimPolicy: Retain
EOF

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install csi syself/csi-hcloud --version 0.2.0 \
--namespace kube-system -f csi-values.yaml
```

## Clean Up

Delete the workload cluster and remove all of the components by using:

```shell
kubectl delete cluster my-cluster
```

> **IMPORTANT**: In order to ensure a proper clean-up of your infrastructure, you must always delete the cluster object. Deleting the entire cluster template with the `kubectl delete -f capi-quickstart.yaml` command might lead to pending resources that have to be cleaned up manually.

Delete management cluster with the following command:

```shell
kind delete cluster
```
