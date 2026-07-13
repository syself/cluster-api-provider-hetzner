---
title: Creating the workload cluster
description: Provision a Kubernetes workload cluster with essential components for a secure deployment, including CNI, CCM, and optional CSI integration.
metatitle: Creating Workload Clusters on Hetzner
---

<Steps>

<Step>Generate the cluster.yaml</Step>

The `clusterctl generate cluster` command returns a YAML template for creating a workload cluster.
It generates a YAML file named `my-cluster.yaml` with a predefined list of Cluster API objects (`Cluster`, `Machines`, `MachineDeployments`, etc.) to be deployed in the current namespace.

```shell
clusterctl generate cluster my-cluster --kubernetes-version v1.36.0 --control-plane-machine-count=3 --worker-machine-count=3  > my-cluster.yaml
```

> [!NOTE]
> With the `--target-namespace` flag, you can specify a different target namespace.
>
> Run the `clusterctl generate cluster --help` command for more information.

> [!NOTE]
> Please note that ready-to-use Kubernetes configurations, production-ready node images, kubeadm configuration, cluster add-ons like CNI, and similar services need to be separately prepared or acquired to ensure a comprehensive and secure Kubernetes deployment. This is where **Syself Autopilot** comes into play, taking on these challenges to offer you a seamless, worry-free Kubernetes experience. Feel free to contact us via e-mail: <info@syself.com>.

<Step>Apply the workload cluster</Step>

The following command applies the configuration of the workload cluster:

```shell
kubectl apply -f my-cluster.yaml
```

<Step>Access the workload cluster</Step>

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

Wait until the `INITIALIZED` column reports `True` before moving on. The `READY` column will stay `False` until we install a CNI in the next step — that is expected. If you'd like to follow the progression live, run `kubectl get kubeadmcontrolplane -w`.

> [!NOTE]
> If you fetch the kubeconfig (next step) before `INITIALIZED` is `True`, kube-apiserver will not be listening yet and `helm` / `kubectl` calls will fail with `Kubernetes cluster unreachable: ... EOF`.

Once initialized, retrieve the kubeconfig of the workload cluster:

```shell
export CAPH_WORKER_CLUSTER_KUBECONFIG=/tmp/workload-kubeconfig
clusterctl get kubeconfig my-cluster > $CAPH_WORKER_CLUSTER_KUBECONFIG
```

A quick reachability check before installing the CNI:

```shell
KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG kubectl get --raw='/readyz'
```

You should see `ok` (a few component-level failures are still acceptable at this stage). If the call hangs or returns an EOF, give it another minute and retry.

<Step>Deploy the CNI solution</Step>

Cilium is used as a CNI solution in this guide. The values file lives in this repo at `templates/cilium/values.yaml`; we install it directly from the raw URL:

```shell
helm repo add cilium https://helm.cilium.io/

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install cilium cilium/cilium \
--namespace kube-system \
-f https://raw.githubusercontent.com/syself/cluster-api-provider-hetzner/main/templates/cilium/values.yaml
```

You can, of course, also install an alternative CNI, e.g., calico.

<Step>Deploy the Cloud Controller Manager (CCM)</Step>

The CCM runs in the workload cluster and integrates Kubernetes with the Hetzner APIs. In practice, it is responsible for tasks such as setting the `ProviderID` on nodes and managing load balancers. You need to install a CCM in this flow so the workload cluster can properly interact with Hetzner infrastructure.

You have two options here:

- **Syself CCM**: the `ccm-hetzner` chart maintained by Syself.
- **Hetzner CCM**: the upstream `hcloud-cloud-controller-manager` from Hetzner Cloud.

### Syself CCM

CAPH already syncs the `hcloud` secret you created on the previous page into the workload cluster's `kube-system` namespace, so the CCM only needs to be pointed at it (the chart default is `hetzner`, so we override `secret.name`):

```shell
helm repo add syself https://charts.syself.com
helm repo update syself

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install ccm syself/ccm-hetzner \
  --version 2.0.6 \
  --namespace kube-system \
  --set secret.name=hcloud
```

### Hetzner CCM (upstream)

The upstream `hcloud-cloud-controller-manager` static manifest (`deploy/ccm.yaml`) is hardcoded to read the token from secret `hcloud` with key `token`. Even though the management secret you created on the previous page only has the key `hcloud`, CAPH's compatibility shim writes the workload-cluster copy with **both** keys (`hcloud` and `token`) — so the upstream manifest works as-is, no patching required:

```shell
KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG kubectl apply \
  -f https://raw.githubusercontent.com/hetznercloud/hcloud-cloud-controller-manager/main/deploy/ccm.yaml
```

If you install the upstream CCM via its Helm chart instead, the defaults also point at secret `hcloud` / key `token`, so no `--set` overrides are needed.

For upstream CCM details and bare-metal `ProviderID` format, see the [Baremetal Docs](/docs/caph/topics/baremetal/creating-workload-cluster#deploying-the-hetzner-cloud-controller-manager).

<Step>Deploy the CSI (optional)</Step>

The Hetzner CSI chart lives in the `hcloud` Helm repo, which we add first. The chart's default `controller.hcloudToken.existingSecret` already points at secret `hcloud` with key `token`, which matches what CAPH wrote into the workload cluster — so no overrides are needed beyond the storage class definition:

```shell
helm repo add hcloud https://charts.hetzner.cloud
helm repo update hcloud

cat << EOF > csi-values.yaml
storageClasses:
- name: hcloud-volumes
  defaultStorageClass: true
  reclaimPolicy: Retain
EOF

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install csi hcloud/hcloud-csi \
--namespace kube-system -f csi-values.yaml
```

<Step>Clean up (optional)</Step>

If you want to continue with the next step and move the Cluster API components to your workload
cluster (so it becomes the new management cluster), do not run the cleanup command.

If you want to stop here, delete the workload cluster and remove all of the components by using:

```shell
kubectl delete cluster my-cluster
```

> [!IMPORTANT]
> In order to ensure a proper clean-up of your infrastructure, you must always delete the cluster object. Deleting the entire cluster template with the `kubectl delete -f my-cluster.yaml` command might lead to pending resources that have to be cleaned up manually.

Delete management cluster with the following command:

```shell
kind delete cluster
```

</Steps>
