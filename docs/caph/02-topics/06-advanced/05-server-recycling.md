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

To hand an existing server to CAPH, set both labels on it (for example with `hcloud server add-label`),
making sure its server type and location match the machines that should claim it. Any server — including
one that was originally created by hand — becomes schedulable this way, without rebuilding it up front.

## How a claim works

When a machine with recycling enabled is created, the controller:

1. Lists servers carrying both `caph-recycle=true` and `caph-recycle-available=true`.
2. Filters them down to those whose server type and location match the machine, and picks the candidate
   with the lowest ID first, so concurrent claims from different machines contend deterministically.
3. **Reserves** the candidate by dropping the `caph-recycle-available` marker and adding a transient
   `caph-recycle-claimant` label naming the machine, then re-reads it to confirm the reservation was not
   lost to another machine that wrote at the same time.
4. **Rebuilds** the reserved server with the machine's image and bootstrap data.
5. **Finalizes** the claim by giving the server the machine's name and labels and dropping the claimant
   marker.

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
- Matching is by server type and location only. A machine never claims a server that does not match its
  template, but it does not otherwise distinguish between candidates beyond picking the lowest ID.
- The size of the recyclable set is not managed by CAPH. If more machines are created than there are
  available recyclable servers, the surplus machines create new servers normally.
- **Claiming is safe only while machine reconciles are serialized.** A server is reserved with an
  optimistic label write plus a re-read, but Hetzner has no atomic (compare-and-swap) label update, so
  two machines reconciling at the same time can, in a narrow window, both believe they reserved the same
  server. This does not happen with the default `--hcloudmachine-concurrency=1` (reconciles run one at a
  time); raising that flag while using recycling can cause two machines to claim one server. Keep
  `--hcloudmachine-concurrency=1` when recycling is in use.
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
- Cleanup after a failed claim is best-effort. If both the rebuild/finalize *and* the subsequent release
  fail (for example during a sustained API outage), a server can be left reserved (labelled with a
  claimant, not `available`); it is logged and needs to be relabelled back to `caph-recycle-available`
  by hand. It is never mistaken for a provisioned node, so this is a capacity/billing concern, not a
  correctness one.
