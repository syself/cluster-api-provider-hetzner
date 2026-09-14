/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// labelValueTrue is the value carried by the recycling marker labels.
const labelValueTrue = "true"

// recycleClaimVerifyDelay is how long a machine waits between writing a claim and verifying it. The
// delay is the point of the split: it gives a competing claim time to land, so that the verification
// reads the settled winner instead of the writer's own value.
const recycleClaimVerifyDelay = 1 * time.Second

// recycleClaimTTL bounds how long a claim may stay unverified before another machine treats it as
// abandoned. A live claim is verified within seconds, so this is orders of magnitude above anything
// legitimate and never races an attempt that is still making progress.
const recycleClaimTTL = 15 * time.Minute

// errRecycleClaimPending is returned once a claim has been written but not yet verified. The caller
// turns it into a short requeue; the claim is picked up again by the next reconcile.
var errRecycleClaimPending = errors.New("recyclable server claim pending verification")

// errRecycleClaimLost signals that a claim this machine held is now held by another machine. It aborts
// the claim without releasing anything, because those labels are not ours to reset any more.
var errRecycleClaimLost = errors.New("recyclable server claim lost to another machine")

// errRecycleRebuildPrereqPending signals that something the rebuild depends on — the placement group,
// the network attachment — has been requested but has not landed yet. hcloud applies both
// asynchronously, and a server with an action still running is locked and rejects the rebuild, so the
// claim is kept and simply retried on the next reconcile.
var errRecycleRebuildPrereqPending = errors.New("recyclable server is not ready for the rebuild yet")

// errRecycleOwnershipLost signals that a server this machine had already finalized is now owned by
// another machine. Unlike a lost claim this is not recoverable by retrying: the machine is provisioning
// hardware that is not its own, so it is handed to remediation.
var errRecycleOwnershipLost = errors.New("recycled server is owned by another machine")

// recyclingEnabled reports whether server recycling is enabled for this machine.
func (s *Service) recyclingEnabled() bool {
	r := s.scope.HCloudMachine.Spec.Recycle
	return r != nil && r.Enabled
}

// isRecyclableServer reports whether a server participates in recycling and must therefore be
// returned to the recyclable set on deletion instead of being deleted.
func isRecyclableServer(server *hcloud.Server) bool {
	return server != nil && server.Labels[infrav1.ServerRecycleLabelKey] == labelValueTrue
}

// tryClaimRecyclableServer looks for an available recyclable server that matches the machine's server
// type and location, and turns it into a node for this machine by claiming it, rebuilding it with the
// given image and bootstrap data, and finally applying the machine's identity. It returns the claimed
// server, or nil if no recyclable server could be claimed, in which case the caller creates a new
// server as usual.
//
// The claim spans two reconciles. Hetzner replaces a server's whole label set on update and offers no
// conditional (compare-and-swap) write, so a claim can never be a lock: whoever writes last wins, and a
// writer that reads back its own value immediately learns nothing, because a competing write may simply
// not have happened yet. Writing the claim and verifying it are therefore separated by
// recycleClaimVerifyDelay: the first reconcile writes and returns errRecycleClaimPending, the next one
// reads the settled result and continues only if this machine is still the claimant.
//
// That makes a collision unlikely, not impossible — two claims further apart than the delay still each
// observe themselves. Safety comes from re-checking ownership at the destructive step (see
// rebuildRecyclableServer) and from a losing machine backing off rather than releasing labels it no
// longer owns. Claims left behind by a machine that died mid-claim are returned to the pool by
// reapAbandonedClaims.
//
// The machine identity that the server lookup keys on is applied only after a successful rebuild, so a
// rebuild that fails leaves a claimed server that no later reconcile can mistake for a provisioned
// machine.
func (s *Service) tryClaimRecyclableServer(ctx context.Context, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) (*hcloud.Server, error) {
	// One list covers all three questions below: do we hold a claim, is anything abandoned, and what is
	// free. Filtering in memory keeps this to a single API call per reconcile.
	listOpts := hcloud.ServerListOpts{}
	listOpts.LabelSelector = utils.LabelsToLabelSelector(map[string]string{
		infrav1.ServerRecycleLabelKey: labelValueTrue,
	})

	servers, err := s.scope.HCloudClient.ListServers(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list recyclable servers: %w", err)
	}

	// A claim written by an earlier reconcile is verified now rather than when it was written, which is
	// what gives a competing claim the chance to overwrite it and be seen.
	if pending := s.pendingClaims(servers); len(pending) > 0 {
		// More than one claim under this machine's name means an earlier attempt wrote a claim it never
		// completed. Keep the first and hand the rest back, so they do not sit out of the pool.
		for _, extra := range pending[1:] {
			s.releaseReservation(ctx, extra)
		}
		return s.completeClaim(ctx, pending[0], opts, image, userData)
	}

	// Claims that no longer belong to a live attempt are returned to the pool before a candidate is
	// picked, so an abandoned claim costs one reconcile rather than a server. The listing above is a
	// snapshot taken before those writes, so it has to be refreshed to see them.
	if reaped := s.reapAbandonedClaims(ctx, servers); reaped > 0 {
		servers, err = s.scope.HCloudClient.ListServers(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to list recyclable servers after reclaiming abandoned ones: %w", err)
		}
	}

	candidates := make([]*hcloud.Server, 0, len(servers))
	rejected := make([]string, 0, len(servers))
	for _, server := range servers {
		if server.Labels[infrav1.ServerRecycleAvailableLabelKey] != labelValueTrue {
			// In use or claimed by someone else. Not a mismatch worth reporting.
			continue
		}
		if reason := recycleMismatchReason(server, opts); reason != "" {
			rejected = append(rejected, fmt.Sprintf("%s (ID %d): %s", server.Name, server.ID, reason))
			continue
		}
		candidates = append(candidates, server)
	}
	if len(candidates) == 0 {
		// Falling through to a normal create is correct, but silently doing so is not: the operator
		// prepared a pool and is about to be billed for a new server without ever learning why.
		if len(rejected) > 0 {
			msg := fmt.Sprintf("no available recyclable server matches this machine, creating a new one instead: %s",
				strings.Join(capStrings(rejected, maxReportedRejections), "; "))
			s.scope.Info(msg)
			record.Eventf(s.scope.HCloudMachine, "NoMatchingRecyclableServer", "%s", msg)
		}
		return nil, nil
	}

	// Claim the candidate with the lowest ID, so that concurrent reconciles contend on the same server
	// rather than scattering across the pool. Contention is detected and resolved on the next reconcile;
	// scattering would quietly consume several servers to provision one machine.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })

	candidate := candidates[0]
	if err := s.writeClaim(ctx, candidate); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("claimed server %d, verifying after %s: %w", candidate.ID, recycleClaimVerifyDelay, errRecycleClaimPending)
}

// claimedByThisMachine reports whether a server carries this machine's claim.
func (s *Service) claimedByThisMachine(server *hcloud.Server) bool {
	return server.Labels[infrav1.ServerRecycleClaimantLabelKey] == s.scope.Name()
}

// pendingClaims returns the servers currently claimed by this machine, lowest ID first.
func (s *Service) pendingClaims(servers []*hcloud.Server) []*hcloud.Server {
	pending := make([]*hcloud.Server, 0, 1)
	for _, server := range servers {
		if s.claimedByThisMachine(server) {
			pending = append(pending, server)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].ID < pending[j].ID })
	return pending
}

// completeClaim turns a verified claim into this machine's server: it rebuilds the server with the
// machine's image and bootstrap data and then applies the machine identity. Any failure that still
// leaves us the claimant releases the claim, so the server returns to the pool instead of being
// stranded outside it.
func (s *Service) completeClaim(ctx context.Context, reserved *hcloud.Server, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) (*hcloud.Server, error) {
	// A claim can outlive the request that produced it, for example when the machine template changed
	// between the two reconciles. A server that no longer matches goes back to the pool.
	if !serverMatchesRequest(reserved, opts) {
		s.releaseReservation(ctx, reserved)
		return nil, nil
	}

	if err := s.rebuildRecyclableServer(ctx, reserved, opts, image, userData); err != nil {
		if errors.Is(err, errRecycleClaimLost) {
			s.scope.Info("lost the claim on a recyclable server before rebuilding it, creating a new server instead",
				"serverID", reserved.ID)
			return nil, nil
		}
		if errors.Is(err, errRecycleRebuildPrereqPending) {
			// Keep the claim: the next reconcile finds it and continues once hcloud has applied the change.
			s.scope.Info("waiting for a claimed server to be ready for the rebuild", "serverID", reserved.ID, "reason", err.Error())
			return nil, fmt.Errorf("claimed server %d: %w", reserved.ID, errRecycleClaimPending)
		}
		s.releaseReservation(ctx, reserved)
		return nil, fmt.Errorf("failed to rebuild recyclable server %d: %w", reserved.ID, err)
	}

	claimed, err := s.finalizeReservation(ctx, reserved, opts)
	if err != nil {
		s.releaseReservation(ctx, reserved)
		return nil, fmt.Errorf("failed to finalize recyclable server %d: %w", reserved.ID, err)
	}

	record.Eventf(s.scope.HCloudMachine, "RecycledServer",
		"Recycled existing server %s (ID %d) instead of creating a new one", claimed.Name, claimed.ID)
	return claimed, nil
}

// reapAbandonedClaims returns servers to the pool whose claim is older than recycleClaimTTL. A claim is
// written and verified within seconds, so one that old belongs to an attempt that died in between.
// Without this its server would stay invisible to every lookup, because claiming drops the available
// marker: the pool would shrink silently, one server per crash.
//
// Two machines reaping the same server write the same labels, so this needs no coordination. Claims
// held by this machine are handled by the verification path and are skipped here.
//
// It returns how many servers were actually returned to the pool, so the caller knows its listing is
// stale.
func (s *Service) reapAbandonedClaims(ctx context.Context, servers []*hcloud.Server) int {
	reaped := 0
	for _, server := range servers {
		claimant := server.Labels[infrav1.ServerRecycleClaimantLabelKey]
		if claimant == "" || s.claimedByThisMachine(server) {
			continue
		}
		claimedAt, ok := claimTimestamp(server)
		if !ok {
			// A claim without a timestamp predates this field and cannot be aged, so it is left alone
			// and logged: silently dropping it could wipe a claim that is still in use.
			s.scope.Info("recyclable server holds a claim without a timestamp and is not reclaimed automatically; relabel it by hand to return it to the pool",
				"serverID", server.ID, "claimant", claimant)
			continue
		}
		if time.Since(claimedAt) < recycleClaimTTL {
			continue
		}

		labels := map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
		}
		if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
			s.scope.Error(err, "failed to return an abandoned recyclable server to the pool",
				"serverID", server.ID, "claimant", claimant)
			continue
		}
		reaped++
		record.Eventf(s.scope.HCloudMachine, "ReclaimedAbandonedServer",
			"Returned server %s (ID %d) to the recyclable set: the claim by %q was never completed", server.Name, server.ID, claimant)
	}
	return reaped
}

// serverAttachedToNetwork reports whether the server already has a private IP on the given network.
func serverAttachedToNetwork(server *hcloud.Server, networkID int64) bool {
	for _, attached := range server.PrivateNet {
		if attached.Network != nil && attached.Network.ID == networkID {
			return true
		}
	}
	return false
}

// maxReportedRejections bounds how many mismatching servers a single message names, so a large pool
// cannot turn one unmatched machine into an unreadable event.
const maxReportedRejections = 5

// capStrings returns at most n entries, noting how many were left out.
func capStrings(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return append(items[:n:n], fmt.Sprintf("and %d more", len(items)-n))
}

// claimTimestamp reads the time at which a server's claim was written.
func claimTimestamp(server *hcloud.Server) (time.Time, bool) {
	raw := server.Labels[infrav1.ServerRecycleClaimedAtLabelKey]
	if raw == "" {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0), true
}

// serverMatchesRequest reports whether a recyclable server can satisfy a create request, so that a
// MachineDeployment never claims a server that does not match its HCloudMachineTemplate.
//
// A rebuild only replaces the disk. Everything the create request would otherwise have decided —
// server type, location, placement group, public addresses — stays whatever the pool server already
// had, so any of those that cannot be corrected has to disqualify the candidate here instead of
// producing a node that quietly differs from its template.
func serverMatchesRequest(server *hcloud.Server, opts hcloud.ServerCreateOpts) bool {
	return recycleMismatchReason(server, opts) == ""
}

// recycleMismatchReason explains why a server cannot satisfy a create request, or returns an empty
// string if it can. The reason is not decoration: when nothing in the pool matches, the machine quietly
// creates a new server instead, and an operator who prepared that pool has no way to tell whether it was
// ignored, mislabelled or subtly mismatched. Deriving both the decision and the explanation here keeps
// the two from drifting apart.
func recycleMismatchReason(server *hcloud.Server, opts hcloud.ServerCreateOpts) string {
	if opts.ServerType != nil && (server.ServerType == nil || server.ServerType.Name != opts.ServerType.Name) {
		return fmt.Sprintf("server type is %q, machine wants %q", serverTypeName(server), opts.ServerType.Name)
	}
	if opts.Location != nil && (server.Location == nil || server.Location.Name != opts.Location.Name) {
		return fmt.Sprintf("location is %q, machine wants %q", locationName(server), opts.Location.Name)
	}
	if !placementGroupSatisfiable(server, opts) {
		return fmt.Sprintf("already in placement group %q, machine wants %q (hcloud cannot move a server between groups)",
			server.PlacementGroup.Name, opts.PlacementGroup.Name)
	}
	return ""
}

func serverTypeName(server *hcloud.Server) string {
	if server.ServerType == nil {
		return "<none>"
	}
	return server.ServerType.Name
}

func locationName(server *hcloud.Server) string {
	if server.Location == nil {
		return "<none>"
	}
	return server.Location.Name
}

// placementGroupSatisfiable reports whether the server can end up in the placement group the request
// asks for. hcloud offers no way to take a server out of a group, so a server that already sits in a
// different one can never satisfy the request; a server in no group at all is fine, because
// rebuildRecyclableServer adds it before the rebuild.
//
// This is not cosmetic. Control planes are spread across physical hosts via a spread placement group;
// a control plane silently provisioned outside that group turns a single host failure into a lost etcd
// quorum. A machine that asks for no group is left alone: keeping a group it already had constrains
// nothing the template cares about.
func placementGroupSatisfiable(server *hcloud.Server, opts hcloud.ServerCreateOpts) bool {
	if opts.PlacementGroup == nil {
		return true
	}
	return server.PlacementGroup == nil || server.PlacementGroup.ID == opts.PlacementGroup.ID
}

// writeClaim takes a server out of the available pool by replacing its labels with the recycle marker,
// a claimant label naming this machine, and the time of the write. The claimant label is intentionally
// not the label the server lookup matches on, so a claimed server is not yet adopted as this machine's
// server.
//
// This deliberately does not read back what it wrote. An immediate re-read only proves that no
// competing write has landed *yet*, which is no evidence at all; the verification happens a reconcile
// later, in tryClaimRecyclableServer.
func (s *Service) writeClaim(ctx context.Context, server *hcloud.Server) error {
	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleClaimantLabelKey:  s.scope.Name(),
		infrav1.ServerRecycleClaimedAtLabelKey: strconv.FormatInt(time.Now().Unix(), 10),
	}
	if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
		return fmt.Errorf("failed to claim server %d: %w", server.ID, err)
	}
	return nil
}

// assertStillClaimed re-reads a server and confirms this machine is still its claimant, returning
// errRecycleClaimLost if it is not.
func (s *Service) assertStillClaimed(ctx context.Context, server *hcloud.Server) error {
	current, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
	if err != nil {
		return fmt.Errorf("failed to verify the claim on server %d: %w", server.ID, err)
	}
	if current == nil || !s.claimedByThisMachine(current) {
		return errRecycleClaimLost
	}
	return nil
}

// rebuildRecyclableServer prepares a reserved server and rebuilds it with the machine's image and
// bootstrap data, so that it boots into the desired node exactly like a freshly created server.
func (s *Service) rebuildRecyclableServer(ctx context.Context, server *hcloud.Server, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) error {
	if image == nil {
		return fmt.Errorf("cannot rebuild server %d: image is nil", server.ID)
	}

	// The last check before the point of no return: a rebuild wipes the disk, so it must not run on a
	// server another machine has claimed since the verification. What remains unguarded after this is
	// the network attach below, which is reversible, and the rebuild call itself.
	if err := s.assertStillClaimed(ctx, server); err != nil {
		return err
	}

	// Put the server in the requested placement group before the rebuild. serverMatchesRequest has
	// already ruled out a server that sits in a different group, so this only ever adds one that is in
	// none.
	//
	// hcloud performs this asynchronously and the rebuild below would race it — a server with an action
	// still running is locked and rejects the rebuild. Rather than poll, the reconcile stops here and
	// picks the claim up again on its next pass: the claim stays ours, so this costs a reconcile and
	// nothing else.
	if opts.PlacementGroup != nil && server.PlacementGroup == nil {
		// hcloud only moves a stopped server into a placement group. A server that was just returned to
		// the pool may still be on its way down, because returning it issues an asynchronous ACPI
		// shutdown and re-pools it immediately. Ask again and come back on the next reconcile; the claim
		// stays ours in the meantime.
		if server.Status != hcloud.ServerStatusOff {
			if err := s.scope.HCloudClient.ShutdownServer(ctx, server); err != nil {
				return fmt.Errorf("failed to shut down server %d before adding it to placement group %d: %w",
					server.ID, opts.PlacementGroup.ID, err)
			}
			return fmt.Errorf("server %d is %q and must be off to join placement group %d: %w",
				server.ID, server.Status, opts.PlacementGroup.ID, errRecycleRebuildPrereqPending)
		}
		if err := s.scope.HCloudClient.AddServerToPlacementGroup(ctx, server, opts.PlacementGroup); err != nil {
			return fmt.Errorf("failed to add server %d to placement group %d: %w", server.ID, opts.PlacementGroup.ID, err)
		}
		current, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
		if err != nil {
			return fmt.Errorf("failed to confirm the placement group of server %d: %w", server.ID, err)
		}
		if current == nil || current.PlacementGroup == nil || current.PlacementGroup.ID != opts.PlacementGroup.ID {
			return fmt.Errorf("server %d is not in placement group %d yet: %w",
				server.ID, opts.PlacementGroup.ID, errRecycleRebuildPrereqPending)
		}
		server.PlacementGroup = current.PlacementGroup
	}

	// Attach the private network before rebuilding, so cloud-init sees it on first boot. Rebuild
	// preserves existing attachments, so an already attached server is left alone.
	//
	// Like the placement group above, hcloud attaches asynchronously, and a server with an action still
	// running is locked and rejects the rebuild. The attachment is therefore confirmed before rebuilding
	// and the reconcile comes back later if it has not landed yet; the claim stays ours meanwhile.
	if len(opts.Networks) > 0 {
		for _, network := range opts.Networks {
			err := s.scope.HCloudClient.AttachServerToNetwork(ctx, server, hcloud.ServerAttachToNetworkOpts{Network: network})
			if err != nil && !hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAttached) {
				return fmt.Errorf("failed to attach server %d to network: %w", server.ID, err)
			}
		}

		current, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
		if err != nil {
			return fmt.Errorf("failed to confirm the network attachment of server %d: %w", server.ID, err)
		}
		if current == nil {
			return fmt.Errorf("server %d disappeared while attaching it to its network: %w", server.ID, errRecycleClaimLost)
		}
		for _, network := range opts.Networks {
			if !serverAttachedToNetwork(current, network.ID) {
				return fmt.Errorf("server %d is not attached to network %d yet: %w",
					server.ID, network.ID, errRecycleRebuildPrereqPending)
			}
		}
	}

	rebuildOpts := hcloud.ServerRebuildOpts{Image: image}
	if len(userData) > 0 {
		rebuildOpts.UserData = ptr.To(string(userData))
	}
	if _, err := s.scope.HCloudClient.RebuildServer(ctx, server, rebuildOpts); err != nil {
		return fmt.Errorf("failed to rebuild server %d: %w", server.ID, err)
	}
	return nil
}

// finalizeReservation applies this machine's identity to a rebuilt server: it gives the server the
// machine's name and the labels a freshly created server would carry (plus the persistent recycle
// marker), dropping the claimant marker. Only after this does the server look like a normal server of
// this machine, both in the Hetzner console and for the server lookup, so the rest of the reconcile
// flow drives it exactly like a newly created one.
func (s *Service) finalizeReservation(ctx context.Context, server *hcloud.Server, opts hcloud.ServerCreateOpts) (*hcloud.Server, error) {
	labels := maps.Clone(opts.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[infrav1.ServerRecycleLabelKey] = labelValueTrue
	// The claimant, claimed-at and available markers are intentionally not carried over: the server is
	// now owned, and ownership is expressed by the machine identity labels.

	finalized, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{
		Name:   s.scope.Name(),
		Labels: labels,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to apply machine identity to server %d: %w", server.ID, err)
	}
	return finalized, nil
}

// releaseReservation returns a claimed server to the available pool by replacing its labels with the
// recycle and available markers, dropping the claimant marker. It is best-effort cleanup on a failed
// claim: an error is logged rather than returned, because the caller is already returning the original
// failure.
//
// The claim is re-checked first, and nothing is written unless this machine is still the claimant.
// Releasing unconditionally would let a machine that failed late reset the labels of a server another
// machine has since claimed — destroying that claim and advertising the server as available while it is
// being rebuilt.
func (s *Service) releaseReservation(ctx context.Context, server *hcloud.Server) {
	if err := s.assertStillClaimed(ctx, server); err != nil {
		if !errors.Is(err, errRecycleClaimLost) {
			s.scope.Error(err, "failed to verify the claim before releasing a recyclable server; it may need to be relabeled manually",
				"serverID", server.ID)
		}
		return
	}

	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
	}
	if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
		s.scope.Error(err, "failed to release a recyclable server claim; it may need to be relabeled manually", "serverID", server.ID)
	}
}

// returnServerToRecycling is the delete-path counterpart of tryClaimRecyclableServer. Instead of
// deleting a recyclable server, it shuts the server down and returns it to the available pool so it can
// be claimed again by a future machine.
//
// The shutdown happens first, while the server still carries this machine's identity, so that a failed
// shutdown can be retried: the machine can still find the server on the next reconcile. The available
// marker is re-added last, so the server rejoins the pool only once it is on its way down. The shutdown
// is an asynchronous ACPI request, so the server may still report Running for a short while (or longer
// if the OS ignores ACPI); this is harmless because the node has already been drained and the next
// claim rebuilds the server before it is used again.
func (s *Service) returnServerToRecycling(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	// A deleted machine must only return a server it still owns. Because findServer resolves by
	// ProviderID, a machine that lost its server to another claimant would otherwise arrive here and
	// shut down and re-pool a server the new owner is actively running.
	if owner := server.Labels[infrav1.MachineNameTagKey]; owner != s.scope.Name() {
		msg := fmt.Sprintf("not returning server %d to the recyclable set: it is owned by %q, not by this machine",
			server.ID, owner)
		s.scope.Info(msg, "serverID", server.ID, "owner", owner)
		record.Warnf(s.scope.HCloudMachine, "ForeignRecycledServerNotReturned", "%s", msg)
		return reconcile.Result{}, nil
	}

	if server.Status != hcloud.ServerStatusOff {
		if err := s.scope.HCloudClient.ShutdownServer(ctx, server); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to shut down server %d before recycling: %w", server.ID, err)
		}
	}

	// Detach the server from the cluster's private network. A normal delete relies on server deletion
	// to detach the network implicitly; because recycling keeps the server, it must detach explicitly,
	// otherwise the server stays attached and later blocks deletion of the network (for example when the
	// cluster is torn down). The network is attached again on the next claim.
	if network := s.scope.HetznerCluster.Status.Network; network != nil {
		err := s.scope.HCloudClient.DetachServerFromNetwork(ctx, server, hcloud.ServerDetachFromNetworkOpts{
			Network: &hcloud.Network{ID: network.ID},
		})
		if err != nil && !hcloud.IsError(err, hcloud.ErrorCodeServerNotAttachedToNetwork) {
			return reconcile.Result{}, fmt.Errorf("failed to detach server %d from network: %w", server.ID, err)
		}
	}

	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
	}
	if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to return server %d to recycling: %w", server.ID, err)
	}

	record.Eventf(s.scope.HCloudMachine, "ReturnedServerToRecycling",
		"Returned server %s (ID %d) to the recyclable set instead of deleting it", server.Name, server.ID)
	return reconcile.Result{}, nil
}

// assertRecycledServerStillOwned guards the one window the claim protocol cannot close by itself.
//
// Once a claim is finalized the claimant label is gone and ownership is expressed by the machine
// identity label. But findServer resolves a server by ProviderID and never looks at labels, so a
// machine whose server has since been taken over by another machine keeps provisioning it, unaware —
// and would, on deletion, shut down and return a server that now belongs to someone else. The claim
// verification and the pre-rebuild check both happen earlier and cannot see this.
//
// So the label is re-checked on every reconcile that touches a live server. A machine that no longer
// owns what it is provisioning is handed to remediation, which deletes it and returns the server to the
// pool. It returns false when ownership was lost, in which case the caller must stop reconciling.
//
// Only recycling-enabled machines are checked: no other machine can claim a normally created server.
func (s *Service) assertRecycledServerStillOwned(ctx context.Context, server *hcloud.Server) (bool, error) {
	if server == nil || !s.recyclingEnabled() {
		return true, nil
	}
	owner := server.Labels[infrav1.MachineNameTagKey]
	if owner == s.scope.Name() {
		return true, nil
	}

	msg := fmt.Sprintf("recycled server %d is owned by %q instead of this machine; handing the machine to remediation",
		server.ID, owner)
	s.scope.Error(errRecycleOwnershipLost, msg, "serverID", server.ID, "owner", owner)
	record.Warnf(s.scope.HCloudMachine, "RecycledServerOwnershipLost", "%s", msg)

	if err := s.scope.SetErrorAndRemediate(ctx, msg); err != nil {
		return false, fmt.Errorf("failed to mark machine for remediation after losing server %d: %w", server.ID, err)
	}
	return false, nil
}
