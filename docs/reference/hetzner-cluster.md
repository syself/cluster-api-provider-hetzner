## HetznerCluster

In HetznerCluster you can define everything related to the general components of the cluster as well as those properties, which are valid cluster-wide.

There are two different modes for the cluster. A pure HCloud cluster and a cluster that uses Hetzner dedicated (bare metal) servers, either as control planes or as workers. The HCloud cluster works with Kubeadm and supports private networks. In a cluster that includes bare metal servers there are no private networks, as this feature has not yet been integrated in cluster-api-provider-hetzner. Apart from SSH, the node image has to support cloud-init, which we use to provision the bare metal machines. In cluster with bare metal servers, you need to use [this CCM](https://github.com/syself/hetzner-cloud-controller-manager), as the official one does not support bare metal.

[Here](/docs/topics/managing-ssh-keys.md) you can find more information regarding the handling of SSH keys. Some of them are specified in ```HetznerCluster``` to have them cluster-wide, others are machine-scoped.

### Usage without HCloud Load Balancer
It is also possible not to use the cloud load balancer from Hetzner. This is useful for setups with only one control plane, or if you have your own cloud load balancer. Using `controlPlaneLoadBalancer.enabled=false` prevents the creation of a hcloud load balancer. Then you need to configure `controlPlaneEndpoint.port=6443` & `controlPlaneEndpoint.host`, which should be a domain that has A records configured pointing to the control plane IP for example. If you are using your own load balancer, you need to point towards it and configure the load balancer to target the control planes of the cluster. 

## Overview of HetznerCluster.Spec
| Key | Type | Default | Required | Description |
|-----|-----|------|---------|-------------|
| hcloudNetwork | object |  | no | Specifies details about Hetzner cloud private networks |
| hcloudNetwork.enabled | bool |  | yes| States whether network should be enabled or disabled |
| hcloudNetwork.cidrBlock | string | "10.0.0.0/16" | no | Defines the CIDR block |
| hcloudNetwork.subnetCidrBlock | string | "10.0.0.0/24" | no | Defines the CIDR block of the subnet. Note that one subnet ist required |
| hcloudNetwork.networkZone | string | "eu-central" | no | Defines the network zone. Must be eu-central, us-east or us-west |
| controlPlaneRegions | []string | []string{fsn1} | no | This is the base for the failureDomains of the cluster |
| sshKeys | object | | no | Cluster-wide SSH keys that serve as default for machines as well |
| sshKeys.hcloud | []object | | no | SSH keys for hcloud |
| sshKeys.hcloud.name | string | | yes | Name of SSH key |
| sshKeys.hcloud.fingerprint | string | | no| Fingerprint of SSH key - used by the controller |
| sshKeys.robotRescueSecretRef | object | | no | Reference to the secret where the SSH key for the rescue system is stored |
| sshKeys.robotRescueSecretRef.name | string | | yes | Name of the secret |
| sshKeys.robotRescueSecretRef.key | object | | yes | Details about the keys used in the data of the secret |
| sshKeys.robotRescueSecretRef.key.name | string | | yes | Name is the key in the secret's data where the SSH key's name is stored |
| sshKeys.robotRescueSecretRef.key.publicKey | string | | yes | PublicKey is the key in the secret's data where the SSH key's public key is stored |
| sshKeys.robotRescueSecretRef.key.privateKey | string | | yes | PrivateKey is the key in the secret's data where the SSH key's private key is stored |
| controlPlaneEndpoint | object | | no | Set by the controller. It is the endpoint to communicate with the control plane |
| controlPlaneEndpoint.host | string | | yes | Defines host |
| controlPlaneEndpoint.port | int32 | | yes | Defines port |
|controlPlaneLoadBalancer | object | | yes | Defines specs of load balancer |
|controlPlaneLoadBalancer.enabled | bool | true | no | Specifies if a load balancer should be created |
|controlPlaneLoadBalancer.name | string | | no | Name of load balancer |
 |controlPlaneLoadBalancer.algorithm | string | round_robin | no | Type of load balancer algorithm. Either round_robin or least_connections |
|controlPlaneLoadBalancer.type | string | lb11 | no | Type of load balancer. One of lb11, lb21, lb31 |
|controlPlaneLoadBalancer.port| int | 6443 | no | Load balancer port. Must be in range 1-65535 |
|controlPlaneLoadBalancer.extraServices| []object | | no | Defines extra services of load balancer |
|controlPlaneLoadBalancer.extraServices.protocol | string | | yes | Defines protocol. Must be one of https, http, or tcp |
|controlPlaneLoadBalancer.extraServices.listenPort | int | | yes | Defines listen port. Must be in range 1-65535 |
|controlPlaneLoadBalancer.extraServices.destinationPort | int | | yes | Defines destination port. Must be in range 1-65535 |
|hcloudPlacementGroup | []object | | no | List of placement groups that should be defined in Hetzner API | 
|hcloudPlacementGroup.name | string | | yes | Name of placement group | 
|hcloudPlacementGroup.type | string | type | no | Type of placement group. Hetzner only supports 'spread' | 
| hetznerSecret | object |  | yes | Reference to secret where Hetzner API credentials are stored |
| hetznerSecret.name | string |  | yes | Name of secret |
| hetznerSecret.key | object |  | yes | Reference to the keys that are used in the secret, either `hcloudToken` or `hetznerRobotUser` and `hetznerRobotPassword` need to be specified |
| hetznerSecret.key.hcloudToken | string |  | no | Name of the key where the token for the Hetzner Cloud API is stored |
| hetznerSecret.key.hetznerRobotUser | string |  | no | Name of the key where the username for the Hetzner Robot API is stored |
| hetznerSecret.key.hetznerRobotPassword | string |  | no | Name of the key where the password for the Hetzner Robot API is stored |

