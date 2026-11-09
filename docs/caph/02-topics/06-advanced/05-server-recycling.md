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
      serverRecycling: Enabled
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

1. Lists the servers carrying `caph-recycle=true`. This covers the whole Hetzner project, not one
   cluster: if several clusters share a project, a server that one of them returned can be claimed by
   any of the others.
2. Keeps those that are available and match the server type and location, and picks one at random.
3. **Claims** it by dropping `caph-recycle-available` and adding `caph-recycle-claimant` with its own
   name, plus `caph-recycle-claimed-at`. It then requeues.
4. On the **next reconcile**, reads the labels again. If another `HCloudMachine`'s write landed in
   between, that one is the claimant now and this one starts over.
5. **Prepares** the server for the rebuild, if the template asks for anything the server does not have
   yet: the placement group it names, and the cluster's private network. hcloud applies both
   asynchronously and rejects a rebuild while an action is still running, so each step is confirmed on
   a later reconcile before the next one starts.
6. **Rebuilds** the server with the image and the bootstrap data. Ownership is checked once more
   directly before the rebuild.
7. **Finalizes** the claim: the server gets the `HCloudMachine`'s name and the identity label
   `machine.caph-name`, and the claimant and timestamp labels are dropped.

The identity label is applied only in the last step, after a successful rebuild. That keeps the claim
idempotent. If the rebuild fails, the claim is released and the server returns to the pool, and a
claimed but not yet rebuilt server is never mistaken for a provisioned node. From the last step on the
server behaves like a freshly created one.

If no matching server is available, the controller creates a new one as usual. Recycling is an
optimization, not a requirement.

### Placement groups

hcloud only moves a server into a placement group while the server is off, and it refuses to add one
that is already in a group. A claim therefore powers the server off and, if it still carries another
cluster's group, takes it out of that one first. Each of those is a separate hcloud action, so the
claim spans a few reconciles before the rebuild starts.

A spread placement group is what keeps servers on separate physical hosts, which matters for control
planes: one provisioned outside the group can share a host with another. The claim moves a server into
the group the template names, so a pool shared between control planes and workers does not need to be
split by group. An `HCloudMachine` that names no placement group leaves whatever group the server
carries alone.


### A claim is not a lock

Hetzner replaces a server's whole label set on update and has no atomic label write, so a claim is
optimistic: the last write wins. Splitting the write and the check across two reconciles gives a
competing write time to land and be seen, and picking the candidate at random keeps concurrent claims
apart. Neither of the two makes a claim exclusive.

Safety does not depend on it. Ownership is re-checked directly before the rebuild, which is the only
destructive step, and an `HCloudMachine` that lost its claim does not touch labels it no longer owns.
Losing a race costs a reconcile, not a server.

### Losing a claimed server

A claim can be lost at three points, and what happens next depends on which one:

- **Before the rebuild**, at the check in step 4: `caph-recycle-claimant` names someone else, so the
  `HCloudMachine` starts over and picks another candidate.
- **Directly before the rebuild**, at the last ownership check: same label, same answer, but there is
  no candidate left to try in this reconcile, so the `HCloudMachine` creates a new server instead.
- **After provisioning**: the server carries someone else's `machine.caph-name`. This case needs its
  own check, because from here on the controller finds its server by ProviderID, which says nothing
  about who owns it; without the check an `HCloudMachine` would keep provisioning a server another one
  had taken over, and would return it to the pool on deletion while the new owner runs on it. Instead
  it annotates its CAPI `Machine` with `cluster.x-k8s.io/remediate-machine` and stops reconciling.

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

On deletion a server carrying `caph-recycle=true` is not deleted. The controller powers it off,
detaches it from the cluster's private network, and returns it to the pool: the
`HCloudMachine`-owned labels are removed and `caph-recycle-available=true` is set again.

The power off comes first, while the server still carries the `HCloudMachine`'s identity, so a failed
attempt can be retried. `caph-recycle-available` is written last, once hcloud reports the server as
off, so everything in the pool is off and the next claim does not have to stop it. There is nothing
left to shut down gracefully for at this point: the node has been drained, and the next claim
overwrites the disk with a rebuild before the server is used again. A graceful shutdown would depend
on the guest reacting to it and could take minutes or never finish.

The network has to be detached explicitly. A normal delete relies on the server deletion to detach it,
so a server that is kept would stay attached and later block deletion of the network.

This is decided from the server's own label, not from the `HCloudMachine`'s configuration. A
recyclable server is therefore never destroyed, even if recycling has been disabled since.

## Limitations

- Recycling requires `imageName` and cannot be combined with `customProvisioner`.
- Matching is by server type and location. Beyond that the candidates are interchangeable.
- **Create-time server properties are not reproduced by a rebuild.** The public network configuration
  and the SSH keys of a claimed server are the ones the pool server already had; they are not applied
  from the template. Node bootstrap does not need injected SSH keys, because the bootstrap data
  arrives through cloud-init, but you cannot rely on the template's SSH keys being present on a
  recycled node.
- **A claimed server is powered off, not shut down gracefully.** A server offered to the pool while it
  is still running is cut off when an `HCloudMachine` claims it. That claim overwrites the disk with a
  rebuild in the next step, so only mark a server as available once you are done with what is on it.
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
  Attaching a private network adds two, a placement group two more, and powering a running server off
  or taking it out of another group one each. Hetzner rate-limits per project, so a large fleet
  reconciling at once uses a noticeable share of that budget.
- Cleanup after a failed claim is best-effort. If both the rebuild and the release fail, for example
  during a longer API outage, a server can stay claimed. That is logged, and the claim is reclaimed
  once it ages out. Such a server is never mistaken for a provisioned node, so it costs capacity, not
  correctness.
