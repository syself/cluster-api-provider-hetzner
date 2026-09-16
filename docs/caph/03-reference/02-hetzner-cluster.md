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
only verifies that the port accepts connections, not that the apiserver is actually ready to serve
requests. Setting `controlPlaneLoadBalancer.healthCheck.protocol` to `http` or `https` switches the
load balancer to request a path (e.g. `/readyz`) instead, so unhealthy control-plane nodes are
taken out of rotation instead of continuing to receive traffic. Field names mirror the Hetzner
Cloud API's load balancer
[`health_check`](https://docs.hetzner.cloud/reference/cloud#tag/load-balancer-actions/add_load_balancer_service)
object.

This is opt-in and requires the kube-apiserver to serve the configured path without
authentication, since the load balancer's health check request is unauthenticated. CAPH does not
configure this for you; you must allow anonymous access to the configured path yourself, e.g. via
kubeadm's default `system:public-info-viewer` RBAC binding or an
[AuthenticationConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-requests)
that scopes anonymous access to that path only. If this tradeoff isn't acceptable for your
environment, leave `healthCheck` unset to keep the plain TCP check.

CAPH only sends the fields you set; a field you never set keeps Hetzner's own default — see the
linked API reference above for the current defaults. `path`, `domain`, `response` and
`statusCodes` are only valid when `protocol` is `http` or `https`.
Use `healthCheck.port` when the health-check endpoint is served on a different port than the API
server itself; left unset, the check runs against the service's own destination port.

Leaving `healthCheck` out entirely means CAPH does not manage the health check at all, so a load
balancer you point CAPH at (or one that already has a check applied) keeps whatever check it has.
That also means deleting `healthCheck` after an http check was applied does not undo it — to get a
tcp check back, keep the field and set `healthCheck.protocol: tcp`.

### Safe migration on an existing cluster

If the load balancer switched to the http or https check immediately, every control-plane node
running an older image that doesn't yet answer the configured path would be marked unhealthy at
once, taking the API server offline. To avoid that, CAPH gates the switch the same way it gates
enabling `enableProxyProtocol`: it waits until every control-plane infra machine carries the
annotation `capi.syself.com/http-health-check-for-controlplane-loadbalancer: "true"` — set on the
control-plane infra machine template — before switching the load balancer's health check in place.
Until then the tcp check stays active and CAPH requeues.

This gate applies to every switch away from tcp, not only the first one. A new cluster whose spec
already sets an http or https check is created with that check from the start (no annotation
needed). The wait only covers the switch away from tcp: setting `protocol` back to `tcp`, and any
change that stays within http/https (e.g. a new `path`, `port` or `statusCodes`), is applied on the
next reconcile without waiting for the gate.

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

<PropField name="controlPlaneLoadBalancer.targetAddressFamily" type="string" defaultValue="dualstack" required={false}>
Which addresses of a bare metal control plane server are attached as load balancer targets. One of `ipv4`, `ipv6`, `dualstack`. Has no effect on HCloud servers. See [Bare metal control planes and the load balancer](/docs/caph/02-topics/05-baremetal/05-load-balancer-targets.md).
</PropField>

<PropField name="controlPlaneLoadBalancer.enableProxyProtocol" type="bool" defaultValue="false" required={false}>
Enables proxy protocol on the kube-apiserver load balancer service. Cannot be disabled once enabled.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck" type="object" required={false}>

Configures the health check for the kube-apiserver load balancer service. If omitted, Hetzner's own default (a plain tcp check) is unchanged. See [above](#https-health-checks-for-the-control-plane-load-balancer) for the linked API reference and current defaults.

<Collapsible title="properties">

<PropField name="controlPlaneLoadBalancer.healthCheck.protocol" type="string" defaultValue="tcp" required={false}>
Protocol used for the health check. One of tcp, http, https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.port" type="int" defaultValue="service port" required={false}>
Port the check runs against. If omitted, the service's destination port is used.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.intervalSeconds" type="int" required={false}>
Time in seconds between two consecutive health checks. If omitted, Hetzner's own default is used (see the linked API reference above).
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.timeoutSeconds" type="int" required={false}>
Time in seconds to wait for a health check attempt to succeed. If omitted, Hetzner's own default is used (see the linked API reference above).
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.retries" type="int" required={false}>
Number of consecutive failed health checks before a target is considered unhealthy. If omitted, Hetzner's own default is used (see the linked API reference above).
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.path" type="string" required={false}>
HTTP(S) path requested for the health check, e.g. `"/readyz"`. Only valid when protocol is http or https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.domain" type="string" required={false}>
Host header sent with the HTTP(S) health check request. Only valid when protocol is http or https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.response" type="string" required={false}>
String that must be contained in the HTTP(S) response for the check to pass. Only valid when protocol is http or https.
</PropField>

<PropField name="controlPlaneLoadBalancer.healthCheck.statusCodes" type="[]string" required={false}>
HTTP(S) response status codes counted as healthy, e.g. `["200"]` or `["2??"]`. If empty, Hetzner's own default is used (see the linked API reference above). Only valid when protocol is http or https.
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
