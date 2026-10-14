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

[Here](/docs/caph/02-topics/01-managing-ssh-keys.md) you can find more information regarding the handling of SSH keys. Some of them are specified in `HetznerCluster` to have them cluster-wide, others are machine-scoped.

## Usage without HCloud Load Balancer

It is also possible not to use the cloud load balancer from Hetzner. This is useful for setups with only one control plane, or if you have your own cloud load balancer.

Using `controlPlaneLoadBalancer.enabled=false` prevents the creation of a hcloud load balancer. Then you need to configure `controlPlaneEndpoint.port=6443` & `controlPlaneEndpoint.host`, which should be a domain that has A records configured pointing to the control plane IP for example.

If you are using your own load balancer, you need to point towards it and configure the load balancer to target the control planes of the cluster.

## HTTP(S) health checks for the control plane load balancer

By default the Hetzner load balancer checks the kube-apiserver service with a plain TCP check: it
only verifies that the port accepts connections, not that the apiserver is ready to serve requests.
Setting `controlPlaneLoadBalancer.healthCheck.protocol` to `http` or `https` makes the load balancer
request a path (for example `/readyz`) instead, so a control plane that is not ready is taken out of
rotation instead of receiving traffic. The field names and their limits mirror the Hetzner Cloud
API's load balancer
[`health_check` object](https://docs.hetzner.cloud/reference/cloud#tag/load-balancer-actions/add_load_balancer_service).

The endpoint the check requests must answer without authentication, because the load balancer cannot
present credentials. CAPH does not configure that for you. Use `healthCheck.port` when that endpoint
is served on a different port than the API server itself.

CAPH sends only the fields you set. A field you never set keeps the default Hetzner applies for it,
see the linked API reference above for the current defaults. Leaving `healthCheck` out means CAPH
does not manage the health check, so a load balancer you point CAPH at keeps the check it already
has. That also means deleting `healthCheck` after an http check was applied does not undo it. To get
a tcp check back, keep the field and set `healthCheck.protocol: tcp`. `path`, `domain`, `response`
and `statusCodes` are only valid when `protocol` is `http` or `https`.

### Safe migration on an existing cluster

If the load balancer switched to the http check right away, every control plane still running an
older image that does not answer the path would be marked unhealthy at once, taking the API server
offline. So CAPH waits, the same way it does for `enableProxyProtocol`: it switches the check only
once every control plane infra machine carries the annotation
`capi.syself.com/http-health-check-for-controlplane-loadbalancer: "true"`, set on the control plane
infra machine template. Until then the tcp check stays active and CAPH requeues.

This applies to every switch from tcp to http or https, not only the first one. A new cluster whose
spec already sets an http or https check is created with that check from the start.

The wait only covers the switch away from tcp. Setting `protocol` back to `tcp` is applied on the
next reconcile, and so is a change that stays within http or https, for example a new `path`, `port`
or `statusCodes`.

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
Defines the CIDR block of the subnet. Note that one subnet is required.
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

<PropField name="controlPlaneLoadBalancer.enableProxyProtocol" type="bool" defaultValue="false" required={false}>
Enables proxy protocol on the kube-apiserver service. Cannot be disabled once enabled.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck" type="object" required={false}>

Health check for the kube-apiserver service. Left out, Hetzner's default (tcp, 15s, 10s, 3 retries) applies and CAPH never touches it. See [above](#https-health-checks-for-the-control-plane-load-balancer).

<Collapsible title="properties">

<PropField name="controlPlaneLoadBalancer.healthCheck.protocol" type="string" defaultValue="tcp" required={false}>
Health check protocol. One of tcp, http, https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.port" type="int" defaultValue="service port" required={false}>
Port the check runs against. Must be in range 1-65535.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.intervalSeconds" type="int" required={false}>
Seconds between two checks. Must be in range 3-60.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.timeoutSeconds" type="int" required={false}>
Seconds to wait for a response. Must be in range 1-60.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.retries" type="int" required={false}>
Consecutive failed checks before a target counts as unhealthy, and successful checks to become healthy again. Must be in range 1-5.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.path" type="string" required={false}>
Request path, for example `/readyz`. Only valid when protocol is http or https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.domain" type="string" required={false}>
Host header sent with the request. Only valid when protocol is http or https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.response" type="string" required={false}>
String that must appear in the response. Only valid when protocol is http or https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.statusCodes" type="[]string" required={false}>
Status codes counted as healthy, for example `["200"]` or `["2??"]`. Only valid when protocol is http or https.
</PropField>

</Collapsible>

</PropField>

<PropField name="controlPlaneLoadBalancer.targetAddressFamily" type="string" defaultValue="dualstack" required={false}>
Which addresses of a bare metal control plane server are attached as load balancer targets. One of `ipv4`, `ipv6`, `dualstack`. Has no effect on HCloud servers. See [Bare metal control planes and the load balancer](/docs/caph/02-topics/05-baremetal/05-load-balancer-targets.md).
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
