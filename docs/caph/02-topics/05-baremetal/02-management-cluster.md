---
title: Management cluster setup for bare metal
metatitle: Configure Your Management Cluster for Handling Hetzner Bare Metal Servers
sidebar: Management cluster setup for baremetal
description: Learn how to provision a management cluster ready to manage bare metal servers.
---

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
  - SSH_KEY_NAME
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
Installing Provider="infrastructure-hetzner" Version="v1.0.0" TargetNamespace="caph-system"

Your management cluster has been initialized successfully!

You can now create your first workload cluster by running the following:

  clusterctl generate cluster [name] --kubernetes-version [version] | kubectl apply -f -
```

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

You can use the public key `~/.ssh/caph.pub` and upload it to your hcloud project. Go to your project and under `Security` -> `SSH Keys` click on `Add SSH key` and add your public key there and in the `Name` of ssh key you'll use the name `my-caph-ssh-key`.

{% callout %}

There is also a helper CLI called [hcloud](https://github.com/hetznercloud/cli) that can be used for the purpose of uploading the SSH key.

{% /callout %}

In the above step, the name of the ssh-key that is recognized by hcloud is `my-caph-ssh-key`. This is important because we will reference the name of the ssh-key later.

This is an important step because the same ssh key is used to access the servers. Make sure you are using the correct ssh key name.

The `my-caph-ssh-key` is the name of the ssh key that we have created above. It is because the generated manifest references `my-caph-ssh-key` as the ssh key name.

```yaml
sshKeys:
  hcloud:
    - name: my-caph-ssh-key
```

{% callout %}

If you want to use some other name then you can modify it accordingly.

{% /callout %}

## Create Secrets In Management Cluster (Hcloud + Robot)

In order for the provider integration hetzner to communicate with the Hetzner API ([HCloud API](https://docs.hetzner.cloud/) + [Robot API](https://robot.your-server.de/doc/webservice/en.html#preface)), we need to create secrets with the access data. The secret must be in the same namespace as the other CRs.

We create two secrets named `hetzner` for Hetzner Cloud and Robot API access and `robot-ssh` for provisioning bare metal servers via SSH.
The `hetzner` secret contains API token for hcloud token. It also contains username and password that is used to interact with robot API. `robot-ssh` secret contains the public-key, private-key and name of the ssh-key used for baremetal servers.

```shell
export HCLOUD_TOKEN="<YOUR-TOKEN>"
export HETZNER_ROBOT_USER="<YOUR-ROBOT-USER>"
export HETZNER_ROBOT_PASSWORD="<YOUR-ROBOT-PASSWORD>"
export SSH_KEY_NAME="<YOUR-SSH-KEY-NAME>"
export HETZNER_SSH_PUB_PATH="<YOUR-SSH-PUBLIC-PATH>"
export HETZNER_SSH_PRIV_PATH="<YOUR-SSH-PRIVATE-PATH>"
```

- `HCLOUD_TOKEN`: The project where your cluster will be placed. You have to get a token from your HCloud Project.
- `HETZNER_ROBOT_USER`: The User you have defined in Robot under settings/web.
- `HETZNER_ROBOT_PASSWORD`: The Robot Password you have set in Robot under settings/web.
- `SSH_KEY_NAME`: The name of the SSH key you want to use.
- `HETZNER_SSH_PUB_PATH`: The Path to your generated Public SSH Key.
- `HETZNER_SSH_PRIV_PATH`: The Path to your generated Private SSH Key. This is needed because CAPH uses this key to provision the node in Hetzner Dedicated.

```shell
kubectl create secret generic hetzner --from-literal=hcloud=$HCLOUD_TOKEN --from-literal=robot-user=$HETZNER_ROBOT_USER --from-literal=robot-password=$HETZNER_ROBOT_PASSWORD

kubectl create secret generic robot-ssh --from-literal=sshkey-name=$SSH_KEY_NAME \
        --from-file=ssh-privatekey=$HETZNER_SSH_PRIV_PATH \
        --from-file=ssh-publickey=$HETZNER_SSH_PUB_PATH
```

{% callout %}

`sshkey-name` (from SSH_KEY_NAME) should must match the name that is present in Hetzner otherwise the controller will not know how to reach the machine. You can upload ssh-keys via the Robot UI (Server / Key Management).

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
