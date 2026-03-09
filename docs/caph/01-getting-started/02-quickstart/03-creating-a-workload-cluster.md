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
clusterctl generate cluster my-cluster --kubernetes-version v1.33.6 --control-plane-machine-count=3 --worker-machine-count=3  > my-cluster.yaml
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

The file `templates/cilium/cilium.yaml` is a repo-provided Helm values file in this repository.
Before running the command, make sure this file exists at that path in your working environment
(for example, by using it from a local checkout of this repo or copying it from
`templates/cilium/cilium.yaml` into your working directory setup).

```shell
helm repo add cilium https://helm.cilium.io/

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install cilium cilium/cilium \
--namespace kube-system \
-f templates/cilium/cilium.yaml
```

You can, of course, also install an alternative CNI, e.g., calico.

## Deploy the CCM

The CCM (Cloud Controller Manager) runs in the workload cluster and integrates Kubernetes with the
Hetzner APIs. In practice, it is responsible for tasks such as setting the `ProviderID` on nodes
and managing load balancers.

You need to install a CCM in this flow so the workload cluster can properly interact with Hetzner
infrastructure.

### Deploy HCloud Cloud Controller Manager - _hcloud only_

The following `make` command will install the CCM in your workload cluster:

`make install-ccm-in-wl-cluster`

For a cluster with a private network use: `make install-ccm-in-wl-cluster PRIVATE_NETWORK=true`

For a more detailed explanation of the CCM and bare-metal setup, see [Baremetal
Docs](/docs/caph/02-topics/05-baremetal/03-creating-workload-cluster.md#deploying-the-hetzner-cloud-controller-manager)

## Deploying the CSI (optional)

```shell
cat << EOF > csi-values.yaml
controller:
  hcloudToken:
    existingSecret:
      name: hetzner
      key: hcloud
storageClasses:
- name: hcloud-volumes
  defaultStorageClass: true
  reclaimPolicy: Retain
EOF

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install csi hcloud/hcloud-csi \
--namespace kube-system -f csi-values.yaml
```

If you want to continue with the next step and move the Cluster API components to your workload
cluster (so it becomes the new management cluster), do not run the cleanup command.

## Clean Up (optional)

If you want to stop here, delete the workload cluster and remove all of the components by using:

```shell
kubectl delete cluster my-cluster
```

> **IMPORTANT**: In order to ensure a proper clean-up of your infrastructure, you must always delete the cluster object. Deleting the entire cluster template with the `kubectl delete -f my-cluster.yaml` command might lead to pending resources that have to be cleaned up manually.

Delete management cluster with the following command:

```shell
kind delete cluster
```
