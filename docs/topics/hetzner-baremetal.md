#### Preparing Hetzner Robot

1. Create a new web service user. [Here](https://robot.your-server.de/preferences/index), you can define a password and copy your user name.
1. Generate an SSH key. You can either upload it via Hetzner Robot UI or just rely on the controller to upload a key that it does not find in the robot API. This is possible, as you have to store the public and private key together with the SSH key's name in a secret that the controller reads.

## Hetzner Dedicated / Bare Metal Server

If you want to create a cluster with bare metal servers, you will also need to set up the robot credentials in the preparation step. As described in the [reference](/docs/reference/hetzner-bare-metal-machine-template.md), you need to buy bare metal servers beforehand manually. To use bare metal servers for your deployment, you should choose one of the following flavors:

| Flavor                                       | What it does                                                                                                                                 |
| -------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| hetzner-baremetal-control-planes-remediation | Uses bare metal servers for the control plane nodes - with custom remediation (try to reboot machines first)  |
| hetzner-baremetal-control-planes             | Uses bare metal servers for the control plane nodes - with normal remediation (unprovision/recreate machines) |
| hetzner-hcloud-control-planes                | Uses the hcloud servers for the control plane nodes and the bare metal servers for the worker nodes                                          |

Next, you need to create a `HetznerBareMetalHost` object for each bare metal server that you bought and specify its server ID in the specs. Refer to an example [here](/docs/reference/hetzner-bare-metal-host.md). Add the created objects to your `my-cluster.yaml` file. If you already know the WWN of the storage device you want to choose for booting, specify it in the `rootDeviceHints` of the object. If not, you can apply the workload cluster, start the provisioning without specifying the WWN, and then wait for the bare metal hosts to show an error.

After that, look at the status of `HetznerBareMetalHost` by running `kubectl describe hetznerbaremetalhost` in your management cluster. There you will find `hardwareDetails` of all of your bare metal hosts, in which you can see a list of all the relevant storage devices as well as their properties. You can copy+paste the WWN:s of your desired storage device into the `rootDeviceHints` of your `HetznerBareMetalHost` objects.

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
```

Patch the created secrets so that they get automatically moved to the target cluster later. The following command helps you do that:

```shell
kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
kubectl patch secret robot-ssh -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'
```

The secret name and the tokens can also be customized in the cluster template.
