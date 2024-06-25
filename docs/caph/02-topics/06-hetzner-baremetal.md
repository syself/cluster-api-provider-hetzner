---
title: Hetzner Baremetal
---

Hetzner have two offerings primarily:

1. `Hetzner Cloud`/`Hcloud` for virtualized servers
2. `Hetzner Dedicated`/`Robot` for bare metal servers

In this guide, we will focus on creating a cluster from baremetal servers.

## Flavors of Hetzner Baremetal

Now, there are different ways you can use baremetal servers, you can use them as controlplanes or as worker nodes or both. Based on that we have created some templates and those templates are released as flavors in GitHub releases.

These flavors can be consumed using [clusterctl](https://main.cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl) tool:

To use bare metal servers for your deployment, you can choose one of the following flavors:

| Flavor                                         | What it does                                                                                                  |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `hetzner-baremetal-control-planes-remediation` | Uses bare metal servers for the control plane nodes - with custom remediation (try to reboot machines first)  |
| `hetzner-baremetal-control-planes`             | Uses bare metal servers for the control plane nodes - with normal remediation (unprovision/recreate machines) |
| `hetzner-hcloud-control-planes`                | Uses the hcloud servers for the control plane nodes and the bare metal servers for the worker nodes           |

{% callout %}

These flavors are only for demonstration purposes and should not be used in production.

{% /callout %}

## Purchasing Bare Metal Servers

If you want to create a cluster with bare metal servers, you will also need to set up the robot credentials. For setting robot credentials, as described in the [reference](/docs/caph/03-reference/06-hetzner-bare-metal-machine-template), you need to purchase bare metal servers beforehand manually.

## Creating a bootstrap cluster

In this guide, we will focus on creating a bootstrap cluster which is basically a local management cluster created using [kind](https://kind.sigs.k8s.io).

To create a bootstrap cluster, you can use the following command:

```shell
kind create cluster
```

```shell
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.29.2) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind

Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
```

After creating the bootstrap cluster, it is also required to have some variables exported and the name of the variables that needs to be exported can be known by running the following command:

```shell
$ clusterctl generate cluster my-cluster --list-variables --flavor hetzner-hcloud-control-planes
Required Variables:
  - HCLOUD_CONTROL_PLANE_MACHINE_TYPE
  - HCLOUD_REGION
  - HCLOUD_SSH_KEY
  - HCLOUD_WORKER_MACHINE_TYPE

Optional Variables:
  - CLUSTER_NAME                 (defaults to my-cluster)
  - CONTROL_PLANE_MACHINE_COUNT  (defaults to 3)
  - KUBERNETES_VERSION           (defaults to v1.29.4)
  - WORKER_MACHINE_COUNT         (defaults to 3)
```

These variables are used during the deployment of Hetzner infrastructure provider in the cluster.

Installing the Hetzner provider can be done using the following command:

```shell
clusterctl init --infrastructure hetzner
```

```shell
Fetching providers
Installing cert-manager Version="v1.14.2"
Waiting for cert-manager to be available...
Installing Provider="cluster-api" Version="v1.7.1" TargetNamespace="capi-system"
Installing Provider="bootstrap-kubeadm" Version="v1.7.1" TargetNamespace="capi-kubeadm-bootstrap-system"
Installing Provider="control-plane-kubeadm" Version="v1.7.1" TargetNamespace="capi-kubeadm-control-plane-system"
Installing Provider="infrastructure-hetzner" Version="v1.0.0-beta.33" TargetNamespace="caph-system"

Your management cluster has been initialized successfully!

You can now create your first workload cluster by running the following:

  clusterctl generate cluster [name] --kubernetes-version [version] | kubectl apply -f -
```

## Generating Workload Cluster Manifest

Once the infrastructure provider is ready, we can create a workload cluster manifest using `clusterctl generate`

```shell
clusterctl generate cluster my-cluster --flavor hetzner-hcloud-control-planes > my-cluster.yaml
```

As of now, our cluster manifest lives in `my-cluster.yaml` file and we will apply this at a later stage after preparing secrets and ssh-keys.

## Preparing Hetzner Robot

1. Create a new web service user. [Here](https://robot.your-server.de/preferences/index), you can define a password and copy your user name.
2. Generate an SSH key. You can either upload it via Hetzner Robot UI or just rely on the controller to upload a key that it does not find in the robot API. You have to store the public and private key together with the SSH key's name in a secret that the controller reads.

For this tutorial, we will let the controller upload keys to hetzner robot.

### Creating new user in Robot

To create new user in Robot, click on the `Create User` button in the Hetzner Robot console. Once you create the new user, a user ID will be provided to you via email from Hetzner Robot. The password will be the same that you used while creating the user.

![robot user](https://syself.com/images/robot-user.png)

This is a required for following the next step.

## Creating and verify ssh-key in hcloud

First you need to create a ssh-key locally and you can `ssh-keygen` command for creation.

```shell
ssh-keygen -t ed25519 -f ~/.ssh/caph
```

Above command will create a public and private key in your `~/.ssh` directory.

You can use the public key `~/.ssh/caph.pub` and upload it to your hcloud project. Go to your project and under `Security` -> `SSH Keys` click on `Add SSH key` and add your public key there and in the `Name` of ssh key you'll use the name `test`.

{% callout %}

There is also a helper CLI called [hcloud](https://github.com/hetznercloud/cli) that can be used for the purpose of uploading the SSH key.

{% /callout %}

In the above step, the name of the ssh-key that is recognized by hcloud is `test`. This is important because we will reference the name of the ssh-key later.

This is an important step because the same ssh key is used to access the servers. Make sure you are using the correct ssh key name.

The `test` is the name of the ssh key that we have created above. It is because the generated manifest references `test` as the ssh key name.

```yaml
sshKeys:
  hcloud:
    - name: test
```

{% callout %}

If you want to use some other name then you can modify it accordingly.

{% /callout %}

## Create Secrets In Management Cluster (Hcloud + Robot)

In order for the provider integration hetzner to communicate with the Hetzner API ([HCloud API](https://docs.hetzner.cloud/) + [Robot API](https://robot.your-server.de/doc/webservice/en.html#preface)), we need to create secrets with the access data. The secret must be in the same namespace as the other CRs.

We create two secrets named `hetzner` for Hetzner Cloud and Robot API access and `robot-ssh` for provisioning bare metal servers via SSH.
The `hetzner` secret contains API token for hcloud token. It also contains username and password that is used to interact with robot API. `robot-ssh` secret contains the public-key, private-key and name of the ssh-key used for baremetal servers.

```shell
export HCLOUD_TOKEN="<YOUR-TOKEN>" \
export HETZNER_ROBOT_USER="<YOUR-ROBOT-USER>" \
export HETZNER_ROBOT_PASSWORD="<YOUR-ROBOT-PASSWORD>" \
export HETZNER_SSH_PUB_PATH="<YOUR-SSH-PUBLIC-PATH>" \
export HETZNER_SSH_PRIV_PATH="<YOUR-SSH-PRIVATE-PATH>"
```

- `HCLOUD_TOKEN`: The project where your cluster will be placed. You have to get a token from your HCloud Project.
- `HETZNER_ROBOT_USER`: The User you have defined in Robot under settings/web.
- `HETZNER_ROBOT_PASSWORD`: The Robot Password you have set in Robot under settings/web.
- `HETZNER_SSH_PUB_PATH`: The Path to your generated Public SSH Key.
- `HETZNER_SSH_PRIV_PATH`: The Path to your generated Private SSH Key. This is needed because CAPH uses this key to provision the node in Hetzner Dedicated.

```shell
kubectl create secret generic hetzner --from-literal=hcloud=$HCLOUD_TOKEN --from-literal=robot-user=$HETZNER_ROBOT_USER --from-literal=robot-password=$HETZNER_ROBOT_PASSWORD

kubectl create secret generic robot-ssh --from-literal=sshkey-name=test --from-file=ssh-privatekey=$HETZNER_SSH_PRIV_PATH --from-file=ssh-publickey=$HETZNER_SSH_PUB_PATH
```

{% callout %}

`sshkey-name` should must match the name that is present in hetzner otherwise the controller will not know how to reach the machine.

{% /callout %}

Patch the created secrets so that they get automatically moved to the target cluster later. The following command helps you do that:

```shell
kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
kubectl patch secret robot-ssh -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
```

The secret name and the tokens can also be customized in the cluster template.

## Creating Host Object In Management Cluster

For using baremetal servers as nodes, you need to create a `HetznerBareMetalHost` object for each bare metal server that you bought and specify its server ID in the specs. Below is a sample manifest for HetznerBareMetalHost object.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerBareMetalHost
metadata:
  name: "caph-baremetal-server"
  namespace: default
spec:
  description: CAPH BareMetal Server
  serverID: <ID-of-your-server> # please check robot console
  rootDeviceHints:
    wwn: <wwn>
  maintenanceMode: false
```

If you already know the WWN of the storage device you want to choose for booting, specify it in the `rootDeviceHints` of the object. If not, you can proceed. During the provisioning process, the controller will fetch information about all available storage devices and store it in the status of the object.

For example, let's consider a `HetznerBareMetalHost` object without specify it's WWN.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerBareMetalHost
metadata:
  name: "caph-baremetal-server"
  namespace: default
spec:
  description: CAPH BareMetal Server
  serverID: <ID-of-your-server> # please check robot console
  maintenanceMode: false
```

In the above server, we have not specified the WWN of the server and we have applied it in the cluster.

After a while, you will see that there is an error in provisioning of `HetznerBareMetalHost` object that you just applied above. The error will look the following:

```shell
$ kubectl get hetznerbaremetalhost -A
default     my-cluster-md-1-tgvl5   my-cluster   default/test-bm-gpu    my-cluster-md-1-t9znj-694hs   Provisioning   23m   ValidationFailed   no root device hints specified
```

After you see the error, get the YAML output of the `HetznerBareMetalHost` object and then you will find the list of storage devices and their `wwn` in the status of the `HetznerBareMetalHost` resource.

```yaml
storage:
  - hctl: "2:0:0:0"
    model: Micron_1100_MTFDDAK512TBN
    name: sda
    serialNumber: 18081BB48B25
    sizeBytes: 512110190592
    sizeGB: 512
    vendor: "ATA     "
    wwn: "0x500a07511bb48b25"
  - hctl: "1:0:0:0"
    model: Micron_1100_MTFDDAK512TBN
    name: sdb
    serialNumber: 18081BB48992
    sizeBytes: 512110190592
    sizeGB: 512
    vendor: "ATA     "
    wwn: "0x500a07511bb48992"
```

In the output above, we can see that on this baremetal servers we have two disk with their respective `Wwn`. We can also verify it by making an ssh connection to the rescue system and executing the following command:

```shell
# lsblk --nodeps --output name,type,wwn
NAME TYPE WWN
sda  disk 0x500a07511bb48992
sdb  disk 0x500a07511bb48b25
```

Since, we are now confirmed about wwn of the two disks, we can use either of them. We will use `kubectl edit` and update the following information in the `HetznerBareMetalHost` object.

{% callout %}

Defining `rootDeviceHints` on your baremetal server is important otherwise the baremetal server will not be able join the cluster.

{% /callout %}

```yaml
rootDeviceHints:
  wwn: "0x500a07511bb48992"
```

{% callout %}

If you've more than one disk then it's recommended to use smaller disk for OS installation so that we can retain the data in between provisioning of machine.

{% /callout %}

We will apply this file in the cluster and the provisioning of the machine will be successful.

To summarize, if you don't know the WWN of your server then there are two ways to find it out:

1. Create the HetznerBareMetalHost without WWN and wait for the controller to fetch all information about the available storage devices. Afterwards, look at status of `HetznerBareMetalHost` by running `kubectl get hetznerbaremetalhost <name-of-hetzner-baremetalhost> -o yaml` in your management cluster. There you will find `hardwareDetails` of all of your bare metal hosts, in which you can see a list of all the relevant storage devices as well as their properties. You can copy+paste the WWN of your desired storage device into the `rootDeviceHints` of your `HetznerBareMetalHost` objects.
2. SSH into the rescue system of the server and use `lsblk --nodeps --output name,type,wwn`

{% callout %}

There might be cases where you've more than one disk.

{% /callout %}

```shell
lsblk -d -o name,type,wwn,size
NAME TYPE WWN                  SIZE
sda  disk <wwn>                238.5G
sdb  disk <wwn>                238.5G
sdc  disk <wwn>                  1.8T
sdd  disk <wwn>                  1.8T
```

In the above case, you can use any of the four disks available to you on a baremetal server.

## Creating Workload Cluster

{% callout %}

Secrets as of now are hardcoded given we are using a flavor which is essentially a template. If you want to use your own naming convention for secrets then you'll have to update the templates. Please make sure that you pay attention to the sshkey name.

{% /callout %}

Since we have already created secret in hetzner robot, hcloud and ssh-keys as secret in management cluster, we can now apply the cluster.

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

$ helm upgrade --install ccm syself/ccm-hetzner --version 1.1.10 \
              --namespace kube-system \
              --set privateNetwork.enabled=false \
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
$ helm install cilium cilium/cilium --version 1.15.3 --kubeconfig workload-kubeconfig
NAME: cilium
LAST DEPLOYED: Thu Apr  4 21:11:13 2024
NAMESPACE: default
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
You have successfully installed Cilium with Hubble.

Your release version is 1.15.3.

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
default     my-cluster-control-plane-6m6zf   my-cluster   my-cluster-control-plane-84hsn   hcloud://45443706     Running        10h   v1.29.4
default     my-cluster-control-plane-m6frm   my-cluster   my-cluster-control-plane-hvl5d   hcloud://45443651     Running        10h   v1.29.4
default     my-cluster-control-plane-qwsq6   my-cluster   my-cluster-control-plane-ss9kc   hcloud://45443746     Running        10h   v1.29.4
default     my-cluster-md-0-2xgj5-c5bhc      my-cluster   my-cluster-md-0-6xttr            hcloud://45443694     Running        10h   v1.29.4
default     my-cluster-md-0-2xgj5-rbnbw      my-cluster   my-cluster-md-0-fdq9l            hcloud://45443693     Running        10h   v1.29.4
default     my-cluster-md-0-2xgj5-tl2jr      my-cluster   my-cluster-md-0-59cgw            hcloud://45443692     Running        10h   v1.29.4
default     my-cluster-md-1-cp2fd-7nld7      my-cluster   bm-my-cluster-md-1-d7526         hcloud://bm-2317525   Running        9h    v1.29.4
default     my-cluster-md-1-cp2fd-n74sm      my-cluster   bm-my-cluster-md-1-l5dnr         hcloud://bm-2105469   Running        10h   v1.29.4
```

Please note that hcloud servers are prefixed with `hcloud://` and baremetal servers are prefixed with `hcloud://bm-`.

## Advanced

### Constant hostnames for bare metal servers

In some cases it has advantages to fix the hostname and with it the names of nodes in your clusters. For cloud servers not so much as for bare metal servers, where there are storage integrations that allow you to use the storage of the bare metal servers and that work with fixed node names.

Therefore, there is the possibility to create a cluster that uses fixed node names for bare metal servers. Please note: this only applies to the bare metal servers and not to Hetzner Cloud servers.

You can trigger this feature by creating a `Cluster` or `HetznerBareMetalMachine` (you can choose) with the annotation `"capi.syself.com/constant-bare-metal-hostname": "true"`. Of course, `HetznerBareMetalMachines` are not created by the user. However, if you use the `ClusterClass`, then you can add the annotation to a `MachineDeployment`, so that all machines are created with this annotation.

This is still an experimental feature but it should be safe to use and to also update existing clusters with this annotation. All new machines will be created with this constant hostname.
