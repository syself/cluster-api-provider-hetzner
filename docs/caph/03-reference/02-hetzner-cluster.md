---
title: HetznerCluster
description: In HetznerCluster you can define everything related to the general components of the cluster as well as those properties, which are valid cluster-wide.
metatitle: HetznerCluster Object Reference
---

In HetznerCluster you can define everything related to the general components of the cluster as well as those properties, which are valid cluster-wide.

There are two different modes for the cluster. A pure HCloud cluster and a cluster that uses Hetzner dedicated (bare metal) servers, either as control planes or as workers.

The HCloud cluster works with Kubeadm and supports private networks.

In a cluster that includes bare metal servers there are no private networks, as this feature has not yet been integrated in cluster-api-provider-hetzner. Apart from SSH, the node image has to support cloud-init, which we use to provision the bare metal machines.

> [!NOTE]
> In clusters with bare metal servers, you need to use [this CCM](https://github.com/syself/hetzner-cloud-controller-manager), as the official one does not support bare metal.

[Here](/docs/caph/topics/managing-ssh-keys) you can find more information regarding the handling of SSH keys. Some of them are specified in `HetznerCluster` to have them cluster-wide, others are machine-scoped.

## Usage without HCloud Load Balancer

It is also possible not to use the cloud load balancer from Hetzner. This is useful for setups with only one control plane, or if you have your own cloud load balancer.

Using `controlPlaneLoadBalancer.enabled=false` prevents the creation of a hcloud load balancer. Then you need to configure `controlPlaneEndpoint.port=6443` & `controlPlaneEndpoint.host`, which should be a domain that has A records configured pointing to the control plane IP for example.

If you are using your own load balancer, you need to point towards it and configure the load balancer to target the control planes of the cluster.

## Overview of HetznerCluster.Spec

<PropField name="hcloudNetwork" type="object" required={false}>

Specifies details about Hetzner cloud private networks.

<Collapsible title="properties">

<PropField name="hcloudNetwork.enabled" type="bool" required={true}>
States whether network should be enabled or disabled.
</PropField>

<PropField name="hcloudNetwork.cidrBlock" type="string" defaultValue='"10.0.0.0/16"' required={false}>
Defines the CIDR block.
</PropField>

<PropField name="hcloudNetwork.subnetCidrBlock" type="string" defaultValue='"10.0.0.0/24"' required={false}>
Defines the CIDR block of the subnet. Note that one subnet ist required.
</PropField>

<PropField name="hcloudNetwork.networkZone" type="string" defaultValue='"eu-central"' required={false}>
Defines the network zone. Must be eu-central, us-east or us-west.
</PropField>

</Collapsible>

</PropField>

<PropField name="controlPlaneRegions" type="[]string" defaultValue={"[]string{fsn1}"} required={false}>
This is the base for the failureDomains of the cluster.
</PropField>

<PropField name="sshKeys" type="object" required={false}>

Cluster-wide SSH keys that serve as default for machines as well.

<Collapsible title="properties">

<PropField name="sshKeys.hcloud" type="[]object" required={false}>

SSH keys for hcloud.

<Collapsible title="properties">

<PropField name="sshKeys.hcloud[].name" type="string" required={true}>
Name of SSH key.
</PropField>

<PropField name="sshKeys.hcloud[].fingerprint" type="string" required={false}>
Fingerprint of SSH key - used by the controller.
</PropField>

</Collapsible>

</PropField>

<PropField name="sshKeys.robotRescueSecretRef" type="object" required={false}>

Reference to the secret where the SSH key for the rescue system is stored.

<Collapsible title="properties">

<PropField name="sshKeys.robotRescueSecretRef.name" type="string" required={true}>
Name of the secret.
</PropField>

<PropField name="sshKeys.robotRescueSecretRef.key" type="object" required={true}>

Details about the keys used in the data of the secret.

<Collapsible title="properties">

<PropField name="sshKeys.robotRescueSecretRef.key.name" type="string" required={true}>
Name is the key in the secret's data where the SSH key's name is stored.
</PropField>

<PropField name="sshKeys.robotRescueSecretRef.key.publicKey" type="string" required={true}>
PublicKey is the key in the secret's data where the SSH key's public key is stored.
</PropField>

<PropField name="sshKeys.robotRescueSecretRef.key.privateKey" type="string" required={true}>
PrivateKey is the key in the secret's data where the SSH key's private key is stored.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="controlPlaneEndpoint" type="object" required={false}>

Set by the controller. It is the endpoint to communicate with the control plane.

<Collapsible title="properties">

<PropField name="controlPlaneEndpoint.host" type="string" required={true}>
Defines host.
</PropField>

<PropField name="controlPlaneEndpoint.port" type="int32" required={true}>
Defines port.
</PropField>

</Collapsible>

</PropField>

<PropField name="controlPlaneLoadBalancer" type="object" required={true}>

Defines specs of load balancer.

<Collapsible title="properties">

<PropField name="controlPlaneLoadBalancer.enabled" type="bool" defaultValue="true" required={false}>
Specifies if a load balancer should be created.
</PropField>

<PropField name="controlPlaneLoadBalancer.name" type="string" required={false}>
Name of load balancer.
</PropField>

<PropField name="controlPlaneLoadBalancer.algorithm" type="string" defaultValue="round_robin" required={false}>
Type of load balancer algorithm. Either round_robin or least_connections.
</PropField>

<PropField name="controlPlaneLoadBalancer.type" type="string" defaultValue="lb11" required={false}>
Type of load balancer. One of lb11, lb21, lb31.
</PropField>

<PropField name="controlPlaneLoadBalancer.port" type="int" defaultValue="6443" required={false}>
Load balancer port. Must be in range 1-65535.
</PropField>

<PropField name="controlPlaneLoadBalancer.extraServices" type="[]object" required={false}>

Defines extra services of load balancer.

<Collapsible title="properties">

<PropField name="controlPlaneLoadBalancer.extraServices[].protocol" type="string" required={true}>
Defines protocol. Must be one of https, http, or tcp.
</PropField>

<PropField name="controlPlaneLoadBalancer.extraServices[].listenPort" type="int" required={true}>
Defines listen port. Must be in range 1-65535.
</PropField>

<PropField name="controlPlaneLoadBalancer.extraServices[].destinationPort" type="int" required={true}>
Defines destination port. Must be in range 1-65535.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="hcloudPlacementGroups" type="[]object" required={false}>

List of placement groups that should be defined in Hetzner API.

<Collapsible title="properties">

<PropField name="hcloudPlacementGroups[].name" type="string" required={true}>
Name of placement group.
</PropField>

<PropField name="hcloudPlacementGroups[].type" type="string" defaultValue="type" required={false}>
Type of placement group. Hetzner only supports 'spread'.
</PropField>

</Collapsible>

</PropField>

<PropField name="hetznerSecret" type="object" required={true}>

Reference to secret where Hetzner API credentials are stored.

<Collapsible title="properties">

<PropField name="hetznerSecret.name" type="string" required={true}>
Name of secret.
</PropField>

<PropField name="hetznerSecret.key" type="object" required={true}>

Reference to the keys that are used in the secret, either `hcloudToken` or `hetznerRobotUser` and `hetznerRobotPassword` need to be specified.

<Collapsible title="properties">

<PropField name="hetznerSecret.key.hcloudToken" type="string" required={false}>
Name of the key where the token for the Hetzner Cloud API is stored.
</PropField>

<PropField name="hetznerSecret.key.hetznerRobotUser" type="string" required={false}>
Name of the key where the username for the Hetzner Robot API is stored.
</PropField>

<PropField name="hetznerSecret.key.hetznerRobotPassword" type="string" required={false}>
Name of the key where the password for the Hetzner Robot API is stored.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="skipCreatingHetznerSecretInWorkloadCluster" type="bool" defaultValue="false" required={false}>
Indicates whether the Hetzner secret should be created in the workload cluster. By default the secret gets created, so that the ccm (running in the wl-cluster) can use that secret. If you prefer to not reveal the secret in the wl-cluster, you can set this to value to false, so that the secret is not created. Be sure to run the ccm outside of the wl-cluster in that case, e.g. in the management cluster.
</PropField>
