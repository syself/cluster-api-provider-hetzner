# Preparation

## Preparation of the Hetzner Project and Credentials

There are several tasks that have to be completed before a workload cluster can be created.

### Preparing Hetzner Cloud

1. Create a new [HCloud project](https://console.hetzner.cloud/projects).
1. Generate an API token with read and write access. You'll find this if you click on the project and go to "security".
1. If you want to use it, generate an SSH key, upload the public key to HCloud (also via "security"), and give it a name. Read more about [Managing SSH Keys](managing-ssh-keys.md).

### Preparing Hetzner Robot

1. Create a new web service user. [Here](https://robot.your-server.de/preferences/index), you can define a password and copy your user name.
1. Generate an SSH key. You can either upload it via Hetzner Robot UI or just rely on the controller to upload a key that it does not find in the robot API. This is possible, as you have to store the public and private key together with the SSH key's name in a secret that the controller reads.

---
## Bootstrap or Management Cluster Installation

### Common Prerequisites

- Install and setup kubectl in your local environment
- Install Kind and Docker

### Install and configure a Kubernetes cluster

Cluster API requires an existing Kubernetes cluster accessible via kubectl. During the installation process, the Kubernetes cluster will be transformed into a management cluster by installing the Cluster API provider components, so it is recommended to keep it separated from any application workload.

It is a common practice to create a temporary, local bootstrap cluster, which is then used to provision a target management cluster on the selected infrastructure provider.

### Choose one of the options below:

#### 1. Existing Management Cluster.

For production use, a “real” Kubernetes cluster should be used with appropriate backup and DR policies and procedures in place. The Kubernetes cluster must be at least a [supported version](../../README.md#fire-compatibility-with-cluster-api-and-kubernetes-versions).

#### 2. Kind.

[kind](https://kind.sigs.k8s.io/) can be used for creating a local Kubernetes cluster for development environments or for the creation of a temporary bootstrap cluster used to provision a target management cluster on the selected infrastructure provider.

---
## Install Clusterctl and initialize Management Cluster

### Install Clusterctl
Please use the instructions here: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl
or use: `make install-clusterctl`

### Initialize the management cluster
Now that we’ve got clusterctl installed and all the prerequisites are in place, we can transform the Kubernetes cluster into a management cluster by using the `clusterctl init` command. More information about clusterctl can be found [here](https://cluster-api.sigs.k8s.io/clusterctl/commands/commands.html).

For the latest version:

```shell
clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner

```

or for a specific version: `--infrastructure hetzner:vX.X.X`

---
## Variable Preparation to generate a cluster-template.

```shell
export HCLOUD_SSH_KEY="<ssh-key-name>" \
export CLUSTER_NAME="my-cluster" \
export HCLOUD_REGION="fsn1" \
export CONTROL_PLANE_MACHINE_COUNT=3 \
export WORKER_MACHINE_COUNT=3 \
export KUBERNETES_VERSION=1.28.4 \
export HCLOUD_CONTROL_PLANE_MACHINE_TYPE=cpx31 \
export HCLOUD_WORKER_MACHINE_TYPE=cpx31
```

* HCLOUD_SSH_KEY: The SSH Key name you loaded in HCloud.
* HCLOUD_REGION: https://docs.hetzner.com/cloud/general/locations/
* HCLOUD_IMAGE_NAME: The Image name of your operating system.
* HCLOUD_X_MACHINE_TYPE: https://www.hetzner.com/cloud#pricing

For a list of all variables needed for generating a cluster manifest (from the cluster-template.yaml), use `clusterctl generate cluster my-cluster --list-variables`:

```
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

In order for the provider integration hetzner to communicate with the Hetzner API ([HCloud API](https://docs.hetzner.cloud/), we need to create a secret with the access data. The secret must be in the same namespace as the other CRs.

`export HCLOUD_TOKEN="<YOUR-TOKEN>" `
- HCLOUD_TOKEN: The project where your cluster will be placed. You have to get a token from your HCloud Project.

```shell
kubectl create secret generic hetzner --from-literal=hcloud=$HCLOUD_TOKEN

# Patch the created secret so it is automatically moved to the target cluster later.
kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
```

The secret name and the tokens can also be customized in the cluster template.

### Create a secret for Hetzner (Hcloud + Robot)

In order for the provider integration hetzner to communicate with the Hetzner API ([HCloud API](https://docs.hetzner.cloud/) + [Robot API](https://robot.your-server.de/doc/webservice/en.html#preface)), we need to create a secret with the access data. The secret must be in the same namespace as the other CRs.

```shell
export HCLOUD_TOKEN="<YOUR-TOKEN>" \
export HETZNER_ROBOT_USER="<YOUR-ROBOT-USER>" \
export HETZNER_ROBOT_PASSWORD="<YOUR-ROBOT-PASSWORD>" \
export HETZNER_SSH_PUB_PATH="<YOUR-SSH-PUBLIC-PATH>" \
export HETZNER_SSH_PRIV_PATH="<YOUR-SSH-PRIVATE-PATH>"
```

- HCLOUD_TOKEN: The project where your cluster will be placed. You have to get a token from your HCloud Project.
- HETZNER_ROBOT_USER: The User you have defined in Robot under settings/web.
- HETZNER_ROBOT_PASSWORD: The Robot Password you have set in Robot under settings/web.
- HETZNER_SSH_PUB_PATH: The Path to your generated Public SSH Key.
- HETZNER_SSH_PRIV_PATH: The Path to your generated Private SSH Key. This is needed because CAPH uses this key to provision the node in Hetzner Dedicated.

```shell
kubectl create secret generic hetzner --from-literal=hcloud=$HCLOUD_TOKEN --from-literal=robot-user=$HETZNER_ROBOT_USER --from-literal=robot-password=$HETZNER_ROBOT_PASSWORD

kubectl create secret generic robot-ssh --from-literal=sshkey-name=cluster --from-file=ssh-privatekey=$HETZNER_SSH_PRIV_PATH --from-file=ssh-publickey=$HETZNER_SSH_PUB_PATH

# Patch the created secrets so that they get automatically moved to the target cluster later.
kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
kubectl patch secret robot-ssh -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
```

The secret name and the tokens can also be customized in the cluster template.


### Creating a viable Node Image

For using cluster-API with the bootstrap provider kubeadm, we need a server with all the necessary binaries and settings for running Kubernetes.
There are several ways to achieve this. In the quick-start guide, we use `pre-kubeadm` commands in the KubeadmControlPlane and KubeadmConfigTemplate objects. These are propagated from the bootstrap provider kubeadm and the control plane provider kubeadm to the node as cloud-init commands. This way is usable universally also in other infrastructure providers.
For Hcloud, there is an alternative way of doing this using Packer. It creates a snapshot to boot from. This makes it easier to version the images, and creating new nodes using this image is faster. The same is possible for Hetzner Bare Metal, as we could use installimage and a prepared tarball, which then gets installed.

See [node-image](./node-image.md) for more information.
