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
have to be rebuilt up front. Turn off its rebuild protection, though: a claim rebuilds the server, so a
server with rebuild protection is skipped.

## How a claim works

When an `HCloudMachine` with recycling enabled is created, the controller:

1. Lists the servers carrying `caph-recycle=true`. This covers the whole Hetzner project, not one
   cluster: if several clusters share a project, a server that one of them returned can be claimed by
   any of the others.
2. Keeps those that are available, match the server type and location and have no rebuild
   protection, and picks one at random.
3. **Claims** it by replacing its labels with `caph-recycle`, `caph-recycle-claimant` with its own
   name, and `caph-recycle-claimed-at`. It then requeues.
4. On the **next reconcile**, reads the labels again. If another `HCloudMachine`'s write landed in
   between, that one is the claimant now, and this one claims another server.
5. **Prepares** the server, if it differs from what the template asks for: it leaves every private
   network other than the cluster's, is attached to the cluster's network, and moves into the
   placement group the template names. hcloud applies each change asynchronously. A step that has to
   wait ends the reconcile, and the next one, five seconds later, checks the claim again and goes on
   from there. A `locked` answer means hcloud is still busy with the previous step and is handled the
   same way.
6. **Rebuilds** the server with the image and the bootstrap data. hcloud starts the server once the
   rebuild is done, whether it was running before or not.
7. **Finalizes** the claim: the server gets the `HCloudMachine`'s name and the identity label
   `machine.caph-name`, and the claimant and timestamp labels are dropped.

Every label write replaces the server's whole label set, so labels you add to a pool server yourself
are gone after its next claim or return.

The identity label is applied only after the rebuild, so a claimed but not yet rebuilt server is never
mistaken for a provisioned node. If a step before the rebuild fails, the claim is released: the server
is powered off and returned to the pool, and the `HCloudMachine` waits a minute before it claims again,
so a failure that repeats on every attempt does not use up the API budget. If the finalize fails, the
claim is kept, because the rebuilt server is already booting this `HCloudMachine`'s node. The next
reconcile sees in hcloud's action log that this claim has rebuilt the server and only repeats the
finalize. From the last step on the server behaves like a freshly created one.

If the finalize reaches hcloud but the server's ProviderID never reaches the `HCloudMachine`, for
example because the controller restarts in between, the next reconcile finds the server by its
`machine.caph-name` label and adopts it instead of claiming a second one.

If no matching server is available, the controller creates a new one as usual. Recycling is an
optimization, not a requirement.

### Placement groups

hcloud only moves a server into a placement group while the server is off, and it refuses to add one
that is already in a group. A claim therefore takes the server out of another cluster's group first and
powers it off. Joining the group starts the server again, and it boots what is still on its disk, the
node it was before, for the few seconds until the rebuild powers it off. That is why the placement
group is the last step before the rebuild.

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

Safety does not depend on it. Every reconcile that prepares or rebuilds the server checks the claim
first, and an `HCloudMachine` that lost its claim does not touch labels it no longer owns. Losing a race
before the rebuild costs a reconcile. Losing it after provisioning costs the losing `Machine`, see
below.

### Losing a claimed server

A claim can be lost at three points, and what happens next depends on which one:

- **At the verification** in step 4: `caph-recycle-claimant` names someone else, so the
  `HCloudMachine` claims another matching server in the same reconcile if there is one, and otherwise
  creates a new server.
- **While the server is prepared or rebuilt**, at the check that starts each of these reconciles:
  same label, same answer, but there is no candidate left to try in this reconcile, so the
  `HCloudMachine` creates a new server instead.
- **After provisioning**: the server carries someone else's `machine.caph-name`. This case needs its
  own check, because from here on the controller finds its server by ProviderID, which says nothing
  about who owns it; without the check an `HCloudMachine` would keep provisioning a server another one
  had taken over. Instead it annotates its CAPI `Machine` with `cluster.x-k8s.io/remediate-machine`
  and stops reconciling. The server stays with its new owner: when the remediated `HCloudMachine` is
  deleted, the delete path sees the other owner's label and leaves the server and its load balancer
  target alone.

Deleting and replacing a `Machine` is CAPI's job, not the provider's, and only a `MachineHealthCheck`
acts on that annotation. Without one the `HCloudMachine` stays behind, annotated and unprovisioned,
and has to be deleted by hand. Attach a `MachineHealthCheck` to any `MachineDeployment` that uses
recycling.

### Abandoned claims

A controller restart does not abandon a claim: the claim carries the `HCloudMachine`'s name, and that
`HCloudMachine` continues it on its next reconcile. A claim is abandoned when its `HCloudMachine` goes
away first, for example when it is deleted or remediated before it has a server. Such a claim would
keep the server out of the pool for good, because a claimed server carries no
`caph-recycle-available` label and matches no lookup. The next recycling `HCloudMachine` that looks
for a server and finds a claim older than 15 minutes powers that server off and returns it to the pool.

## Returning a server to the pool

On deletion a server carrying `caph-recycle=true` is not deleted. The controller powers it off,
detaches it from the cluster's private network, and returns it to the pool: its labels are replaced
with `caph-recycle=true` and `caph-recycle-available=true`.

The power off comes first, while the server still carries the `HCloudMachine`'s identity, so a failed
attempt can be retried. `caph-recycle-available` is written last, once hcloud reports the server as
off, so a server this path returns is off and cannot boot the drained node again. There is nothing
left to shut down gracefully for at this point: the node has been drained, and the next claim
overwrites the disk with a rebuild before the server is used again. A graceful shutdown would depend
on the guest reacting to it and could take minutes or never finish.

The network has to be detached explicitly. A normal delete detaches it by deleting the server, so a
server that is kept would keep its private IP in the cluster's network. A claim that fails after
attaching the network only resets the labels when it releases the server, so the next claim detaches
every network other than its own cluster's before the rebuild.

This is decided from the server's own label, not from the `HCloudMachine`'s configuration. A
recyclable server is therefore never destroyed, even if recycling has been disabled since.

## Limitations

- Recycling requires `imageName` and cannot be combined with `customProvisioner`.
- Matching is by server type and location, and a server with rebuild protection is skipped. Beyond
  that the candidates are interchangeable.
- **Create-time server properties are not reproduced by a rebuild.** The public network configuration
  and the SSH keys of a claimed server are the ones the pool server already had; they are not applied
  from the template. Node bootstrap does not need injected SSH keys, because the bootstrap data
  arrives through cloud-init, but you cannot rely on the template's SSH keys being present on a
  recycled node.
- **A claim stops a running server hard.** A server offered to the pool while it is still running is
  powered off when an `HCloudMachine` claims it, either before it joins a placement group or by the
  rebuild, which powers it off first. The rebuild overwrites the disk, so only mark a server as
  available once you are done with what is on it.
- **A broken server returns to the pool, it is not quarantined.** When a `MachineHealthCheck`
  remediates a recycled node, the `Machine` is deleted and the server goes back to the recyclable set,
  where it can be claimed again. For a transient fault that is the intended self-healing. A server
  with a hard fault can loop instead: claimed, fails its health check, remediated, returned, claimed
  again. Remove the `caph-recycle` label from it while it is parked to take it out of the pool. On a
  server that is in use, removing the label makes the controller delete the server when its `Machine`
  is deleted.
- The size of the recyclable set is not managed by CAPH. If more `HCloudMachine`s are created than
  there are available servers, the surplus ones create new servers.
- A returned server keeps the deleted `HCloudMachine`'s name until it is claimed again. Only its
  labels are reset, so parked servers show a stale name in the Hetzner console.
- **Claims cost hcloud API calls.** Every reconcile of an `HCloudMachine` that has no server yet makes
  four calls before it gets to recycling, the same as a normal create: the server type, two image
  lookups and the SSH keys. A claim adds six over its two reconciles: two pool listings, the claim
  write, the ownership check, the rebuild and the finalize. That is 14 calls where a normal create
  needs five. Attaching a private network adds two, and so does joining a placement group. Each step
  that has to wait for hcloud costs another reconcile every five seconds until it lands, about seven
  calls each, and a server that is running when it is rebuilt costs one more call for its action log.
  In a test on a real project, preparing a running server that sat in another cluster's placement
  group and network took seven of these reconciles and about 40 seconds. Hetzner rate-limits per
  project, so a large fleet reconciling at once uses a noticeable share of that budget. An
  `HCloudMachine` that hits the limit waits it out like for any other hcloud call, and a claim in
  progress keeps its server meanwhile.
- Cleanup after a failed claim is best-effort. If both the rebuild and the release fail, for example
  during a longer API outage, a server can stay claimed. That is logged, and the claim's
  `HCloudMachine` continues it on a later reconcile. If that `HCloudMachine` goes away, the claim is
  reaped once it is 15 minutes old. Such a server is never mistaken for a provisioned node, so it
  costs capacity, not correctness.
