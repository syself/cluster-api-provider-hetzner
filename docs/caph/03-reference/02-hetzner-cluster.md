---
title: HetznerCluster
metatitle: HetznerCluster Object Reference
description: In HetznerCluster you can define everything related to the general components of the cluster as well as those properties, which are valid cluster-wide.
---

In HetznerCluster you can define everything related to the general components of the cluster as well as those properties, which are valid cluster-wide.

There are two different modes for the cluster. A pure HCloud cluster and a cluster that uses Hetzner dedicated (bare metal) servers, either as control planes or as workers.

The HCloud cluster works with Kubeadm and supports private networks.

In a cluster that includes bare metal servers there are no private networks, as this feature has not yet been integrated in cluster-api-provider-hetzner. Apart from SSH, the node image has to support cloud-init, which we use to provision the bare metal machines.

For bare-metal clusters, you can use either the Syself CCM or the upstream HCloud CCM. For setup
details and ProviderID format requirements, see
[Creating Workload Cluster](/docs/caph/02-topics/05-baremetal/03-creating-workload-cluster.md#deploying-the-hetzner-cloud-controller-manager).

[Here](/docs/caph/02-topics/01-managing-ssh-keys.md) you can find more information regarding the handling of SSH keys. Some of them are specified in `HetznerCluster` to have them cluster-wide, others are machine-scoped.

## Usage without HCloud Load Balancer

It is also possible not to use the cloud load balancer from Hetzner. This is useful for setups with only one control plane, or if you have your own cloud load balancer.

Using `controlPlaneLoadBalancer.enabled=false` prevents the creation of a hcloud load balancer. Then you need to configure `controlPlaneEndpoint.port=6443` & `controlPlaneEndpoint.host`, which should be a domain that has A records configured pointing to the control plane IP for example.

If you are using your own load balancer, you need to point towards it (by setting
`controlPlaneLoadBalancer.name`) and configure the load balancer to target the control planes of the
cluster.

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

CAPH sends only the fields you set. A field you never set keeps Hetzner's own default for it (tcp,
15s interval, 10s timeout, 3 retries). If `healthCheck` is left out entirely, CAPH never touches the
load balancer's health check at all. Removing `healthCheck` again does not bring the TCP check back:
CAPH has nothing to send in its place, so the load balancer keeps the last check that was applied.
`path`, `domain`, `response` and `statusCodes` are only valid when `protocol` is `http` or `https`.

### Safe migration on an existing cluster

Switching an existing cluster from tcp to http or https is a one-way migration. If the load balancer
switched right away, every control plane still running an older image that does not answer the path
would be marked unhealthy at once, taking the API server offline. CAPH gates the switch the same way
it gates `enableProxyProtocol`: it waits until every control plane machine carries the annotation
`capi.syself.com/http-health-check-for-controlplane-loadbalancer: "true"`, set on the control plane
machine template, before switching the check in place. Until then the tcp check stays active and
CAPH requeues.

The gate only covers the switch away from tcp. A new cluster whose spec already sets an http or https
check is created with that check from the start, and a change that stays within http or https (for
example editing `path` or `intervalSeconds`) is applied right away.

## Overview of HetznerCluster.Spec

| Key                                                      | Type       | Default          | Required | Description                                                                                                                                   |
| -------------------------------------------------------- | ---------- | ---------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `hcloudNetwork`                                          | `object`   |                  | no       | Specifies details about Hetzner cloud private networks                                                                                        |
| `hcloudNetwork.enabled`                                  | `bool`     |                  | yes      | States whether network should be enabled or disabled                                                                                          |
| `hcloudNetwork.cidrBlock`                                | `string`   | `"10.0.0.0/16"`  | no       | Defines the CIDR block                                                                                                                        |
| `hcloudNetwork.subnetCidrBlock`                          | `string`   | `"10.0.0.0/24"`  | no       | Defines the CIDR block of the subnet. Note that one subnet ist required                                                                       |
| `hcloudNetwork.networkZone`                              | `string`   | `"eu-central"`   | no       | Defines the network zone. Must be eu-central, us-east or us-west                                                                              |
| `controlPlaneRegions`                                    | `[]string` | `[]string{fsn1}` | no       | This is the base for the failureDomains of the cluster                                                                                        |
| `sshKeys`                                                | `object`   |                  | no       | Cluster-wide SSH keys that serve as default for machines as well                                                                              |
| `sshKeys.hcloud`                                         | `[]object` |                  | no       | SSH keys for hcloud                                                                                                                           |
| `sshKeys.hcloud[].name`                                    | `string`   |                  | yes      | Name of SSH key                                                                                                                               |
| `sshKeys.hcloud[].fingerprint`                             | `string`   |                  | no       | Fingerprint of SSH key - used by the controller                                                                                               |
| `sshKeys.robotRescueSecretRef`                           | `object`   |                  | no       | Reference to the secret where the SSH key for the rescue system is stored                                                                     |
| `sshKeys.robotRescueSecretRef.name`                      | `string`   |                  | yes      | Name of the secret                                                                                                                            |
| `sshKeys.robotRescueSecretRef.key`                       | `object`   |                  | yes      | Details about the keys used in the data of the secret                                                                                         |
| `sshKeys.robotRescueSecretRef.key.name`                  | `string`   |                  | yes      | Name is the key in the secret's data where the SSH key's name is stored                                                                       |
| `sshKeys.robotRescueSecretRef.key.publicKey`             | `string`   |                  | yes      | PublicKey is the key in the secret's data where the SSH key's public key is stored                                                            |
| `sshKeys.robotRescueSecretRef.key.privateKey`            | `string`   |                  | yes      | PrivateKey is the key in the secret's data where the SSH key's private key is stored                                                          |
| `controlPlaneEndpoint`                                   | `object`   |                  | no       | Set by the controller. It is the endpoint to communicate with the control plane                                                               |
| `controlPlaneEndpoint.host`                              | `string`   |                  | yes      | Defines host                                                                                                                                  |
| `controlPlaneEndpoint.port`                              | `int`32    |                  | yes      | Defines port                                                                                                                                  |
| `controlPlaneLoadBalancer`                               | `object`   |                  | yes      | Defines specs of load balancer                                                                                                                |
| `controlPlaneLoadBalancer.enabled`                       | `bool`     | `true`           | no       | Specifies if a load balancer should be created                                                                                                |
| `controlPlaneLoadBalancer.name`                          | `string`   |                  | no       | Name of load balancer                                                                                                                         |
| `controlPlaneLoadBalancer.algorithm`                     | `string`   | `round_robin`    | no       | Type of load balancer algorithm. Either round_robin or least_connections                                                                      |
| `controlPlaneLoadBalancer.type`                          | `string`   | `lb11`           | no       | Type of load balancer. One of lb11, lb21, lb31                                                                                                |
| `controlPlaneLoadBalancer.port`                          | `int`      | `6443`           | no       | Load balancer port. Must be in range 1-65535                                                                                                  |
| `controlPlaneLoadBalancer.extraServices`                 | `[]object` |                  | no       | Defines extra services of load balancer                                                                                                       |
| `controlPlaneLoadBalancer.extraServices[].protocol`        | `string`   |                  | yes      | Defines protocol. Must be one of https, http, or tcp                                                                                          |
| `controlPlaneLoadBalancer.extraServices[].listenPort`      | `int`      |                  | yes      | Defines listen port. Must be in range 1-65535                                                                                                 |
| `controlPlaneLoadBalancer.extraServices[].destinationPort` | `int`      |                  | yes      | Defines destination port. Must be in range 1-65535                                                                                            |
| `controlPlaneLoadBalancer.enableProxyProtocol`           | `bool`     | `false`          | no       | Enables proxy protocol on the kube-apiserver service. Cannot be disabled once enabled                                                          |
| `controlPlaneLoadBalancer.healthCheck`                   | `object`   |                  | no       | Health check for the kube-apiserver service. Left out, Hetzner's default (tcp, 15s, 10s, 3 retries) applies and CAPH never touches it. See [above](#https-health-checks-for-the-control-plane-load-balancer) |
| `controlPlaneLoadBalancer.healthCheck.protocol`          | `string`   | `tcp`            | no       | Health check protocol. One of tcp, http, https                                                                                                |
| `controlPlaneLoadBalancer.healthCheck.port`              | `int`      | service port     | no       | Port the check runs against. Must be in range 1-65535                                                                                         |
| `controlPlaneLoadBalancer.healthCheck.intervalSeconds`   | `int`      |                  | no       | Seconds between two checks. Must be in range 3-60                                                                                             |
| `controlPlaneLoadBalancer.healthCheck.timeoutSeconds`    | `int`      |                  | no       | Seconds to wait for a response. Must be in range 1-60                                                                                         |
| `controlPlaneLoadBalancer.healthCheck.retries`           | `int`      |                  | no       | Consecutive failed checks before a target counts as unhealthy, and successful checks to become healthy again. Must be in range 1-5            |
| `controlPlaneLoadBalancer.healthCheck.path`              | `string`   |                  | no       | Request path, for example `/readyz`. Only valid when protocol is http or https                                                                |
| `controlPlaneLoadBalancer.healthCheck.domain`            | `string`   |                  | no       | Host header sent with the request. Only valid when protocol is http or https                                                                  |
| `controlPlaneLoadBalancer.healthCheck.response`          | `string`   |                  | no       | String that must appear in the response. Only valid when protocol is http or https                                                            |
| `controlPlaneLoadBalancer.healthCheck.statusCodes`       | `[]string` |                  | no       | Status codes counted as healthy, for example `["200"]` or `["2??"]`. Only valid when protocol is http or https                                 |
| `hcloudPlacementGroups`                                   | `[]object` |                  | no       | List of placement groups that should be defined in Hetzner API                                                                                |
| `hcloudPlacementGroups[].name`                              | `string`   |                  | yes      | Name of placement group                                                                                                                       |
| `hcloudPlacementGroups[].type`                              | `string`   | `type`           | no       | Type of placement group. Hetzner only supports 'spread'                                                                                       |
| `hetznerSecret`                                          | `object`   |                  | yes      | Reference to secret where Hetzner API credentials are stored                                                                                  |
| `hetznerSecret.name`                                     | `string`   |                  | yes      | Name of secret                                                                                                                                |
| `hetznerSecret.key`                                      | `object`   |                  | yes      | Reference to the keys that are used in the secret, either `hcloudToken` or `hetznerRobotUser` and `hetznerRobotPassword` need to be specified |
| `hetznerSecret.key.hcloudToken`                          | `string`   |                  | no       | Name of the key where the token for the Hetzner Cloud API is stored                                                                           |
| `hetznerSecret.key.hetznerRobotUser`                     | `string`   |                  | no       | Name of the key where the username for the Hetzner Robot API is stored                                                                        |
| `hetznerSecret.key.hetznerRobotPassword`                 | `string`   |                  | no       | Name of the key where the password for the Hetzner Robot API is stored                                                                        |
| `skipCreatingHetznerSecretInWorkloadCluster`             | `bool`   | `false`            | no       | Indicates whether the Hetzner secret should be created in the workload cluster. By default the secret gets created, so that the ccm (running in the wl-cluster) can use that secret. If you prefer to not reveal the secret in the wl-cluster, you can set this to value to false, so that the secret is not created. Be sure to run the ccm outside of the wl-cluster in that case, e.g. in the management cluster. |
