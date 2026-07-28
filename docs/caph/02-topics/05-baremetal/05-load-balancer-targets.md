---
title: Bare metal control planes and the load balancer
metatitle: Choose Which Addresses of a Bare Metal Control Plane Server Serve API Traffic
sidebar: Load balancer targets
description: Select the address family CAPH uses when it attaches a bare metal control plane server to the control plane load balancer.
---

## Why bare metal servers are different

CAPH attaches every control plane machine to the control plane load balancer, but it cannot do it the same way for both kinds of server.

An HCloud server is a resource of the HCloud API, the same API the load balancer belongs to, so CAPH attaches it by its server ID. This is a target of type `server`, and the load balancer resolves the address of that server on its own. There is nothing to choose.

A bare metal server is a Hetzner Robot resource. It has a server ID too, but not one the HCloud load balancer can reference, so CAPH attaches it by address instead. This is a target of type `ip`, and such a target holds exactly one address. Reaching one server over both protocols therefore takes two targets, and CAPH has to decide which of them to create.

`spec.controlPlaneLoadBalancer.targetAddressFamily` is that decision.

The field is limited to that decision, so two things it does not touch are worth naming. Worker machines are never attached to the control plane load balancer, bare metal or not, so the field does not apply to them. And a `Service` of type `LoadBalancer` inside the workload cluster gets its own load balancer from the cloud controller manager, not from CAPH, so the targets of that one are configured there instead.

## Choosing the address family

| Value       | Targets created for a bare metal control plane server |
| ----------- | ----------------------------------------------------- |
| `ipv4`      | The IPv4 address only.                                |
| `ipv6`      | The IPv6 address only.                                |
| `dualstack` | Both addresses, as two separate targets. The default. |

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerCluster
spec:
  controlPlaneLoadBalancer:
    targetAddressFamily: ipv4
```

The field is optional. Leaving it out is the same as setting `dualstack`.

An address the server does not have is never attached. If a server has no IPv6 address, `dualstack` attaches its IPv4 address and nothing else.

## When to set ipv4

Hetzner routes an IPv6 subnet to a bare metal server, so an IPv6 address for it exists in the Robot API, and CAPH records it as `spec.status.ipv6` on the `HetznerBareMetalHost`. That an address exists is not the same as the server answering on it: the installed OS still has to configure it, and an image that sets up IPv4 only is common. The Robot API does not report what the running OS configured, so CAPH cannot detect this and choose for you.

When the server does not answer on the address, the target is still created and simply never passes its health check. The load balancer then reports an unhealthy target for as long as the machine exists, which buries a genuinely unhealthy control plane in the noise, and one target slot is spent on a target that cannot serve traffic.

The default is `dualstack`, so both addresses are attached. Set `ipv4` on a cluster whose servers only use IPv4, so the load balancer does not carry an IPv6 target that never becomes healthy. Set `ipv6` for a single-stack IPv6 setup.

A quick way to check what a server actually configured, from a shell on the machine:

```shell
ip -6 addr show scope global
ip -6 route show default
```

If both are empty, the server has no usable IPv6 address and an IPv6 target for it cannot become healthy.

## Changing the value later

You can change the field at any time. On the next reconcile of each bare metal control plane machine, CAPH attaches the addresses of the newly selected family that are missing, and detaches the targets of the addresses the family no longer selects. No manual cleanup in the HCloud API is needed.

Detaching is not held back by the kube-apiserver health gate that governs attaching, because a target that is no longer selected cannot serve traffic and there is nothing to wait for.

Each change is visible in `status.controlPlaneLoadBalancer.targets` on the `HetznerCluster`, which lists the targets as they exist on the load balancer:

```shell
kubectl get hetznercluster <name> -o jsonpath='{.status.controlPlaneLoadBalancer.targets}'
```

CAPH also emits events on the `HetznerCluster` when it attaches or detaches an address.

## Upgrading an existing cluster

The default `dualstack` attaches both addresses, so an upgrade attaches nothing new and removes nothing: every bare metal control plane server keeps both of its targets.

To drop an IPv6 target that never becomes healthy, set `targetAddressFamily: ipv4` on the `HetznerCluster`, or on the `HetznerClusterTemplate` if you use `ClusterClass`. On the next reconcile the IPv6 target of every bare metal control plane server is detached. The IPv4 target of the same server stays attached, so the API server remains reachable through the load balancer.

This does not affect HCloud control plane machines, which are attached by server ID and are untouched by the field.
