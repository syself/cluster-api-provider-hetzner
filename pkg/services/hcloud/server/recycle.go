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
	"fmt"
	"maps"
	"sort"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// labelValueTrue is the value carried by the recycling marker labels.
const labelValueTrue = "true"

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
// type and location, and turns it into a node for this machine by reserving it, rebuilding it with the
// given image and bootstrap data, and finally applying the machine's identity. It returns the claimed
// server, or nil if no recyclable server could be claimed, in which case the caller creates a new
// server as usual.
//
// The claim is deliberately split into a reserve step and a finalize step. The machine identity that
// the server lookup keys on is applied only after a successful rebuild, so a rebuild that fails leaves
// a reserved server that no later reconcile can mistake for a provisioned machine. Any failure releases
// the reservation, returning the server to the pool for a clean retry.
func (s *Service) tryClaimRecyclableServer(ctx context.Context, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) (*hcloud.Server, error) {
	listOpts := hcloud.ServerListOpts{}
	listOpts.LabelSelector = utils.LabelsToLabelSelector(map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
	})

	candidates, err := s.scope.HCloudClient.ListServers(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list recyclable servers: %w", err)
	}

	// Reserve the candidate with the lowest ID first, so that if machine reconciles ever run
	// concurrently they contend on the same server rather than scattering across the pool (see
	// reserveRecyclableServer for the concurrency caveat).
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })

	for _, candidate := range candidates {
		if !serverMatchesRequest(candidate, opts) {
			continue
		}

		reserved, err := s.reserveRecyclableServer(ctx, candidate)
		if err != nil {
			// A failed reservation must not block provisioning: log and try the next candidate.
			s.scope.Error(err, "failed to reserve recyclable server, trying next candidate", "serverID", candidate.ID)
			continue
		}
		if reserved == nil {
			// Another machine won the reservation; try the next candidate.
			continue
		}

		// The reserved server does not carry this machine's identity yet, so any failure from here can
		// be undone by releasing the reservation, leaving nothing a later reconcile could adopt as a
		// provisioned machine.
		if err := s.rebuildRecyclableServer(ctx, reserved, opts, image, userData); err != nil {
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

	return nil, nil
}

// serverMatchesRequest reports whether a recyclable server can satisfy a create request. Only servers
// of the requested type and location qualify, so a MachineDeployment never claims a server that does
// not match its HCloudMachineTemplate.
func serverMatchesRequest(server *hcloud.Server, opts hcloud.ServerCreateOpts) bool {
	if opts.ServerType != nil && (server.ServerType == nil || server.ServerType.Name != opts.ServerType.Name) {
		return false
	}
	if opts.Location != nil && (server.Location == nil || server.Location.Name != opts.Location.Name) {
		return false
	}
	return true
}

// reserveRecyclableServer takes a server out of the available pool by replacing its labels with the
// recycle marker and a claimant label naming this machine. The claimant label is intentionally not the
// label the server lookup matches on, so a reserved server is not yet adopted as this machine's server.
//
// Label writes are last-writer-wins and Hetzner offers no conditional (compare-and-swap) label update,
// so the reservation cannot be made truly atomic. After the update the claimant is re-read as a
// best-effort check: it catches the common case where another machine has overwritten the claimant, but
// two machines that each complete their update+re-read without interleaving can both observe their own
// name and proceed. Reliable single-owner reservation therefore relies on machine reconciles being
// serialized, which they are with the default --hcloudmachine-concurrency=1 (see the docs "Limitations").
func (s *Service) reserveRecyclableServer(ctx context.Context, server *hcloud.Server) (*hcloud.Server, error) {
	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:         labelValueTrue,
		infrav1.ServerRecycleClaimantLabelKey: s.scope.Name(),
	}
	if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
		return nil, fmt.Errorf("failed to reserve server %d: %w", server.ID, err)
	}

	reserved, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify reservation of server %d: %w", server.ID, err)
	}
	if reserved == nil || reserved.Labels[infrav1.ServerRecycleClaimantLabelKey] != s.scope.Name() {
		return nil, nil
	}
	return reserved, nil
}

// rebuildRecyclableServer prepares a reserved server and rebuilds it with the machine's image and
// bootstrap data, so that it boots into the desired node exactly like a freshly created server.
func (s *Service) rebuildRecyclableServer(ctx context.Context, server *hcloud.Server, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) error {
	if image == nil {
		return fmt.Errorf("cannot rebuild server %d: image is nil", server.ID)
	}
	// Attach the private network before rebuilding, so cloud-init sees it on first boot. Rebuild
	// preserves existing attachments, so this is skipped when the server is already attached.
	for _, network := range opts.Networks {
		err := s.scope.HCloudClient.AttachServerToNetwork(ctx, server, hcloud.ServerAttachToNetworkOpts{Network: network})
		if err != nil && !hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAttached) {
			return fmt.Errorf("failed to attach server %d to network: %w", server.ID, err)
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
	// The claimant and available markers are intentionally not carried over: the server is now owned.

	finalized, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{
		Name:   s.scope.Name(),
		Labels: labels,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to apply machine identity to server %d: %w", server.ID, err)
	}
	return finalized, nil
}

// releaseReservation returns a reserved server to the available pool by replacing its labels with the
// recycle and available markers, dropping the claimant marker. It is best-effort cleanup on a failed
// claim: an error is logged rather than returned, because the caller is already returning the original
// failure and a leftover reserved server is harmless (it is simply out of the pool until relabeled).
func (s *Service) releaseReservation(ctx context.Context, server *hcloud.Server) {
	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
	}
	if _, err := s.scope.HCloudClient.UpdateServer(ctx, server, hcloud.ServerUpdateOpts{Labels: labels}); err != nil {
		s.scope.Error(err, "failed to release recyclable server reservation; server may need to be relabeled manually", "serverID", server.ID)
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
