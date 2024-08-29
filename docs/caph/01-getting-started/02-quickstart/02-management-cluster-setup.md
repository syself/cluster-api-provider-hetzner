---
title: Management cluster setup
---

You have two options: either create a pure HCloud cluster or a hybrid cluster with Hetzner dedicated (bare metal) servers. For a full list of flavors, please check out the [release page](https://github.com/syself/cluster-api-provider-hetzner/releases). In the quickstart guide, we will go with the cluster creation on a pure Hetzner Cloud server.

To create a workload cluster, we need to do some preparation:

- Set up the projects and credentials in HCloud.
- Create the management/bootstrap cluster.
- Export variables needed for cluster-template.
- Create a secret with the credentials.

## Preparation of the Hetzner Project and Credentials

There are several tasks that have to be completed before a workload cluster can be created.

### Preparing Hetzner Cloud

1. Create a new [HCloud project](https://console.hetzner.cloud/projects).
1. Generate an API token with read and write access. You'll find this if you click on the project and go to "security".
1. If you want to use it, generate an SSH key, upload the public key to HCloud (also via "security"), and give it a name. Read more about [Managing SSH Keys](/docs/caph/02-topics/01-managing-ssh-keys.md).

## Bootstrap or Management Cluster Installation

### Common Prerequisites

- Install and setup kubectl in your local environment
- Install Kind and Docker

### Install and configure a Kubernetes cluster

Cluster API requires an existing Kubernetes cluster accessible via kubectl. During the installation process, the Kubernetes cluster will be transformed into a management cluster by installing the Cluster API provider components, so it is recommended to keep it separated from any application workload.

It is a common practice to create a temporary, local bootstrap cluster, which is then used to provision a target management cluster on the selected infrastructure provider.

## Choose one of the options below

### 1. Existing Management Cluster

For production use, a “real” Kubernetes cluster should be used with appropriate backup and DR policies and procedures in place. The Kubernetes cluster must be at least a [supported version](https://github.com/syself/cluster-api-provider-hetzner/blob/main/README.md#%EF%B8%8F-compatibility-with-cluster-api-and-kubernetes-versions).

### 2. Kind

[kind](https://kind.sigs.k8s.io/) can be used for creating a local Kubernetes cluster for development environments or for the creation of a temporary bootstrap cluster used to provision a target management cluster on the selected infrastructure provider.

---

## Install Clusterctl and initialize Management Cluster

### Install Clusterctl

To install Clusterctl, refer to the instructions available in the official ClusterAPI documentation [here](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl).

### Initialize the management cluster

Now that we’ve got clusterctl installed and all the prerequisites are in place, we can transform the Kubernetes cluster into a management cluster by using the `clusterctl init` command. More information about clusterctl can be found [here](https://cluster-api.sigs.k8s.io/clusterctl/commands/commands.html).

For the latest version:

```shell
clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
```

{% callout %}

For a specific version, use the `--infrastructure hetzner:vX.X.X` flag with the above command.

{% /callout %}

---

## Variable Preparation to generate a cluster-template

```shell
export SSH_KEY_NAME="<ssh-key-name>" \
export CLUSTER_NAME="my-cluster" \
export HCLOUD_REGION="fsn1" \
export CONTROL_PLANE_MACHINE_COUNT=3 \
export WORKER_MACHINE_COUNT=3 \
export KUBERNETES_VERSION=1.29.4 \
export HCLOUD_CONTROL_PLANE_MACHINE_TYPE=cpx31 \
export HCLOUD_WORKER_MACHINE_TYPE=cpx31
```

- **SSH_KEY_NAME**: The SSH Key name you loaded in HCloud.
- **HCLOUD_REGION**: The region of the Hcloud cluster. Find the full list of regions [here](https://docs.hetzner.com/cloud/general/locations/).
- **HCLOUD_IMAGE_NAME**: The Image name of the operating system.
- **HCLOUD_X_MACHINE_TYPE**: The type of the Hetzner cloud server. Find more information [here](https://www.hetzner.com/cloud#pricing).

For a list of all variables needed for generating a cluster manifest (from the cluster-template.yaml), use the following command:

```shell
clusterctl generate cluster my-cluster --list-variables
```

Running the above command will give you an output in the following manner:

```shell
Required Variables:
  - HCLOUD_CONTROL_PLANE_MACHINE_TYPE
  - HCLOUD_REGION
  - SSH_KEY_NAME
  - HCLOUD_WORKER_MACHINE_TYPE

Optional Variables:
  - CLUSTER_NAME                 (defaults to my-cluster)
  - CONTROL_PLANE_MACHINE_COUNT  (defaults to 1)
  - WORKER_MACHINE_COUNT         (defaults to 0)
```

## Create a secret for hcloud only

In order for the provider integration hetzner to communicate with the Hetzner API ([HCloud API](https://docs.hetzner.cloud/)), we need to create a secret with the access data. The secret must be in the same namespace as the other CRs.

`export HCLOUD_TOKEN="<YOUR-TOKEN>"`

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
