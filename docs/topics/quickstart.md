# Quickstart Guide

This guide goes through all the necessary steps to create a cluster on Hetzner infrastructure (on HCloud).

>Note: The cluster templates used in the repository and in this guide for creating clusters are for development purposes only. These templates are not advised to be used in the production environment. However, the software is production-ready and users use it in their production environment. Make your clusters production-ready with the help of Syself Autopilot. For more information, contact <info@syself.com>. 

## Prerequisites

There are certain prerequisites that you need to comply with before getting started with this guide.

### Installing Helm

Helm is a package manager that facilitates the installation and management of applications in a Kubernetes cluster. Refer to the [official docs](https://helm.sh/docs/intro/install/) for installation.

### Understanding Cluster API and clusterctl

Cluster API Provider Hetzner uses Cluster API to create a cluster in provider Hetzner. So, it is essential to understand Cluster API before getting started with the cluster creation on Hetzner infrastructure. It is a subproject of Kubernetes focused on providing declarative APIs and tooling to simplify provisioning, upgrading, and operating multiple Kubernetes clusters. Know more about Cluster API from its [official documentation](https://cluster-api.sigs.k8s.io/introduction).

`clusterctl` is the command-line tool used for managing the lifecycle of a Cluster API management cluster. Learn more about `clusterctl` and its commands from the official documentation of Cluster API [here](https://cluster-api.sigs.k8s.io/clusterctl/overview).

## Preparation

You have two options: either create a pure HCloud cluster or a hybrid cluster with Hetzner dedicated (bare metal) servers. For a full list of flavors, please check out the [release page](https://github.com/syself/cluster-api-provider-hetzner/releases). In the quickstart guide, we will go with the cluster creation on a pure Hetzner Cloud server.

To create a workload cluster, we need to do some preparation:

- Set up the projects and credentials in HCloud.
- Create the management/bootstrap cluster.
- Export variables needed for cluster-template.
- Create a secret with the credentials.

### Preparation of the Hetzner Project and Credentials

There are several tasks that have to be completed before a workload cluster can be created.

#### Preparing Hetzner Cloud

1. Create a new [HCloud project](https://console.hetzner.cloud/projects).
1. Generate an API token with read and write access. You'll find this if you click on the project and go to "security".
1. If you want to use it, generate an SSH key, upload the public key to HCloud (also via "security"), and give it a name. Read more about [Managing SSH Keys](managing-ssh-keys.md).

### Bootstrap or Management Cluster Installation

#### Common Prerequisites

- Install and setup kubectl in your local environment
- Install Kind and Docker

#### Install and configure a Kubernetes cluster

Cluster API requires an existing Kubernetes cluster accessible via kubectl. During the installation process, the Kubernetes cluster will be transformed into a management cluster by installing the Cluster API provider components, so it is recommended to keep it separated from any application workload.

It is a common practice to create a temporary, local bootstrap cluster, which is then used to provision a target management cluster on the selected infrastructure provider.

### Choose one of the options below:

#### 1. Existing Management Cluster.

For production use, a “real” Kubernetes cluster should be used with appropriate backup and DR policies and procedures in place. The Kubernetes cluster must be at least a [supported version](../../README.md#fire-compatibility-with-cluster-api-and-kubernetes-versions).

#### 2. Kind.

[kind](https://kind.sigs.k8s.io/) can be used for creating a local Kubernetes cluster for development environments or for the creation of a temporary bootstrap cluster used to provision a target management cluster on the selected infrastructure provider.

---
### Install Clusterctl and initialize Management Cluster

#### Install Clusterctl
To install Clusterctl, refer to the instructions available in the official ClusterAPI documentation [here](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl).
Alternatively, use the `make install-clusterctl` command to do the same.

#### Initialize the management cluster
Now that we’ve got clusterctl installed and all the prerequisites are in place, we can transform the Kubernetes cluster into a management cluster by using the `clusterctl init` command. More information about clusterctl can be found [here](https://cluster-api.sigs.k8s.io/clusterctl/commands/commands.html).

For the latest version:

```shell
clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
```

>Note: For a specific version, use the `--infrastructure hetzner:vX.X.X` flag with the above command.

---
### Variable Preparation to generate a cluster-template

```shell
export HCLOUD_SSH_KEY="<ssh-key-name>" \
export CLUSTER_NAME="my-cluster" \
export HCLOUD_REGION="fsn1" \
export CONTROL_PLANE_MACHINE_COUNT=3 \
export WORKER_MACHINE_COUNT=3 \
export KUBERNETES_VERSION=1.29.4 \
export HCLOUD_CONTROL_PLANE_MACHINE_TYPE=cpx31 \
export HCLOUD_WORKER_MACHINE_TYPE=cpx31
```

* **HCLOUD_SSH_KEY**: The SSH Key name you loaded in HCloud.
* **HCLOUD_REGION**: The region of the Hcloud cluster. Find the full list of regions [here](https://docs.hetzner.com/cloud/general/locations/).
* **HCLOUD_IMAGE_NAME**: The Image name of the operating system.
* **HCLOUD_X_MACHINE_TYPE**: The type of the Hetzner cloud server. Find more information [here](https://www.hetzner.com/cloud#pricing).

For a list of all variables needed for generating a cluster manifest (from the cluster-template.yaml), use the following command:

````shell
clusterctl generate cluster my-cluster --list-variables
````
Running the above command will give you an output in the following manner:

```shell
Required Variables:
  - HCLOUD_CONTROL_PLANE_MACHINE_TYPE
  - HCLOUD_REGION
  - HCLOUD_SSH_KEY
  - HCLOUD_WORKER_MACHINE_TYPE

Optional Variables:
  - CLUSTER_NAME                 (defaults to my-cluster)
  - CONTROL_PLANE_MACHINE_COUNT  (defaults to 1)
  - WORKER_MACHINE_COUNT         (defaults to 0)
```

### Create a secret for hcloud only

In order for the provider integration hetzner to communicate with the Hetzner API ([HCloud API](https://docs.hetzner.cloud/)), we need to create a secret with the access data. The secret must be in the same namespace as the other CRs.

`export HCLOUD_TOKEN="<YOUR-TOKEN>" `

- HCLOUD_TOKEN: The project where your cluster will be placed. You have to get a token from your HCloud Project.

Use the below command to create the required secret with the access data:

```shell
kubectl create secret generic hetzner --from-literal=hcloud=$HCLOUD_TOKEN
```

Patch the created secret so that it can be automatically moved to the target cluster later. The following command helps you do that:

```shell
kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
```

The secret name and the tokens can also be customized in the cluster template.

## Generating the cluster.yaml

The `clusterctl generate cluster` command returns a YAML template for creating a workload cluster.
It generates a YAML file named `my-cluster.yaml` with a predefined list of Cluster API objects (`Cluster`, `Machines`, `MachineDeployments`, etc.) to be deployed in the current namespace. 

```shell
clusterctl generate cluster my-cluster --kubernetes-version v1.29.4 --control-plane-machine-count=3 --worker-machine-count=3  > my-cluster.yaml
```
>Note: With the `--target-namespace` flag, you can specify a different target namespace.
Run the `clusterctl generate cluster --help` command for more information.

>**Note**: Please note that ready-to-use Kubernetes configurations, production-ready node images, kubeadm configuration, cluster add-ons like CNI, and similar services need to be separately prepared or acquired to ensure a comprehensive and secure Kubernetes deployment. This is where **Syself Autopilot** comes into play, taking on these challenges to offer you a seamless, worry-free Kubernetes experience. Feel free to contact us via e-mail: <info@syself.com>.

## Applying the workload cluster

The following command applies the configuration of the workload cluster:

```shell
kubectl apply -f my-cluster.yaml
```

### Accessing the workload cluster

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

>Note: The control plane won’t be `ready` until we install a CNI in the next step.

After the first control plane node is up and running, we can retrieve the kubeconfig of the workload cluster with:

```shell
export CAPH_WORKER_CLUSTER_KUBECONFIG=/tmp/workload-kubeconfig
clusterctl get kubeconfig my-cluster > $CAPH_WORKER_CLUSTER_KUBECONFIG
```

## Deploying the CNI solution

Cilium is used as a CNI solution in this guide. The following command deploys it to your cluster:

```shell
helm repo add cilium https://helm.cilium.io/

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install cilium cilium/cilium --version 1.15.4 \
--namespace kube-system \
-f templates/cilium/cilium.yaml
```

You can, of course, also install an alternative CNI, e.g., calico.

>Note: There is a bug in Ubuntu that requires the older version of Cilium for this quickstart guide.

## Deploy the CCM

### Deploy HCloud Cloud Controller Manager - _hcloud only_

The following `make` command will install the CCM in your workload cluster:

`make install-ccm-in-wl-cluster PRIVATE_NETWORK=false`


For a cluster without a private network, use the following command:

```shell
helm repo add syself https://charts.syself.com
helm repo update syself

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install ccm syself/ccm-hcloud --version 1.0.11 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
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

## Next Steps

### Switching to the workload cluster

As a next step, you need to switch to the workload cluster and the below command will do it:

```shell
export KUBECONFIG=/tmp/workload-kubeconfig
```

### Moving components

To move the Cluster API objects from your bootstrap cluster to the new management cluster, firstly you need to install the Cluster API controllers. To install the components with the latest version, run the below command:

```shell
clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
```

>Note: For a specific version, use the flag `--infrastructure hetzner:vX.X.X` with the above command.

You can switch back to the management cluster with the following command:

```shell
export KUBECONFIG=~/.kube/config
```

Move the objects into the new cluster by using:

```shell
clusterctl move --to-kubeconfig $CAPH_WORKER_CLUSTER_KUBECONFIG
```

Clusterctl Flags:

| Flag                      | Description                                                                                                                   |
| ------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| _--namespace_             | The namespace where the workload cluster is hosted. If unspecified, the current context's namespace is used.                  |
| _--kubeconfig_            | Path to the kubeconfig file for the source management cluster. If unspecified, default discovery rules apply.                 |
| _--kubeconfig-context_    | Context to be used within the kubeconfig file for the source management cluster. If empty, the current context will be used.      |
| _--to-kubeconfig_         | Path to the kubeconfig file to use for the destination management cluster.                                                    |
| _--to-kubeconfig-context_ | Context to be used within the kubeconfig file for the destination management cluster. If empty, the current context will be used. |
