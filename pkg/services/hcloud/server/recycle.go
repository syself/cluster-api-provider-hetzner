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
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// labelValueTrue is the value carried by the recycling marker labels.
const labelValueTrue = "true"

// recycleClaimVerifyDelay is how long a machine waits between writing a claim and verifying it. The
// delay is the point of the split: it gives a competing claim time to land, so that the verification
// reads the settled winner instead of the writer's own value.
const recycleClaimVerifyDelay = 1 * time.Second

// recycleClaimTTL bounds how long a claim may exist, counted from its write, before another machine treats
// it as abandoned. A live attempt never gets that old: a machine that still has no server after six
// minutes is handed to remediation, see handleBootStateUnset.
const recycleClaimTTL = 15 * time.Minute

// recycleServerOffDelay is how long the delete path waits before looking again at a server it has
// powered off. hcloud takes several seconds to report such a server as off.
const recycleServerOffDelay = 5 * time.Second

// recycleVolumeDetachDelay is how long the delete path waits before looking again at a server it is
// detaching volumes from. hcloud takes several seconds to detach a volume and lists it on the server
// until it has.
const recycleVolumeDetachDelay = 5 * time.Second

// recycleClaimRetryDelay is how long a machine waits before it claims again after a claim failed. Without
// it a failure that repeats on every attempt would claim and release servers as fast as the controller
// requeues: the requeue that verifies a claim resets the controller's backoff, so the backoff never builds
// up, and every attempt costs several calls from the project's API budget.
const recycleClaimRetryDelay = time.Minute

// recycleClaimWaitDelay is how long a claim waits before it looks again at a change it asked hcloud for,
// such as a power off or leaving a network. Every look is a reconcile with its own hcloud calls, and a
// power off alone takes about ten seconds, so looking every second would cost several times the calls for
// the same result.
const recycleClaimWaitDelay = 5 * time.Second

// errorCodeResourceAlreadyDetaching is what hcloud answers when a server is asked to leave a network it is
// already leaving. hcloud-go has no constant for it.
const errorCodeResourceAlreadyDetaching hcloud.ErrorCode = "resource_already_detaching"

// errorCodeAlreadyInPlacementGroup is what hcloud answers when a server that is already in a placement
// group is asked to join one. hcloud-go has no constant for it.
const errorCodeAlreadyInPlacementGroup hcloud.ErrorCode = "already_in_placement_group"

// rebuildServerCommand is the command of the action hcloud records for a rebuild.
const rebuildServerCommand = "rebuild_server"

// errRecycleClaimPending is returned once a claim has been written but not yet verified. The caller
// turns it into a short requeue; the claim is picked up again by the next reconcile.
var errRecycleClaimPending = errors.New("recyclable server claim pending verification")

// errRecycleClaimWaiting marks a pending claim that waits for hcloud to finish preparing the server. It
// matches errRecycleClaimPending as well, but the caller waits recycleClaimWaitDelay instead of
// recycleClaimVerifyDelay.
var errRecycleClaimWaiting = fmt.Errorf("claimed server is waiting for hcloud: %w", errRecycleClaimPending)

// errRecycleClaimFailed is returned when a claim failed and this machine gave it up. The caller waits
// recycleClaimRetryDelay before the next attempt.
var errRecycleClaimFailed = errors.New("recyclable server claim failed")

// errRecycleClaimLost signals that a claim this machine held is now held by another machine. It aborts
// the claim without releasing anything, because those labels are not ours to reset any more.
var errRecycleClaimLost = errors.New("recyclable server claim lost to another machine")

// errRecycleRebuildPrereqPending signals that a step of the preparation for the rebuild has been requested
// but has not landed yet: leaving or joining a network or a placement group, or the power off. hcloud
// applies all of them asynchronously, and a server with an action still running is locked and rejects the
// next one, so the claim is kept and the step is checked again on the next reconcile.
var errRecycleRebuildPrereqPending = errors.New("recyclable server is not ready for the rebuild yet")

// errRecycleOwnershipLost signals that a server this machine had already finalized is now owned by
// another machine. Unlike a lost claim this is not recoverable by retrying: the machine is provisioning
// hardware that is not its own, so it is handed to remediation.
var errRecycleOwnershipLost = errors.New("recycled server is owned by another machine")

// recyclingEnabled reports whether server recycling is enabled for this machine.
func (s *Service) recyclingEnabled() bool {
	return s.scope.HCloudMachine.Spec.ServerRecycling == infrav1.ServerRecyclingEnabled
}

// recycledServerOwnedByAnotherMachine reports whether a recyclable server carries the identity of a
// machine other than the named one, which is the case once another machine has taken it over.
func recycledServerOwnedByAnotherMachine(server *hcloud.Server, machineName string) bool {
	return isRecyclableServer(server) && server.Labels[infrav1.MachineNameTagKey] != machineName
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
// That makes a collision unlikely, not impossible: two claims further apart than the delay still each
// observe themselves. Safety comes from re-checking ownership at the destructive step (see
// rebuildRecyclableServer) and from a losing machine backing off rather than releasing labels it no
// longer owns. Claims left behind by an HCloudMachine that went away before completing them are returned
// to the pool by reapAbandonedClaims.
//
// The machine identity that the server lookup keys on is applied only after the rebuild, so no reconcile
// can mistake a claimed but not yet rebuilt server for a provisioned machine.
func (s *Service) tryClaimRecyclableServer(ctx context.Context, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) (*hcloud.Server, error) {
	// One list covers all the questions below: is there a server we already finalized, do we hold a claim,
	// is anything abandoned, and what is free. Filtering in memory keeps this to a single API call per
	// reconcile.
	listOpts := hcloud.ServerListOpts{}
	listOpts.LabelSelector = utils.LabelsToLabelSelector(map[string]string{
		infrav1.ServerRecycleLabelKey: labelValueTrue,
	})

	servers, err := s.scope.HCloudClient.ListServers(ctx, listOpts)
	if err != nil {
		return nil, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "ListServers", "failed to list recyclable servers")
	}

	// A server that already carries this HCloudMachine's identity is its server. That happens when the
	// finalize reached hcloud but the ProviderID never made it into the HCloudMachine, for example because
	// patching the HCloudMachine failed or the controller restarted in between. The server carries no
	// claim any more, so without this the HCloudMachine would claim a second one and leave the first
	// running without an owner. A normally created server is recovered the same way, through the
	// uniqueness error that the second create runs into.
	if finalized := s.finalizedServers(servers, opts); len(finalized) > 0 {
		if len(finalized) > 1 {
			return nil, fmt.Errorf("found %d recyclable servers carrying the identity of %s", len(finalized), s.scope.Name())
		}
		// Claims this HCloudMachine still holds are not needed any more.
		for _, extra := range s.pendingClaims(servers) {
			s.releaseReservation(ctx, extra)
		}
		adopted := finalized[0]
		s.scope.Info("adopting a recycled server that was already finalized for this HCloudMachine", "serverID", adopted.ID)
		s.scope.EventRecorder.Eventf(s.scope.HCloudMachine, corev1.EventTypeNormal, "AdoptedRecycledServer",
			"Adopted recycled server %s (ID %d), which was finalized for this HCloudMachine before", adopted.Name, adopted.ID)
		return adopted, nil
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
			return nil, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "ListServers",
				"failed to list recyclable servers after reclaiming abandoned ones")
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
			s.scope.EventRecorder.Event(s.scope.HCloudMachine, corev1.EventTypeNormal, "NoMatchingRecyclableServer", msg)
		}
		return nil, nil
	}

	// Pick a random candidate. With a fixed order every HCloudMachine goes for the same server, so
	// under contention only one of them wins a round and the rest have to retry. Spreading the picks
	// keeps concurrent claims apart.
	//
	// The trade-off: a claim that was written but is not visible in the listing yet now leads to a
	// claim on a different server instead of the same one again, so an HCloudMachine can briefly hold
	// two. pendingClaims releases the extra ones on the next reconcile, which costs a reconcile and
	// not a server.
	candidate := candidates[rand.IntN(len(candidates))] //nolint:gosec // Spreading claims over the pool, not a security decision.
	if err := s.writeClaim(ctx, candidate); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("claimed server %d, verifying after %s: %w", candidate.ID, recycleClaimVerifyDelay, errRecycleClaimPending)
}

// claimedByThisMachine reports whether a server carries this machine's claim.
func (s *Service) claimedByThisMachine(server *hcloud.Server) bool {
	return server.Labels[infrav1.ServerRecycleClaimantLabelKey] == s.scope.Name()
}

// finalizedServers returns the servers that carry every label this machine's create request asks for,
// including the machine identity. A recyclable server only gets those from a finalize for this machine.
func (s *Service) finalizedServers(servers []*hcloud.Server, opts hcloud.ServerCreateOpts) []*hcloud.Server {
	var finalized []*hcloud.Server
	for _, server := range servers {
		if server.Labels[infrav1.MachineNameTagKey] != s.scope.Name() {
			continue
		}
		matches := true
		for key, value := range opts.Labels {
			if server.Labels[key] != value {
				matches = false
				break
			}
		}
		if matches {
			finalized = append(finalized, server)
		}
	}
	return finalized
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
// machine's image and bootstrap data and then applies the machine identity. A step that hcloud has not
// finished yet, a locked server and a rate limit keep the claim for the next reconcile, and so does any
// failure after the rebuild. Any other failure releases the claim, so the server returns to the pool
// instead of being stranded outside it.
func (s *Service) completeClaim(ctx context.Context, reserved *hcloud.Server, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) (*hcloud.Server, error) {
	// A claim can outlive the request that produced it, for example when the server's type or protection
	// was changed meanwhile. A server that no longer matches goes back to the pool.
	if !serverMatchesRequest(reserved, opts) {
		s.releaseReservation(ctx, reserved)
		return nil, nil
	}

	if err := s.rebuildRecyclableServer(ctx, reserved, opts, image, userData); err != nil {
		switch {
		case errors.Is(err, errRecycleClaimLost):
			s.scope.Info("lost the claim on a recyclable server before rebuilding it, creating a new server instead",
				"serverID", reserved.ID)
			return nil, nil
		case errors.Is(err, errRecycleRebuildPrereqPending), hcloud.IsError(err, hcloud.ErrorCodeLocked):
			// Keep the claim: the next reconcile finds it and continues once hcloud has applied the change.
			// A locked server has an action still running, usually one this claim started itself on an
			// earlier reconcile, such as the power off or the placement group change, so it is the same
			// wait.
			s.scope.Info("waiting for a claimed server to be ready for the rebuild", "serverID", reserved.ID, "reason", err.Error())
			return nil, fmt.Errorf("claimed server %d: %w", reserved.ID, errRecycleClaimWaiting)
		case hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded):
			// Keep the claim as well. Releasing it would take the API calls that just ran out, and the
			// controller waits out the rate limit before it reconciles this HCloudMachine again.
			return nil, err
		}
		s.releaseReservation(ctx, reserved)
		return nil, s.claimFailed(reserved, err)
	}

	claimed, err := s.finalizeReservation(ctx, reserved, opts)
	if err != nil {
		// The rebuild has started the server with this machine's bootstrap data, so releasing it would
		// put a booting node into the pool. The claim is kept, and the next reconcile finishes the
		// finalize without rebuilding a second time, see rebuiltByThisClaim.
		return nil, fmt.Errorf("failed to finalize recyclable server %d: %w", reserved.ID, err)
	}

	s.scope.EventRecorder.Eventf(s.scope.HCloudMachine, corev1.EventTypeNormal, "RecycledServer",
		"Recycled existing server %s (ID %d) instead of creating a new one", claimed.Name, claimed.ID)
	return claimed, nil
}

// claimFailed records that this machine gave up a claim and marks the error, so that the caller waits
// recycleClaimRetryDelay before the next attempt.
func (s *Service) claimFailed(server *hcloud.Server, err error) error {
	s.scope.EventRecorder.Eventf(s.scope.HCloudMachine, corev1.EventTypeWarning, "RecycledServerClaimFailed",
		"Gave up the claim on recyclable server %s (ID %d), trying again in %s: %s", server.Name, server.ID, recycleClaimRetryDelay, err)
	return fmt.Errorf("%w: %w", errRecycleClaimFailed, err)
}

// claimRequeueDelay returns how long a machine with a pending claim waits before the next reconcile: long
// enough for a competing claim to land when the claim was just written, and longer while it waits for
// hcloud to finish preparing the server.
func claimRequeueDelay(err error) time.Duration {
	if errors.Is(err, errRecycleClaimWaiting) {
		return recycleClaimWaitDelay
	}
	return recycleClaimVerifyDelay
}

// reapAbandonedClaims returns servers to the pool whose claim is older than recycleClaimTTL, powered off.
// A claim is abandoned when its HCloudMachine goes away before completing it, for example because it is
// deleted or remediated before it has a server. A controller restart does not abandon a claim: the
// claimant label names the HCloudMachine, and its next reconcile continues the claim. Without this an
// abandoned claim's server would stay invisible to every lookup, because claiming drops the available
// marker: the pool would shrink silently, one server per abandoned claim.
//
// Two machines reaping the same server write the same labels, so this needs no coordination. Claims
// held by this machine are handled by the verification path and are skipped here. Only a recycling
// machine that is looking for a server runs this, so an abandoned claim waits for the next one.
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
			// A claim without a valid timestamp was not written by writeClaim, for example one set by
			// hand. It cannot be aged, so it is left alone and logged: silently dropping it could wipe a
			// claim that is still in use.
			s.scope.Info("recyclable server holds a claim without a timestamp and is not reclaimed automatically; relabel it by hand to return it to the pool",
				"serverID", server.ID, "claimant", claimant)
			continue
		}
		if time.Since(claimedAt) < recycleClaimTTL {
			continue
		}
		s.powerOffForPool(ctx, server)

		labels := map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
		}
		if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
			err = handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "UpdateServer",
				fmt.Sprintf("failed to return abandoned recyclable server %d to the pool", server.ID))
			s.scope.Error(err, "failed to return an abandoned recyclable server to the pool",
				"serverID", server.ID, "claimant", claimant)
			continue
		}
		reaped++
		s.scope.EventRecorder.Eventf(s.scope.HCloudMachine, corev1.EventTypeNormal, "ReclaimedAbandonedServer",
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
// A rebuild only replaces the disk, and a claim does not change a server's type or location, so a server
// that differs in either is not a candidate. Neither is a server with rebuild protection, because hcloud
// refuses to rebuild it. The placement group and the private networks are brought in line with the request
// before the rebuild, and attached volumes are detached, see rebuildRecyclableServer. The public network
// and the SSH keys stay what the pool server already had; the documentation lists that as a limitation.
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
	if server.Protection.Rebuild {
		return "rebuild protection is enabled"
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

// serverInPlacementGroup reports whether the server already sits in the given placement group.
func serverInPlacementGroup(server *hcloud.Server, group *hcloud.PlacementGroup) bool {
	return server.PlacementGroup != nil && group != nil && server.PlacementGroup.ID == group.ID
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
		return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "UpdateServer",
			fmt.Sprintf("failed to claim server %d", server.ID))
	}
	return nil
}

// assertStillClaimed re-reads a server and confirms this machine is still its claimant. It returns the
// server as hcloud reports it now, or errRecycleClaimLost if the claim is no longer this machine's.
func (s *Service) assertStillClaimed(ctx context.Context, server *hcloud.Server) (*hcloud.Server, error) {
	current, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
	if err != nil {
		return nil, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "GetServer",
			fmt.Sprintf("failed to verify the claim on server %d", server.ID))
	}
	if current == nil || !s.claimedByThisMachine(current) {
		return nil, errRecycleClaimLost
	}
	return current, nil
}

// rebuildRecyclableServer prepares a reserved server and rebuilds it with the machine's image and
// bootstrap data, so that it boots into the desired node exactly like a freshly created server.
//
// The preparation can take several reconciles: a step that hcloud has to apply first ends the reconcile
// with errRecycleRebuildPrereqPending, and the next one checks the claim again and goes on from there. The
// networks come first and the placement group last. hcloud starts a server when it joins a placement
// group, and the server then boots what is still on its disk, which is the node it was before; joining
// last keeps that boot short, because the rebuild follows right after.
func (s *Service) rebuildRecyclableServer(ctx context.Context, server *hcloud.Server, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) error {
	if image == nil {
		return fmt.Errorf("cannot rebuild server %d: image is nil", server.ID)
	}

	// A rebuild wipes the disk, so it must not run on a server another machine has claimed since the
	// verification. The steps below run without another check in the same reconcile; a step that ends
	// the reconcile is checked again on the next one.
	current, err := s.assertStillClaimed(ctx, server)
	if err != nil {
		return err
	}

	// Rebuild keeps network attachments, including ones this machine does not want. A claim that fails
	// after the attach below is released or reaped with only its labels reset, so its server returns to
	// the pool still attached to that cluster's network; a server labelled by hand can carry networks of
	// its own. Either would give the node a private IP in a network it does not belong to, so those
	// networks are detached first.
	if err := s.detachUnrequestedNetworks(ctx, current, opts.Networks); err != nil {
		return err
	}

	// Attach the private network before rebuilding, so cloud-init sees it on first boot. The attach is
	// sent on every pass, and hcloud answers server_already_attached once the server is attached.
	//
	// hcloud attaches asynchronously, and a server with an action still running is locked and rejects
	// the rebuild. The attachment is therefore confirmed before rebuilding, and the reconcile comes back
	// later if it has not landed yet; the claim stays ours meanwhile.
	if len(opts.Networks) > 0 {
		for _, network := range opts.Networks {
			err := s.scope.HCloudClient.AttachServerToNetwork(ctx, current, hcloud.ServerAttachToNetworkOpts{Network: network})
			if err != nil && !hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAttached) {
				return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "AttachServerToNetwork",
					fmt.Sprintf("failed to attach server %d to network %d", server.ID, network.ID))
			}
		}

		attached, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
		if err != nil {
			return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "GetServer",
				fmt.Sprintf("failed to confirm the network attachment of server %d", server.ID))
		}
		if attached == nil {
			return fmt.Errorf("server %d disappeared while attaching it to its network: %w", server.ID, errRecycleClaimLost)
		}
		for _, network := range opts.Networks {
			if !serverAttachedToNetwork(attached, network.ID) {
				return fmt.Errorf("server %d is not attached to network %d yet: %w",
					server.ID, network.ID, errRecycleRebuildPrereqPending)
			}
		}
	}

	// Put the server in the requested placement group. This matters for control planes: a spread group
	// keeps them on separate physical hosts, and one provisioned outside the group can share a host with
	// another. A machine that asks for no group is left alone, because the group a server happens to
	// carry constrains nothing the template cares about.
	if opts.PlacementGroup != nil && !serverInPlacementGroup(current, opts.PlacementGroup) {
		// A server returned to the pool keeps the group of the cluster that used it, and hcloud rejects
		// adding a server that is already in a group. Take it out first; unlike adding, removing works
		// on a running server. The next reconcile sees it without a group and goes on from there.
		if current.PlacementGroup != nil {
			if err := s.scope.HCloudClient.RemoveServerFromPlacementGroup(ctx, current); err != nil {
				return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "RemoveServerFromPlacementGroup",
					fmt.Sprintf("failed to remove server %d from placement group %d", server.ID, current.PlacementGroup.ID))
			}
			return fmt.Errorf("server %d is leaving placement group %d to join %d: %w",
				server.ID, current.PlacementGroup.ID, opts.PlacementGroup.ID, errRecycleRebuildPrereqPending)
		}

		// hcloud only moves a stopped server into a placement group. A server that the delete path
		// returned to the pool is off already; one an operator labelled by hand can still be running, so
		// it is powered off here. That is a hard stop, which is what the claim does to it anyway: the
		// rebuild below overwrites the disk. Come back on the next reconcile once hcloud reports it as
		// off; the claim stays ours in the meantime.
		if current.Status != hcloud.ServerStatusOff {
			// A server that is stopping is still on an earlier power off, so it is only checked again.
			if current.Status != hcloud.ServerStatusStopping {
				if err := s.scope.HCloudClient.PowerOffServer(ctx, current); err != nil {
					return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "PowerOffServer",
						fmt.Sprintf("failed to power off server %d before adding it to placement group %d", server.ID, opts.PlacementGroup.ID))
				}
			}
			return fmt.Errorf("server %d is %q and must be off to join placement group %d: %w",
				server.ID, current.Status, opts.PlacementGroup.ID, errRecycleRebuildPrereqPending)
		}
		// A listing taken while an earlier join was still running shows no group yet, so the join is sent
		// again; hcloud then answers already_in_placement_group, and the check below decides.
		err := s.scope.HCloudClient.AddServerToPlacementGroup(ctx, current, opts.PlacementGroup)
		if err != nil && !hcloud.IsError(err, errorCodeAlreadyInPlacementGroup) {
			return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "AddServerToPlacementGroup",
				fmt.Sprintf("failed to add server %d to placement group %d", server.ID, opts.PlacementGroup.ID))
		}
		joined, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
		if err != nil {
			return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "GetServer",
				fmt.Sprintf("failed to confirm the placement group of server %d", server.ID))
		}
		if joined == nil || joined.PlacementGroup == nil || joined.PlacementGroup.ID != opts.PlacementGroup.ID {
			return fmt.Errorf("server %d is not in placement group %d yet: %w",
				server.ID, opts.PlacementGroup.ID, errRecycleRebuildPrereqPending)
		}
	}

	// A rebuild starts the server with this machine's bootstrap data. If an earlier reconcile got that far
	// and then failed to finalize, the server is already booting this machine's node, and a second rebuild
	// would wipe it while it joins. hcloud starts every server it rebuilds, so a server that was off when
	// this reconcile began cannot have been rebuilt by this claim, and the action log is only read for one
	// that is running.
	if current.Status != hcloud.ServerStatusOff {
		rebuilt, err := s.rebuiltByThisClaim(ctx, current)
		if err != nil {
			return err
		}
		if rebuilt {
			s.scope.Info("an earlier reconcile already rebuilt the claimed server, finishing the claim", "serverID", server.ID)
			return nil
		}
	}

	// A rebuild replaces the server's own disk, but attached volumes stay attached, and the new node could
	// mount them and read the data of the cluster they belong to. The delete path detaches them before it
	// returns a server, but a server labelled by hand can still have some, and nothing stops a volume from
	// being attached to a parked server. So the rebuild waits until hcloud lists no volume on the server.
	// This comes after the check above, so it never detaches a volume from the node this claim started.
	if len(current.Volumes) > 0 {
		if err := s.detachVolumes(ctx, current); err != nil {
			return err
		}
		return fmt.Errorf("server %d still has volumes %v attached: %w",
			server.ID, volumeIDs(current), errRecycleRebuildPrereqPending)
	}

	rebuildOpts := hcloud.ServerRebuildOpts{Image: image}
	if len(userData) > 0 {
		rebuildOpts.UserData = ptr.To(string(userData))
	}
	if _, err := s.scope.HCloudClient.RebuildServer(ctx, current, rebuildOpts); err != nil {
		return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "RebuildServer",
			fmt.Sprintf("failed to rebuild server %d", server.ID))
	}
	return nil
}

// rebuiltByThisClaim reports whether hcloud has rebuilt the server, or is rebuilding it, since this
// machine wrote its claim. It compares the claim's timestamp with the start of the rebuild. A controller
// clock that runs ahead of hcloud's by more than the seconds between claim and rebuild makes it miss the
// rebuild, which only costs a second one.
func (s *Service) rebuiltByThisClaim(ctx context.Context, server *hcloud.Server) (bool, error) {
	claimedAt, ok := claimTimestamp(server)
	if !ok {
		return false, nil
	}
	actions, err := s.scope.HCloudClient.ListServerActions(ctx, server, hcloud.ActionListOpts{
		ListOpts: hcloud.ListOpts{PerPage: 25},
		Status:   []hcloud.ActionStatus{hcloud.ActionStatusRunning, hcloud.ActionStatusSuccess},
		Sort:     []string{"id:desc"},
	})
	if err != nil {
		return false, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "ListServerActions",
			fmt.Sprintf("failed to read the actions of server %d", server.ID))
	}
	for _, action := range actions {
		if action.Command == rebuildServerCommand && action.Status != hcloud.ActionStatusError && !action.Started.Before(claimedAt) {
			return true, nil
		}
	}
	return false, nil
}

// detachUnrequestedNetworks detaches the server from the first network it is attached to that is not in
// wanted. hcloud detaches asynchronously and locks the server until it has, so it handles one network
// per reconcile: it returns errRecycleRebuildPrereqPending after a detach, and nil once nothing is left.
// The listing still shows the network while the detach runs, so the next reconcile asks again, and hcloud
// answers resource_already_detaching. That is the same wait.
func (s *Service) detachUnrequestedNetworks(ctx context.Context, server *hcloud.Server, wanted []*hcloud.Network) error {
	for _, attached := range server.PrivateNet {
		if attached.Network == nil || networkRequested(wanted, attached.Network.ID) {
			continue
		}
		err := s.scope.HCloudClient.DetachServerFromNetwork(ctx, server, hcloud.ServerDetachFromNetworkOpts{
			Network: &hcloud.Network{ID: attached.Network.ID},
		})
		if err != nil && !hcloud.IsError(err, errorCodeResourceAlreadyDetaching) {
			return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "DetachServerFromNetwork",
				fmt.Sprintf("failed to detach server %d from network %d", server.ID, attached.Network.ID))
		}
		return fmt.Errorf("server %d is leaving network %d, which this machine does not use: %w",
			server.ID, attached.Network.ID, errRecycleRebuildPrereqPending)
	}
	return nil
}

// networkRequested reports whether the network with the given ID is one of the requested networks.
func networkRequested(networks []*hcloud.Network, id int64) bool {
	for _, network := range networks {
		if network != nil && network.ID == id {
			return true
		}
	}
	return false
}

// detachVolumes asks hcloud to detach every volume attached to the server. hcloud detaches
// asynchronously and lists a volume on the server until it is detached, so the caller looks again later.
// A volume that is still being detached is locked, and hcloud answers another detach with locked; that is
// the same wait.
func (s *Service) detachVolumes(ctx context.Context, server *hcloud.Server) error {
	for _, volume := range server.Volumes {
		err := s.scope.HCloudClient.DetachVolume(ctx, volume)
		if err == nil || hcloud.IsError(err, hcloud.ErrorCodeLocked) {
			continue
		}
		// The server was read at the start of this reconcile. If an earlier detach has finished since,
		// hcloud answers this one with a plain service_error, so the server is read again before the
		// error counts.
		if hcloud.IsError(err, hcloud.ErrorCodeServiceError) {
			attached, checkErr := s.volumeStillAttached(ctx, server.ID, volume.ID)
			if checkErr != nil {
				return checkErr
			}
			if !attached {
				continue
			}
		}
		return handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "DetachVolume",
			fmt.Sprintf("failed to detach volume %d from server %d", volume.ID, server.ID))
	}
	return nil
}

// volumeStillAttached reads the server again and reports whether hcloud still lists the volume on it.
func (s *Service) volumeStillAttached(ctx context.Context, serverID, volumeID int64) (bool, error) {
	current, err := s.scope.HCloudClient.GetServer(ctx, serverID)
	if err != nil {
		return false, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "GetServer",
			fmt.Sprintf("failed to check whether volume %d is still attached to server %d", volumeID, serverID))
	}
	if current == nil {
		return false, fmt.Errorf("server %d disappeared while detaching volume %d", serverID, volumeID)
	}
	for _, volume := range current.Volumes {
		if volume.ID == volumeID {
			return true, nil
		}
	}
	return false, nil
}

// volumeIDs returns the IDs of the volumes attached to the server, for logs and errors.
func volumeIDs(server *hcloud.Server) []int64 {
	ids := make([]int64, 0, len(server.Volumes))
	for _, volume := range server.Volumes {
		ids = append(ids, volume.ID)
	}
	return ids
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
		return nil, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "UpdateServer",
			fmt.Sprintf("failed to apply machine identity to server %d", server.ID))
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
// machine has since claimed, which destroys that claim and advertises the server as available while it is
// being rebuilt. A server that is running is powered off first, see powerOffForPool.
func (s *Service) releaseReservation(ctx context.Context, server *hcloud.Server) {
	current, err := s.assertStillClaimed(ctx, server)
	if err != nil {
		if !errors.Is(err, errRecycleClaimLost) {
			s.scope.Error(err, "failed to verify the claim before releasing a recyclable server; it may need to be relabeled manually",
				"serverID", server.ID)
		}
		return
	}
	s.powerOffForPool(ctx, current)

	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
	}
	if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
		err = handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "UpdateServer",
			fmt.Sprintf("failed to release the claim on server %d", server.ID))
		s.scope.Error(err, "failed to release a recyclable server claim; it may need to be relabeled manually", "serverID", server.ID)
	}
}

// powerOffForPool powers off a server that goes back to the pool after a failed or abandoned claim. A
// claim can have started it: joining a placement group starts a server, and so does a rebuild. It then
// runs the node it was before, or this machine's bootstrap data, and either could join a cluster as a
// node that no Machine owns. This is best-effort cleanup like the release itself, so an error is logged,
// not returned.
func (s *Service) powerOffForPool(ctx context.Context, server *hcloud.Server) {
	if server.Status == hcloud.ServerStatusOff || server.Status == hcloud.ServerStatusStopping {
		return
	}
	if err := s.scope.HCloudClient.PowerOffServer(ctx, server); err != nil {
		err = handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "PowerOffServer",
			fmt.Sprintf("failed to power off server %d before returning it to the pool", server.ID))
		s.scope.Error(err, "failed to power off a recyclable server before returning it to the pool", "serverID", server.ID)
	}
}

// returnServerToRecycling is the delete-path counterpart of tryClaimRecyclableServer. Instead of
// deleting a recyclable server, it powers the server off and returns it to the available pool so it can
// be claimed again by a future machine.
//
// The server is powered off first, while it still carries this machine's identity, so that a retry
// still passes the ownership check below. The available marker is written last, once hcloud reports
// the server as off and without volumes, so a server this path returns is off, cannot boot the old
// node again and carries none of this cluster's volumes.
//
// Stopping means a power off, not an ACPI shutdown. A graceful shutdown would depend on the guest
// reacting to it and could therefore take minutes or never finish, while there is nothing left to shut
// down gracefully for: the node has been drained or its whole cluster is being deleted, and the next
// claim overwrites the disk with a rebuild before the server is used again.
func (s *Service) returnServerToRecycling(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	// A deleted machine must only return a server it still owns. findServer returns the server the
	// ProviderID points to without checking its labels, so a machine that lost its server to another
	// claimant would otherwise arrive here and power off and re-pool a server the new owner is running.
	if owner := server.Labels[infrav1.MachineNameTagKey]; owner != s.scope.Name() {
		// An earlier attempt returned the server already, and only removing the finalizer failed.
		if owner == "" && server.Labels[infrav1.ServerRecycleAvailableLabelKey] == labelValueTrue {
			s.scope.Info("recyclable server is already back in the recyclable set", "serverID", server.ID)
			return reconcile.Result{}, nil
		}
		msg := fmt.Sprintf("not returning server %d to the recyclable set: it is owned by %q, not by this machine",
			server.ID, owner)
		s.scope.Info(msg, "serverID", server.ID, "owner", owner)
		s.scope.EventRecorder.Event(s.scope.HCloudMachine, corev1.EventTypeWarning, "ForeignRecycledServerNotReturned", msg)
		return reconcile.Result{}, nil
	}

	if server.Status != hcloud.ServerStatusOff {
		// A power off takes hcloud about ten seconds. A server that is stopping is still on an earlier
		// one, and while it is, hcloud answers another one with locked; both only mean looking again.
		if server.Status != hcloud.ServerStatusStopping {
			if err := s.scope.HCloudClient.PowerOffServer(ctx, server); err != nil && !hcloud.IsError(err, hcloud.ErrorCodeLocked) {
				return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "PowerOffServer",
					fmt.Sprintf("failed to power off server %d before recycling", server.ID))
			}
		}
		s.scope.Info("powering off a recyclable server before returning it to the recyclable set",
			"serverID", server.ID, "status", server.Status)
		return reconcile.Result{RequeueAfter: recycleServerOffDelay}, nil
	}

	// Detach every volume. A normal delete detaches them by deleting the server, but a rebuild keeps them,
	// so a volume still attached here would reach the node of whichever machine claims the server next,
	// and that node could mount it and read this cluster's data. CAPI waits for the volumes of a deleted
	// Machine to be detached, but skips that wait in some cases, for example when the cluster is being
	// deleted, so the CSI driver may have left some. The server is marked available only once hcloud lists
	// no volume on it.
	if len(server.Volumes) > 0 {
		if err := s.detachVolumes(ctx, server); err != nil {
			return reconcile.Result{}, err
		}
		s.scope.Info("detaching volumes from a recyclable server before returning it to the recyclable set",
			"serverID", server.ID, "volumes", volumeIDs(server))
		return reconcile.Result{RequeueAfter: recycleVolumeDetachDelay}, nil
	}

	// Detach the server from the cluster's private network. A normal delete detaches it by deleting the
	// server; because recycling keeps the server, it detaches explicitly, otherwise a parked server would
	// keep its private IP in this cluster's network. The next claim attaches the network it needs.
	//
	// A server that is no longer attached is skipped, for example when an earlier attempt detached it and
	// then failed to relabel it. hcloud answers a detach from a network the server is not attached to with
	// a plain service_error, so a retry that sent one would fail every time. A detach that is still running
	// is answered with resource_already_detaching; the server is on its way out, so the relabel goes ahead.
	if network := s.scope.HetznerCluster.Status.Network; network != nil && serverAttachedToNetwork(server, network.ID) {
		err := s.scope.HCloudClient.DetachServerFromNetwork(ctx, server, hcloud.ServerDetachFromNetworkOpts{
			Network: &hcloud.Network{ID: network.ID},
		})
		if err != nil && !hcloud.IsError(err, errorCodeResourceAlreadyDetaching) {
			return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "DetachServerFromNetwork",
				fmt.Sprintf("failed to detach server %d from network", server.ID))
		}
	}

	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
	}
	if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, s.scope.EventRecorder, err, "UpdateServer",
			fmt.Sprintf("failed to return server %d to recycling", server.ID))
	}

	s.scope.EventRecorder.Eventf(s.scope.HCloudMachine, corev1.EventTypeNormal, "ReturnedServerToRecycling",
		"Returned server %s (ID %d) to the recyclable set instead of deleting it", server.Name, server.ID)
	return reconcile.Result{}, nil
}

// assertRecycledServerStillOwned guards the one window the claim protocol cannot close by itself.
//
// Once a claim is finalized the claimant label is gone and ownership is expressed by the machine
// identity label. But findServer returns the server the ProviderID points to without checking its labels,
// so a machine whose server has since been taken over by another machine would keep provisioning it,
// unaware. The claim verification and the pre-rebuild check both happen earlier and cannot see this.
//
// So the label is re-checked on every reconcile that touches a live server. A machine that no longer
// owns what it is provisioning is handed to remediation. The server stays with its new owner: the delete
// path checks the identity label itself and leaves the server and its load balancer target alone. It
// returns false when ownership was lost, in which case the caller must stop reconciling.
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
	s.scope.EventRecorder.Event(s.scope.HCloudMachine, corev1.EventTypeWarning, "RecycledServerOwnershipLost", msg)

	if err := s.scope.SetErrorAndRemediate(ctx, msg); err != nil {
		return false, fmt.Errorf("failed to mark machine for remediation after losing server %d: %w", server.ID, err)
	}
	return false, nil
}
