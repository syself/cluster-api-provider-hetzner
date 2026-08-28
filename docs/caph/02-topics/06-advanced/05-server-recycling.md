---
title: Server recycling
metatitle: Reusing existing Hetzner Cloud servers instead of creating new ones
sidebar: Server recycling
description: Back a MachineDeployment with a fixed set of pre-existing servers that are rebuilt and reused across scale events instead of being created and destroyed.
---

Server recycling lets an `HCloudMachine` reuse a pre-existing Hetzner Cloud server instead of creating a
new one. When a machine with recycling enabled is provisioned, the controller looks for a server that is
labelled as recyclable and matches the machine's server type and location, claims it, and rebuilds it
with the machine's bootstrap data. On deletion the server is returned to the recyclable set instead of
being deleted.

> Throughout this page, *labels* means **Hetzner Cloud** server labels (managed through the hcloud API
> or CLI, e.g. `hcloud server add-label`) — **not** Kubernetes labels.

This is useful when a fixed set of servers should back a `MachineDeployment` — for example servers on a
reserved or otherwise favourable plan — so that scaling up and down reuses those servers rather than
provisioning and destroying a fresh server on every scale event. Because a recycled server is a fully
managed node like any other, it also takes part in machine health checks and remediation. See
[Limitations](#limitations) for how remediation of a recycled server interacts with the pool.

## Enabling recycling

Recycling is configured per machine, through the `HCloudMachineTemplate` of a `MachineDeployment` (or on
a standalone `HCloudMachine`):

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
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

Recycling currently supports only `imageName` (snapshot provisioning); the admission webhook rejects
`recycle.enabled: true` together with `imageURL`. This is a scoping decision for now, not a hard
limitation: the `imageURL` flow provisions the OS through the rescue system rather than the hcloud
*rebuild* API that the recycle path uses, so it could be supported later by re-running that rescue flow
on a claimed server. It is left out for now, so recycling is snapshot-only.

## Marking a server as recyclable

A server joins the recyclable set by carrying two labels:

| Label | Value | Meaning |
| --- | --- | --- |
| `caph-recycle` | `true` | The server participates in recycling and is never deleted by the controller. |
| `caph-recycle-available` | `true` | The server is currently unclaimed and may be picked up by a machine. |

While a machine is claiming a server, the controller adds two further labels itself. They are transient
and are dropped again once the claim completes; you never set them by hand.

| Label | Value | Meaning |
| --- | --- | --- |
| `caph-recycle-claimant` | machine name | The machine currently claiming the server. |
| `caph-recycle-claimed-at` | Unix seconds | When the claim was written, so an abandoned claim can be aged out. |

To hand an existing server to CAPH, set both of the first two labels on it (for example with `hcloud server add-label`),
making sure its server type and location match the machines that should claim it. Any server — including
one that was originally created by hand — becomes schedulable this way, without rebuilding it up front.

## How a claim works

When a machine with recycling enabled is created, the controller:

1. Lists servers carrying `caph-recycle=true`.
2. Filters them down to those that are available and whose server type and location match the machine,
   and picks the candidate with the lowest ID first, so concurrent claims from different machines
   contend deterministically.
3. **Claims** the candidate by dropping the `caph-recycle-available` marker and adding a transient
   `caph-recycle-claimant` label naming the machine, plus `caph-recycle-claimed-at` with the time of the
   write. It then **stops and requeues** without reading anything back.
4. On the **next reconcile**, it reads the claim again. If another machine's write landed in the
   meantime, that machine is now the claimant and this one backs off and starts over.
5. **Rebuilds** the claimed server with the machine's image and bootstrap data, after re-checking
   ownership one last time immediately before the rebuild and, if the template names a placement group
   the server is not yet in, adding it to that group.
6. **Finalizes** the claim by giving the server the machine's name and labels and dropping the claimant
   and timestamp markers.

From then on ownership is expressed by the machine identity label. The controller resolves its server by
ProviderID, which says nothing about who owns it, so that label is re-checked on every reconcile that
touches a live server: a machine whose server has been taken over is handed to remediation rather than
provisioning hardware that is not its own, and it will not return that server to the pool on deletion.

The machine's identity — the label the controller's server lookup keys on — is applied only in the
final step, after a successful rebuild. This keeps the claim idempotent: if the rebuild fails, the
reservation is released and the server returns to the pool, and no half-claimed, un-rebuilt server is
ever mistaken for a provisioned machine. From the final step onward the server is indistinguishable from
a freshly created one and flows through the normal reconcile path.

If no matching recyclable server is available, provisioning falls back to creating a new server as
usual — recycling is a best-effort optimization, never a hard requirement.

On deletion, a server that still carries `caph-recycle=true` is not deleted. Instead the controller
shuts the server down, detaches it from the cluster's private network, and returns it to the pool by
removing the machine-owned labels and re-adding `caph-recycle-available=true`, so it can be claimed
again. The shutdown happens first, while the server still carries the machine's identity, so a failed
shutdown can be retried and the server never rejoins the pool while still running the deleted node. The
network is detached because a normal delete relies on server deletion to detach it implicitly; keeping
the server would otherwise leave it attached and later block deletion of the network. This is decided
from the server's own label rather than the machine's current configuration, so a recyclable server is
never destroyed even if recycling has since been disabled on the machine.

## Limitations

- Recycling requires `imageName`; it cannot be combined with `imageURL`.
- Matching is by server type, location and placement group. Beyond that a machine does not distinguish
  between candidates: it takes the lowest ID.
- **Control planes: mind the placement group.** A rebuild does not move a server between placement
  groups, and hcloud offers no way to take a server out of one. A candidate already sitting in a
  *different* group is therefore skipped, and one in no group is added to the requested group before the
  rebuild. This matters most for control planes, which rely on a spread placement group to sit on
  separate physical hosts: a control plane silently provisioned outside that group turns one host
  failure into a lost etcd quorum. If a pool is meant for control planes, either leave its servers
  without a placement group or put them in the one the template names.

  hcloud only moves a **stopped** server into a placement group, and it does so asynchronously. A server
  that was just returned to the pool may still be shutting down — returning it issues an ACPI request
  and re-pools it immediately — so the claim asks it to stop and retries on the next reconcile. A server
  whose OS ignores ACPI never reaches that state and its claim keeps retrying; the machine stays
  unprovisioned and visible rather than failing silently, but it needs a hand.
- The size of the recyclable set is not managed by CAPH. If more machines are created than there are
  available recyclable servers, the surplus machines create new servers normally.
- **A claim is not a lock.** Hetzner replaces a server's whole label set on update and has no atomic
  (compare-and-swap) label write, so claiming is optimistic: whoever writes last wins. Splitting the
  write from the verification across two reconciles makes a collision unlikely — a competing write has
  time to land and be observed — but two claims further apart than that delay still each read back their
  own name. Safety therefore does not rest on the claim being exclusive; it rests on ownership being
  re-checked immediately before the rebuild (the only destructive step), on a machine that has lost its
  claim backing off instead of touching labels it no longer owns, and on the losing machine simply
  creating a new server. The worst outcome of a lost race is a wasted reconcile, not a shared server.
- **Losing a server needs a `MachineHealthCheck` to be recoverable.** When a machine detects that its
  recycled server now belongs to someone else, it annotates the CAPI machine with
  `cluster.x-k8s.io/remediate-machine` and stops reconciling. Deleting and replacing that machine is
  CAPI's job, not the provider's — and only a `MachineHealthCheck` acts on that annotation. Without one
  the machine stays behind as annotated, unprovisioned and no longer reconciled, and has to be deleted
  by hand. Measured both ways: with a `MachineHealthCheck` the machine was replaced within 30 seconds;
  without one it sat unchanged for over eight minutes. Attach a `MachineHealthCheck` to any
  `MachineDeployment` that uses recycling.
- **A remediated server is returned to the pool, not quarantined.** A recycled server takes part in
  machine health checks like any node; when a `MachineHealthCheck` remediates it, the machine is deleted
  and — because recycling never destroys the server — the server is returned to the recyclable set and
  can be claimed again. For a transient fault this is the intended self-healing. A *persistently* broken
  server (bad disk, hardware fault), however, can enter a remediate-and-recycle loop: claimed → fails its
  health check → remediated → returned to the pool → claimed again. CAPH does not currently quarantine
  such a server automatically. As with bare metal, a hard fault needs manual intervention: remove the
  `caph-recycle` label from the broken server to take it out of the pool, or do not attach an aggressive
  `MachineHealthCheck` to a recycling-backed `MachineDeployment`.
- Create-time server properties are not reproduced by a rebuild. Besides the placement group (a recycled
  server keeps whatever group, if any, it already had), the public-network configuration and SSH keys of
  a claimed server are whatever the pool server had — they are not re-applied from the machine template.
  Node bootstrap does not depend on injected SSH keys (RKE2 joins via cloud-init), but you cannot rely on
  template SSH keys being present on a recycled node.
- A returned server keeps the deleted machine's name until it is claimed again (only its labels are
  reset). Parked pool servers therefore show a stale machine name in the Hetzner console.
- **A claim costs six hcloud API calls**, and more when the template asks for extras: two pool listings
  (one per reconcile), the claim write, one ownership re-check before the rebuild, the rebuild, and the
  finalize. Attaching a private network adds two, a placement group adds two more. Hetzner rate-limits
  per project, so a large fleet of recycling machines reconciling at once uses a noticeable share of the
  budget. The count is pinned by a unit test so it cannot grow unnoticed.
- **The 15-minute claim TTL is compared against the controller's own clock.** A claim carries the wall
  time of the controller that wrote it, and whichever controller later ages it out uses its own clock to
  do so. After a leader change that is a different process; if its clock runs far ahead it can reclaim a
  claim that is still live. Ordinary NTP-synced hosts are nowhere near that margin, and the ownership
  check before the rebuild catches the consequence, but the assumption is worth knowing.
- An abandoned claim is reclaimed after 15 minutes, not immediately. A claim is written and verified
  within seconds, so a controller that dies in between would otherwise pin its server outside the pool
  forever: a claimed server carries no `caph-recycle-available` marker and matches no lookup. Any machine
  that finds a claim older than that returns the server to the pool. Claims written before
  `caph-recycle-claimed-at` existed carry no timestamp, cannot be aged, and are logged instead of
  reclaimed — relabel those by hand.
- Cleanup after a failed claim is best-effort. If both the rebuild/finalize *and* the subsequent release
  fail (for example during a sustained API outage), a server can be left claimed (labelled with a
  claimant, not `available`); it is logged, and reclaimed automatically once the claim ages out
  by hand. It is never mistaken for a provisioned node, so this is a capacity/billing concern, not a
  correctness one.
