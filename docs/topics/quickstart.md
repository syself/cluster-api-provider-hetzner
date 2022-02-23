# Installation

## Common Prerequisites
* Install and setup kubectl in your local environment
* Install Kind and Docker

## Install and/or configure a Kubernetes cluster
Cluster API requires an existing Kubernetes cluster accessible via kubectl. During the installation process the Kubernetes cluster will be transformed into a management cluster by installing the Cluster API provider components, so it is recommended to keep it separated from any application workload.

It is a common practice to create a temporary, local bootstrap cluster which is then used to provision a target management cluster on the selected infrastructure provider.

## Choose one of the options below:

### 1. Existing Management Cluster.
For production use-cases a “real” Kubernetes cluster should be used with appropriate backup and DR policies and procedures in place. The Kubernetes cluster must be at least a [supported version](../../README.md#compatibility-with-cluster-api-and-kubernetes-versions).
### 2. Kind. 
kind can be used for creating a local Kubernetes cluster for development environments or for the creation of a temporary bootstrap cluster used to provision a target management cluster on the selected infrastructure provider.

    
## Install clusterctl

Please use the instructions here: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl
or use: `make install-clusterctl`


## Initialize the management cluster
Now that we’ve got clusterctl installed and all the prerequisites in place, let’s transform the Kubernetes cluster into a management cluster by using `clusterctl init`. More informations about clusterctl can be found [here](https://cluster-api.sigs.k8s.io/clusterctl/commands/commands.html).


### Initialization of the cluster-api components

For the latest version:
```shell
clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner

```
or for a specific version: `--infrastructure hetzner:vX.X.X`

## HA Cluster API Components (optional)
The clusterctl CLI will create all the four needed components cluster-api (CAPI), cluster-api-bootstrap-provider-kubeadm (CAPBK), cluster-api-control-plane-kubeadm (KCP) and cluster-api-provider-hetzner (CAPH).
It uses the respective *-components.yaml from the releases. However, these are not highly available. By scaling the components we can at least reduce the probability of failure. For whom this is not enough could add anti-affinity rules and PDBs.

Scale up the deployments
```shell
kubectl -n capi-system scale deployment capi-controller-manager --replicas=2

kubectl -n capi-kubeadm-bootstrap-system scale deployment capi-kubeadm-bootstrap-controller-manager --replicas=2

kubectl -n capi-kubeadm-control-plane-system scale deployment capi-kubeadm-control-plane-controller-manager --replicas=2

kubectl -n cluster-api-provider-hetzner-system scale deployment caph-controller-manager --replicas=2

```

---
## Create your first workload cluster
Once the management cluster is ready, you can create your first workload cluster.

### Preparing the workload cluster configuration
To create a workload cluster we need to do some preparation:
1. first we need a HCloud project
2. we need to generate an API token with read & write rights.
3. we need to generate a ssh key, upload the public key to HCloud and give it a name.

We export the HCloud token as environment variable to use it later. We do the same with our SSH key name. 

#### Required configuration for hetzner provider

```shell
export HCLOUD_TOKEN="<YOUR-TOKEN>" \
export HCLOUD_SSH_KEY="<ssh-key-name>" \
export HCLOUD_IMAGE_NAME=test-image \
export CLUSTER_NAME="my-cluster" \
export HCLOUD_REGION="fsn1" \
export CONTROL_PLANE_MACHINE_COUNT=3 \
export WORKER_MACHINE_COUNT=3 \
export KUBERNETES_VERSION=1.23.4 \
export HCLOUD_CONTROL_PLANE_MACHINE_TYPE=cpx31 \
export HCLOUD_WORKER_MACHINE_TYPE=cpx31 
```
HCLOUD_TOKEN: The project where your cluster will be placed to. You have to get a token from your HCloud Project.
HCLOUD_SSH_KEY: The SSH Key name you loaded in HCloud.
HCLOUD_IMAGE_NAME: The Image name of your operating system. 

For a list of all variables need for generating a cluster manifest (from the cluster-template.yaml) use `clusterctl generate cluster my-cluster --list-variables`:
```
Required Variables:
  - HCLOUD_CONTROL_PLANE_MACHINE_TYPE
  - HCLOUD_IMAGE_NAME
  - HCLOUD_WORKER_MACHINE_TYPE
  - HCLOUD_REGION
  - HCLOUD_SSH_KEY

Optional Variables:
  - CLUSTER_NAME                 (defaults to my-cluster)
  - CONTROL_PLANE_MACHINE_COUNT  (defaults to 1)
  - KUBERNETES_VERSION           
  - WORKER_MACHINE_COUNT         (defaults to 1)
```

#### Create a secret for the hetzner provider.

In order for the provider integration hetzner to communicate with the Hetzner API ([HCloud API](https://docs.hetzner.cloud/) + [Robot API](https://robot.your-server.de/doc/webservice/en.html#preface)), we need to create a secret with the access data. The secret must be in the same namespace as the other CRs.

```shell
kubectl create secret generic hetzner --from-literal=hcloud=$HCLOUD_TOKEN

# Patch the created secret so it is automatically moved to the target cluster later.
kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
``` 
The secret name and the tokens can also be customized in the cluster template, however, this is out of scope of the quickstart guide.

### Creating a viable Node Image
For using cluster-api with the bootstrap provider kubeadm, we need a server with all the necessary binaries and settings for running kubernetes.
There are several ways to achieve this. Here in this quick-start guide we use pre-kubeadm commands in the KubeadmControlPlane and KubeadmConfigTemplate object. These are propagated from the bootstrap provider kubeadm and the control plane provider kubeadm to the node as cloud-init commands. This way is usable universally also in other infrastructure providers. 
For Hcloud there is an alternative way using packer, that creates a snapshot to boot from, this is in the sense of versioning and the speed of creating a node clearly advantageous.

### Generate your cluster.yaml
The clusterctl generate cluster command returns a YAML template for creating a workload cluster.
Generates a YAML file named my-cluster.yaml with a predefined list of Cluster API objects; Cluster, Machines, Machine Deployments, etc. to be deployed in the current namespace (in case, use the --target-namespace flag to specify a different target namespace).
See also `clusterctl generate cluster --help`.

```shell
clusterctl generate cluster my-cluster --kubernetes-version v1.23.4 --control-plane-machine-count=3 --worker-machine-count=3  > my-cluster.yaml
```

To use for example the hcloud network use a flavor:
```shell
clusterctl generate cluster my-cluster --kubernetes-version v1.23.4 --control-plane-machine-count=3 --worker-machine-count=3  --flavor hcloud-network > my-cluster.yaml
```

For a full list of flavors please check out the [release page](https://github.com/syself/cluster-api-provider-hetzner/releases) all cluster-templates starts with `cluster-template-`. The flavor name is the suffix.

### Apply the workload cluster
```shell
kubectl apply -f my-cluster.yaml
```

### Accessing the workload cluster
The cluster will now start provisioning. You can check status with:
```shell
kubectl get cluster
```
You can also get an “at glance” view of the cluster and its resources by running:
```shell
clusterctl describe cluster my-cluster
```
To verify the first control plane is up:
```shell
kubectl get kubeadmcontrolplane
```
> The control plane won’t be Ready until we install a CNI in the next step.

After the first control plane node is up and running, we can retrieve the workload cluster Kubeconfig:
```shell
export CAPH_WORKER_CLUSTER_KUBECONFIG=/tmp/workload-kubeconfig
clusterctl get kubeconfig my-cluster > $CAPH_WORKER_CLUSTER_KUBECONFIG
```

### Deploy a CNI solution
```shell
helm repo add cilium https://helm.cilium.io/

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install cilium cilium/cilium --version 1.11.1 \
--namespace kube-system \
-f templates/cilium/cilium.yaml
```

Of course feel free to install an alternative CNI like calico.
### Deploy HCloud Cloud Controller Manager

For a cluster without private network: 

```shell
helm repo add syself https://charts.syself.com

KUBECONFIG=$CAPH_WORKER_CLUSTER_KUBECONFIG helm upgrade --install ccm syself/ccm-hcloud --version 1.0.9 \
--namespace kube-system \
--set privateNetwork.enabled=false
```

### Deploy the CSI (optional)
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

Delete workload cluster.
```shell
kubectl delete cluster my-cluster
```
> **IMPORTANT**: In order to ensure a proper cleanup of your infrastructure you must always delete the cluster object. Deleting the entire cluster template with kubectl delete -f capi-quickstart.yaml might lead to pending resources to be cleaned up manually.

Delete management cluster
```shell
kind delete cluster
```

## Next Steps

### Switch to the workload cluster

```shell
export KUBECONFIG=/tmp/workload-kubeconfig
```
### Moving components

In the target cluster run: 

For the latest version:
```shell
clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner

```
or for a specific version: `--infrastructure hetzner:vX.X.X`

Then switch back to the management cluster!

For example with: 
```shell
export KUBECONFIG=~/.kube/config
```

In the management cluster do:
```shell
clusterctl move --to-kubeconfig $CAPH_WORKER_CLUSTER_KUBECONFIG
```
Clusterctl Flags:

| Flag | Description |
| ---- | ----------- |
|*--namespace* | The namespace where the workload cluster is hosted. If unspecified, the current context's namespace is used. |
| *--kubeconfig*| Path to the kubeconfig file for the source management cluster. If unspecified, default discovery rules apply. |
|*--kubeconfig-context*| Context to be used within the kubeconfig file for the source management cluster. If empty, current context will be used. |
|*--to-kubeconfig*| Path to the kubeconfig file to use for the destination management cluster. |
|*--to-kubeconfig-context*| Context to be used within the kubeconfig file for the destination management cluster. If empty, current context will be used.|