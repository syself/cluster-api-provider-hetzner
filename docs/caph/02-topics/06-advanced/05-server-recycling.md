---
title: Server recycling
metatitle: Reusing existing Hetzner Cloud servers instead of creating new ones
sidebar: Server recycling
description: Back a MachineDeployment with a fixed set of pre-existing servers that are rebuilt and reused across scale events instead of being created and destroyed.
---

Server recycling lets an `HCloudMachine` reuse a pre-existing Hetzner Cloud server instead of creating
a new one. The controller looks for a server that is labelled as recyclable and matches the
`HCloudMachine`'s server type and location, claims it, and rebuilds it with the `HCloudMachine`'s
bootstrap data. On deletion the server is returned to the recyclable set instead of being deleted.

> Throughout this page, *labels* means **Hetzner Cloud** server labels, managed through the hcloud API
> or CLI (for example `hcloud server add-label`). They are not Kubernetes labels.

This is useful when a fixed set of servers should back a `MachineDeployment`, for example servers on a
reserved or otherwise favourable plan. Scaling up and down then reuses those servers instead of
creating and destroying a new one every time. A recycled server is a fully managed node like any
other, so it also takes part in machine health checks and remediation.

## Enabling recycling

Recycling is configured per `HCloudMachine`, usually through the `HCloudMachineTemplate` of a
`MachineDeployment`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: HCloudMachineTemplate
metadata:
  name: worker
spec:
  template:
    spec:
      type: cpx31
      imageName: my-snapshot
      recycle:
        enabled: true
```

Recycling requires `imageName`. The admission webhook rejects it together with `customProvisioner`
(`imageURL` and `imageURLCommand` in v1beta1). A custom provisioner installs the OS through the rescue
system, while the recycle path uses the hcloud rebuild API. Supporting it would mean re-running the
rescue flow on a claimed server, which is possible but out of scope for now.

## Marking a server as recyclable

A server joins the recyclable set by carrying two labels:

| Label | Value | Meaning |
| --- | --- | --- |
| `caph-recycle` | `true` | The server takes part in recycling and is never deleted by the controller. |
| `caph-recycle-available` | `true` | The server is unclaimed and can be picked up. |

While a claim is in progress the controller adds two more labels itself. They are transient and are
dropped once the claim completes. You never set them by hand.

| Label | Value | Meaning |
| --- | --- | --- |
| `caph-recycle-claimant` | `HCloudMachine` name | The `HCloudMachine` currently claiming the server. |
| `caph-recycle-claimed-at` | Unix seconds | When the claim was written, so an abandoned claim can be aged out. |

To hand an existing server to CAPH, set the first two labels on it, for example with
`hcloud server add-label`. Make sure its server type and location match the `HCloudMachine`s that
should claim it. Any server works this way, including one that was created by hand, and it does not
have to be rebuilt up front.

## How a claim works

When an `HCloudMachine` with recycling enabled is created, the controller:

1. Lists the servers carrying `caph-recycle=true`.
2. Keeps those that are available and match the server type and location, and picks one at random.
3. **Claims** it by dropping `caph-recycle-available` and adding `caph-recycle-claimant` with its own
   name, plus `caph-recycle-claimed-at`. It then requeues.
4. On the **next reconcile**, reads the labels again. If another `HCloudMachine`'s write landed in
   between, that one is the claimant now and this one starts over.
5. **Rebuilds** the server with the image and the bootstrap data. Ownership is checked once more
   directly before the rebuild. If the template names a placement group the server is not in yet, the
   server is added to that group first.
6. **Finalizes** the claim: the server gets the `HCloudMachine`'s name and identity label, and the
   claimant and timestamp labels are dropped.

The identity label is applied only in the last step, after a successful rebuild. That keeps the claim
idempotent. If the rebuild fails, the claim is released and the server returns to the pool, and a
claimed but not yet rebuilt server is never mistaken for a provisioned node. From the last step on the
server behaves like a freshly created one.

If no matching server is available, the controller creates a new one as usual. Recycling is an
optimization, not a requirement.

### A claim is not a lock

Hetzner replaces a server's whole label set on update and has no atomic label write, so a claim is
optimistic: the last write wins. Splitting the write and the check across two reconciles gives a
competing write time to land and be seen, and picking the candidate at random keeps concurrent claims
apart. Neither of the two makes a claim exclusive.

Safety does not depend on it. Ownership is re-checked directly before the rebuild, which is the only
destructive step, and an `HCloudMachine` that lost its claim does not touch labels it no longer owns.
Losing a race costs a reconcile, not a server.

### Losing a claimed server

The controller finds its server by ProviderID, and that says nothing about who owns it. The identity
label is therefore checked again on every reconcile that touches a live server. What happens next
depends on how far the claim had come:

- **Before the rebuild**, at the check in step 4: the `HCloudMachine` is not the claimant, so it starts
  over and picks another candidate.
- **Directly before the rebuild**, at the last ownership check in step 5: the `HCloudMachine` creates a
  new server instead.
- **After provisioning**: the server belongs to someone else now. The `HCloudMachine` annotates its
  CAPI `Machine` with `cluster.x-k8s.io/remediate-machine` and stops reconciling. It does not
  provision hardware that is not its own, and it does not return that server to the pool on deletion.

Deleting and replacing a `Machine` is CAPI's job, not the provider's, and only a `MachineHealthCheck`
acts on that annotation. Without one the `HCloudMachine` stays behind, annotated and unprovisioned,
and has to be deleted by hand. Attach a `MachineHealthCheck` to any `MachineDeployment` that uses
recycling.

### Abandoned claims

A claim is written and checked within seconds. If the controller dies in between, the claim would
otherwise keep the server out of the pool for good, because a claimed server carries no
`caph-recycle-available` label and matches no lookup. Any `HCloudMachine` that finds a claim older
than 15 minutes returns that server to the pool.

## Returning a server to the pool

On deletion a server carrying `caph-recycle=true` is not deleted. The controller shuts it down,
detaches it from the cluster's private network, and returns it to the pool: the
`HCloudMachine`-owned labels are removed and `caph-recycle-available=true` is set again.

The shutdown happens first, while the server still carries the `HCloudMachine`'s identity, so a failed
shutdown can be retried and the server never rejoins the pool while it still runs the deleted node.
The network has to be detached explicitly. A normal delete relies on the server deletion to detach it,
so a server that is kept would stay attached and later block deletion of the network.

This is decided from the server's own label, not from the `HCloudMachine`'s configuration. A
recyclable server is therefore never destroyed, even if recycling has been disabled since.

## Limitations

- Recycling requires `imageName` and cannot be combined with `customProvisioner`.
- Matching is by server type, location and placement group. Beyond that the candidates are
  interchangeable.
- **Create-time server properties are not reproduced by a rebuild.** The public network configuration
  and the SSH keys of a claimed server are the ones the pool server already had; they are not applied
  from the template. Node bootstrap does not need injected SSH keys, because RKE2 joins through
  cloud-init, but you cannot rely on the template's SSH keys being present on a recycled node.
- **Placement groups need care for control planes.** A rebuild does not move a server between
  placement groups, and hcloud cannot take a server out of one. A candidate in a different group is
  therefore skipped, and one in no group is added to the requested group before the rebuild. A spread
  placement group is what keeps servers on separate physical hosts, so a control plane outside it can
  end up sharing a host with another one. If a pool is meant for control planes, either leave its
  servers without a placement group, or put them in the group the template names.
- A server has to be stopped before hcloud moves it into a placement group, and it does so
  asynchronously. A server that was just returned to the pool is usually still shutting down. The
  claim asks it to stop and tries again on the next reconcile. A server whose OS ignores the shutdown
  request never gets there, and its claim keeps retrying.
- **A broken server returns to the pool, it is not quarantined.** When a `MachineHealthCheck`
  remediates a recycled node, the `Machine` is deleted and the server goes back to the recyclable set,
  where it can be claimed again. For a transient fault that is the intended self-healing. A server
  with a hard fault can loop instead: claimed, fails its health check, remediated, returned, claimed
  again. Remove the `caph-recycle` label from it to take it out of the pool.
- The size of the recyclable set is not managed by CAPH. If more `HCloudMachine`s are created than
  there are available servers, the surplus ones create new servers.
- A returned server keeps the deleted `HCloudMachine`'s name until it is claimed again. Only its
  labels are reset, so parked servers show a stale name in the Hetzner console.
- **A claim costs six hcloud API calls**, and more when the template asks for extras: two pool
  listings, the claim write, the ownership check before the rebuild, the rebuild and the finalize.
  Attaching a private network adds two, a placement group two more. Hetzner rate-limits per project,
  so a large fleet reconciling at once uses a noticeable share of that budget. A unit test pins the
  count so that it cannot grow unnoticed.
- Cleanup after a failed claim is best-effort. If both the rebuild and the release fail, for example
  during a longer API outage, a server can stay claimed. That is logged, and the claim is reclaimed
  once it ages out. Such a server is never mistaken for a provisioned node, so it costs capacity, not
  correctness.
