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
	"strconv"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	fakehcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
)

const (
	recycleTestType     = "cpx31"
	recycleTestLocation = "fsn1"
)

func Test_isRecyclableServer(t *testing.T) {
	tests := []struct {
		name   string
		server *hcloud.Server
		want   bool
	}{
		{name: "nil server", server: nil, want: false},
		{name: "no labels", server: &hcloud.Server{}, want: false},
		{
			name:   "recycle label true",
			server: &hcloud.Server{Labels: map[string]string{infrav1.ServerRecycleLabelKey: "true"}},
			want:   true,
		},
		{
			name:   "recycle label false",
			server: &hcloud.Server{Labels: map[string]string{infrav1.ServerRecycleLabelKey: "false"}},
			want:   false,
		},
		{
			name:   "unrelated labels only",
			server: &hcloud.Server{Labels: map[string]string{"foo": "bar"}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isRecyclableServer(tt.server))
		})
	}
}

func Test_recyclingEnabled(t *testing.T) {
	tests := []struct {
		name    string
		recycle *infrav1.ServerRecycling
		want    bool
	}{
		{name: "nil recycle", recycle: nil, want: false},
		{name: "disabled", recycle: &infrav1.ServerRecycling{Enabled: false}, want: false},
		{name: "enabled", recycle: &infrav1.ServerRecycling{Enabled: true}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{scope: newRecycleScope(t, "m", fakehcloudclient.NewHCloudClientFactory().NewClient(""))}
			svc.scope.HCloudMachine.Spec.Recycle = tt.recycle
			require.Equal(t, tt.want, svc.recyclingEnabled())
		})
	}
}

func Test_serverMatchesRequest(t *testing.T) {
	opts := hcloud.ServerCreateOpts{
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
	}
	tests := []struct {
		name   string
		server *hcloud.Server
		opts   hcloud.ServerCreateOpts
		want   bool
	}{
		{
			name:   "type and location match",
			server: &hcloud.Server{ServerType: &hcloud.ServerType{Name: recycleTestType}, Location: &hcloud.Location{Name: recycleTestLocation}},
			opts:   opts,
			want:   true,
		},
		{
			name:   "type mismatch",
			server: &hcloud.Server{ServerType: &hcloud.ServerType{Name: "cx22"}, Location: &hcloud.Location{Name: recycleTestLocation}},
			opts:   opts,
			want:   false,
		},
		{
			name:   "location mismatch",
			server: &hcloud.Server{ServerType: &hcloud.ServerType{Name: recycleTestType}, Location: &hcloud.Location{Name: "nbg1"}},
			opts:   opts,
			want:   false,
		},
		{
			name:   "server has no type",
			server: &hcloud.Server{Location: &hcloud.Location{Name: recycleTestLocation}},
			opts:   opts,
			want:   false,
		},
		{
			name:   "server has no location",
			server: &hcloud.Server{ServerType: &hcloud.ServerType{Name: recycleTestType}},
			opts:   opts,
			want:   false,
		},
		{
			name:   "request without constraints matches any server",
			server: &hcloud.Server{ServerType: &hcloud.ServerType{Name: recycleTestType}, Location: &hcloud.Location{Name: recycleTestLocation}},
			opts:   hcloud.ServerCreateOpts{},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, serverMatchesRequest(tt.server, tt.opts))
		})
	}
}

// claimRecyclable drives the full two-reconcile claim: the first call writes the claim and asks to be
// requeued, the second verifies it and completes. It returns the outcome of the second call.
func claimRecyclable(t *testing.T, svc *Service, opts hcloud.ServerCreateOpts, image *hcloud.Image, userData []byte) (*hcloud.Server, error) {
	t.Helper()
	claimed, err := svc.tryClaimRecyclableServer(context.Background(), opts, image, userData)
	require.ErrorIs(t, err, errRecycleClaimPending, "the first reconcile must write a claim and defer the verification")
	require.Nil(t, claimed, "no server is returned before the claim has been verified")
	return svc.tryClaimRecyclableServer(context.Background(), opts, image, userData)
}

// Test_tryClaimRecyclableServer_firstReconcileOnlyWritesClaim pins the reason the claim is split across
// two reconciles: the first one must write the claim and stop, so that a competing claim has time to
// land before anything is read back or rebuilt.
func Test_tryClaimRecyclableServer_firstReconcileOnlyWritesClaim(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	pool := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.ErrorIs(t, err, errRecycleClaimPending)
	require.Nil(t, claimed)

	got, err := client.GetServer(context.Background(), pool.ID)
	require.NoError(t, err)

	// The claim is written and the server has left the pool...
	require.Equal(t, "worker-1", got.Labels[infrav1.ServerRecycleClaimantLabelKey])
	require.Contains(t, got.Labels, infrav1.ServerRecycleClaimedAtLabelKey, "a claim must be timestamped so it can be reclaimed if abandoned")
	require.NotContains(t, got.Labels, infrav1.ServerRecycleAvailableLabelKey)

	// ...but nothing irreversible has happened: no rebuild, no machine identity.
	require.Equal(t, "pool-a", got.Name, "the machine identity must not be applied before the claim is verified")
	require.Nil(t, got.Image, "the server must not be rebuilt before the claim is verified")
	require.Empty(t, listAvailableRecyclable(t, client))
}

func Test_tryClaimRecyclableServer_claimsAndRebuilds(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	image := &hcloud.Image{ID: 42, Name: "snapshot"}
	claimed, err := claimRecyclable(t, svc, recycleCreateOpts("worker-1"), image, []byte("#cloud-config"))
	require.NoError(t, err)
	require.NotNil(t, claimed, "an available matching server must be claimed")

	// The claimed server carries the machine name (both as server name and label) and the persistent
	// recycle marker, and no longer advertises itself as available or claimed.
	require.Equal(t, "worker-1", claimed.Name)
	require.Equal(t, "worker-1", claimed.Labels[infrav1.MachineNameTagKey])
	require.Equal(t, "true", claimed.Labels[infrav1.ServerRecycleLabelKey])
	require.NotContains(t, claimed.Labels, infrav1.ServerRecycleAvailableLabelKey)
	require.NotContains(t, claimed.Labels, infrav1.ServerRecycleClaimantLabelKey)
	require.NotContains(t, claimed.Labels, infrav1.ServerRecycleClaimedAtLabelKey)

	// It has been rebuilt with the requested image and is running.
	require.NotNil(t, claimed.Image)
	require.Equal(t, image.ID, claimed.Image.ID)
	require.Equal(t, hcloud.ServerStatusRunning, claimed.Status)

	// The pool no longer offers an available server.
	require.Empty(t, listAvailableRecyclable(t, client))
}

func Test_tryClaimRecyclableServer_noMatchingType(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	// Available, but the wrong server type.
	pool := seedRecyclableServer(t, client, "pool-wrong-type", "cx22", recycleTestLocation, true)

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "a server of the wrong type must not be claimed")
	require.Len(t, listAvailableRecyclable(t, client), 1, "the unmatched server stays available")

	got, err := client.GetServer(context.Background(), pool.ID)
	require.NoError(t, err)
	require.NotContains(t, got.Labels, infrav1.ServerRecycleClaimantLabelKey, "no claim may be written for a server that does not match")
}

func Test_tryClaimRecyclableServer_notAvailable(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	// Recyclable and matching, but already claimed (no available marker).
	seedRecyclableServer(t, client, "pool-claimed", recycleTestType, recycleTestLocation, false)

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "a server that is not available must not be claimed")
}

func Test_tryClaimRecyclableServer_emptyPool(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "an empty pool means fall through to a normal create")
}

func Test_tryClaimRecyclableServer_picksLowestID(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	// Two matching available servers; the fake assigns ascending IDs, so "pool-low" gets the lower ID.
	low := seedRecyclableServer(t, client, "pool-low", recycleTestType, recycleTestLocation, true)
	high := seedRecyclableServer(t, client, "pool-high", recycleTestType, recycleTestLocation, true)
	require.Less(t, low.ID, high.ID)

	claimed, err := claimRecyclable(t, svc, recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.Equal(t, low.ID, claimed.ID, "the lowest-ID candidate must be claimed first")

	// The higher-ID server is untouched and still available.
	remaining := listAvailableRecyclable(t, client)
	require.Len(t, remaining, 1)
	require.Equal(t, high.ID, remaining[0].ID)
}

// Test_tryClaimRecyclableServer_losesClaimToLaterWriter is the race the two-reconcile split exists for:
// this machine writes a claim, another machine overwrites it before the verification runs, and the
// verification therefore reads the other machine's name. The loser must back off completely — no
// rebuild, and no touching of labels that now belong to the winner.
func Test_tryClaimRecyclableServer_losesClaimToLaterWriter(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	pool := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	// First reconcile: worker-1 writes its claim.
	_, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.ErrorIs(t, err, errRecycleClaimPending)

	// Between the two reconciles, worker-2's write lands last and therefore wins.
	overwriteClaim(t, client, pool, "worker-2")

	// Second reconcile: worker-1 sees it no longer holds the claim.
	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "losing the claim must fall through to a normal create")

	got, err := client.GetServer(context.Background(), pool.ID)
	require.NoError(t, err)
	require.Equal(t, "worker-2", got.Labels[infrav1.ServerRecycleClaimantLabelKey], "the winner's claim must survive untouched")
	require.Nil(t, got.Image, "the loser must not rebuild a server it does not own")
	require.NotContains(t, got.Labels, infrav1.ServerRecycleAvailableLabelKey, "the loser must not advertise a claimed server as available")
}

// Test_tryClaimRecyclableServer_losesClaimJustBeforeRebuild covers the window the verification cannot
// close: the claim still looks ours when the reconcile starts, and is taken over before the rebuild.
// The check immediately before the destructive call must catch it, because a rebuild wipes the disk.
func Test_tryClaimRecyclableServer_losesClaimJustBeforeRebuild(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	// The list at the start of the reconcile shows a claim held by this machine.
	ours := &hcloud.Server{
		ID:         7,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          "true",
			infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
			infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
		},
	}
	// The re-read just before the rebuild shows it has been taken over since.
	theirs := &hcloud.Server{
		ID: 7,
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:         "true",
			infrav1.ServerRecycleClaimantLabelKey: "worker-2",
		},
	}
	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{ours}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(7)).Return(theirs, nil).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "a lost claim must fall through to a normal create")
	// mc asserts on cleanup that RebuildServer was never called, and that no UpdateServer released
	// labels that now belong to worker-2.
}

// Test_tryClaimRecyclableServer_releasesSupernumeraryClaims covers an attempt that was interrupted after
// writing a claim and then claimed a second server on a later try: the leftover must go back to the pool
// rather than sit outside it forever.
func Test_tryClaimRecyclableServer_releasesSupernumeraryClaims(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	first := seedRecyclableServer(t, client, "pool-first", recycleTestType, recycleTestLocation, false)
	second := seedRecyclableServer(t, client, "pool-second", recycleTestType, recycleTestLocation, false)
	overwriteClaim(t, client, first, "worker-1")
	overwriteClaim(t, client, second, "worker-1")

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.Equal(t, first.ID, claimed.ID, "the lowest-ID claim is the one that is completed")

	got, err := client.GetServer(context.Background(), second.ID)
	require.NoError(t, err)
	require.Equal(t, "true", got.Labels[infrav1.ServerRecycleAvailableLabelKey], "the surplus claim must be returned to the pool")
	require.NotContains(t, got.Labels, infrav1.ServerRecycleClaimantLabelKey)
}

// Test_tryClaimRecyclableServer_rebuildFailureReleasesReservation verifies that when the rebuild fails
// after a server has been claimed, the claim is released (the server returns to the available pool)
// instead of leaving a half-claimed server that no lookup can see any more.
func Test_tryClaimRecyclableServer_rebuildFailureReleasesReservation(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	// A network in opts that does not exist in the fake makes the rebuild's network attach fail.
	opts := recycleCreateOpts("worker-1")
	opts.Networks = []*hcloud.Network{{ID: 999}}

	claimed, err := claimRecyclable(t, svc, opts, &hcloud.Image{ID: 42}, []byte("#cloud-config"))
	require.Error(t, err)
	require.Nil(t, claimed)

	// The claim was released: the server is available again and carries no claimant.
	available := listAvailableRecyclable(t, client)
	require.Len(t, available, 1, "the server must be returned to the available pool")
	require.NotContains(t, available[0].Labels, infrav1.ServerRecycleClaimantLabelKey)
	require.NotContains(t, available[0].Labels, infrav1.MachineNameTagKey, "an un-rebuilt server must not carry the machine identity")
}

// Test_releaseReservation_keepsForeignClaim guards the cleanup path against destroying a claim that is
// no longer ours: a machine failing late must not reset the labels of a server another machine has since
// claimed, because that would both void the winner's claim and advertise a server that is being rebuilt.
func Test_releaseReservation_keepsForeignClaim(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	server := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, false)
	overwriteClaim(t, client, server, "worker-2")

	svc.releaseReservation(context.Background(), server)

	got, err := client.GetServer(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, "worker-2", got.Labels[infrav1.ServerRecycleClaimantLabelKey], "a foreign claim must be left alone")
	require.NotContains(t, got.Labels, infrav1.ServerRecycleAvailableLabelKey)
	require.Empty(t, listAvailableRecyclable(t, client))
}

func Test_releaseReservation_returnsOwnClaim(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	server := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, false)
	overwriteClaim(t, client, server, "worker-1")

	svc.releaseReservation(context.Background(), server)

	got, err := client.GetServer(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, "true", got.Labels[infrav1.ServerRecycleAvailableLabelKey])
	require.NotContains(t, got.Labels, infrav1.ServerRecycleClaimantLabelKey)
}

// Test_reapAbandonedClaims covers the failure mode the claim timestamp exists for: a machine that dies
// between writing a claim and verifying it leaves a server that carries no available marker and is
// therefore invisible to every lookup. Without reaping, the pool would shrink by one server per crash.
func Test_reapAbandonedClaims(t *testing.T) {
	tests := []struct {
		name         string
		claimant     string
		claimedAt    string
		wantReturned bool
	}{
		{
			name:         "claim older than the TTL is returned to the pool",
			claimant:     "worker-2",
			claimedAt:    strconv.FormatInt(time.Now().Add(-2*recycleClaimTTL).Unix(), 10),
			wantReturned: true,
		},
		{
			name:         "claim within the TTL is left alone",
			claimant:     "worker-2",
			claimedAt:    strconv.FormatInt(time.Now().Unix(), 10),
			wantReturned: false,
		},
		{
			name:         "claim without a timestamp cannot be aged and is left alone",
			claimant:     "worker-2",
			claimedAt:    "",
			wantReturned: false,
		},
		{
			name:         "an unparseable timestamp is treated like a missing one",
			claimant:     "worker-2",
			claimedAt:    "not-a-timestamp",
			wantReturned: false,
		},
		{
			name:         "this machine's own claim is left to the verification path",
			claimant:     "worker-1",
			claimedAt:    strconv.FormatInt(time.Now().Add(-2*recycleClaimTTL).Unix(), 10),
			wantReturned: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
			svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

			server := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, false)
			labels := map[string]string{
				infrav1.ServerRecycleLabelKey:         "true",
				infrav1.ServerRecycleClaimantLabelKey: tt.claimant,
			}
			if tt.claimedAt != "" {
				labels[infrav1.ServerRecycleClaimedAtLabelKey] = tt.claimedAt
			}
			_, err := client.UpdateServer(context.Background(), server, hcloud.ServerUpdateOpts{Labels: labels})
			require.NoError(t, err)

			svc.reapAbandonedClaims(context.Background(), []*hcloud.Server{server})

			got, err := client.GetServer(context.Background(), server.ID)
			require.NoError(t, err)
			if tt.wantReturned {
				require.Equal(t, "true", got.Labels[infrav1.ServerRecycleAvailableLabelKey])
				require.NotContains(t, got.Labels, infrav1.ServerRecycleClaimantLabelKey)
				return
			}
			require.Equal(t, tt.claimant, got.Labels[infrav1.ServerRecycleClaimantLabelKey])
			require.NotContains(t, got.Labels, infrav1.ServerRecycleAvailableLabelKey)
		})
	}
}

// Test_reapAbandonedClaims_freesTheServerForANewClaim ties the reaper back to the claim path: once an
// abandoned claim has been reaped, the next reconcile can claim that server normally.
func Test_reapAbandonedClaims_freesTheServerForANewClaim(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	server := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, false)
	_, err := client.UpdateServer(context.Background(), server, hcloud.ServerUpdateOpts{Labels: map[string]string{
		infrav1.ServerRecycleLabelKey:          "true",
		infrav1.ServerRecycleClaimantLabelKey:  "worker-dead",
		infrav1.ServerRecycleClaimedAtLabelKey: strconv.FormatInt(time.Now().Add(-2*recycleClaimTTL).Unix(), 10),
	}})
	require.NoError(t, err)

	// Reaping and claiming happen in the same reconcile: the reaper refreshes the listing it works from,
	// so an abandoned claim costs one reconcile rather than a server.
	claimed, err := claimRecyclable(t, svc, recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed, "the reclaimed server must be usable straight away")
	require.Equal(t, server.ID, claimed.ID)
	require.Equal(t, "worker-1", claimed.Labels[infrav1.MachineNameTagKey])
}

func Test_returnServerToRecycling(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	// A server currently claimed by this machine: recycle marker set, available marker absent.
	claimed := seedRecyclableServer(t, client, "worker-1", recycleTestType, recycleTestLocation, false)
	claimed.Labels[infrav1.MachineNameTagKey] = "worker-1"

	_, err := svc.returnServerToRecycling(context.Background(), claimed)
	require.NoError(t, err)

	got, err := client.GetServer(context.Background(), claimed.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	// The machine-owned labels are stripped and the server is available again.
	require.Equal(t, "true", got.Labels[infrav1.ServerRecycleLabelKey])
	require.Equal(t, "true", got.Labels[infrav1.ServerRecycleAvailableLabelKey])
	require.NotContains(t, got.Labels, infrav1.MachineNameTagKey)
	require.Equal(t, hcloud.ServerStatusOff, got.Status, "the returned server is shut down")
}

// Test_returnServerToRecycling_detachesNetwork verifies that, when the cluster has a private network,
// returning a server also detaches it. A normal delete detaches implicitly via server deletion; since
// recycling keeps the server, an explicit detach is required so the server does not block network
// deletion later. The fake client cannot be seeded with a network from this package, so a mock is used.
func Test_returnServerToRecycling_detachesNetwork(t *testing.T) {
	mc := mocks.NewClient(t)
	scp := newRecycleScope(t, "worker-1", mc)
	scp.HetznerCluster.Status.Network = &infrav1.NetworkStatus{ID: 4711}
	svc := &Service{scope: scp}

	server := &hcloud.Server{
		ID:     5,
		Status: hcloud.ServerStatusRunning,
		Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"},
	}
	mc.On("ShutdownServer", mock.Anything, mock.Anything).Return(nil).Once()
	mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(server, nil).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	// mc asserts on cleanup that ShutdownServer, DetachServerFromNetwork and UpdateServer were all called.
}

// newRecycleScope builds a Service scope for a worker HCloudMachine with recycling enabled, wired to
// the given hcloud client.
func newRecycleScope(t *testing.T, machineName string, client hcloudclient.Client) *scope.MachineScope {
	t.Helper()
	hcloudMachine := &infrav1.HCloudMachine{
		ObjectMeta: metav1.ObjectMeta{Name: machineName, Namespace: "default"},
		Spec: infrav1.HCloudMachineSpec{
			Type:      recycleTestType,
			ImageName: "snapshot",
			Recycle:   &infrav1.ServerRecycling{Enabled: true},
		},
	}
	hcloudMachine.Status.Region = recycleTestLocation
	return newTestService(hcloudMachine, client).scope
}

// recycleCreateOpts mirrors the labels and constraints that createServer would pass for the given
// machine, so a claimed server ends up owned by that machine.
func recycleCreateOpts(machineName string) hcloud.ServerCreateOpts {
	return hcloud.ServerCreateOpts{
		Name:       machineName,
		Labels:     map[string]string{infrav1.MachineNameTagKey: machineName},
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
	}
}

// seedRecyclableServer creates a recyclable server in the fake client's project.
func seedRecyclableServer(t *testing.T, client hcloudclient.Client, name, serverType, location string, available bool) *hcloud.Server {
	t.Helper()
	labels := map[string]string{infrav1.ServerRecycleLabelKey: "true"}
	if available {
		labels[infrav1.ServerRecycleAvailableLabelKey] = "true"
	}
	res, err := client.CreateServer(context.Background(), hcloud.ServerCreateOpts{
		Name:       name,
		Labels:     labels,
		ServerType: &hcloud.ServerType{Name: serverType},
		Location:   &hcloud.Location{Name: location},
	})
	require.NoError(t, err)
	return res.Server
}

// listAvailableRecyclable returns the servers still advertised as available for recycling.
func listAvailableRecyclable(t *testing.T, client hcloudclient.Client) []*hcloud.Server {
	t.Helper()
	var opts hcloud.ServerListOpts
	opts.LabelSelector = infrav1.ServerRecycleAvailableLabelKey + "==true"
	servers, err := client.ListServers(context.Background(), opts)
	require.NoError(t, err)
	return servers
}

// overwriteClaim simulates another machine's label write landing on a server: Hetzner replaces the
// whole label set on update, so a competing claim leaves nothing of the previous one behind.
func overwriteClaim(t *testing.T, client hcloudclient.Client, server *hcloud.Server, claimant string) {
	t.Helper()
	_, err := client.UpdateServer(context.Background(), server, hcloud.ServerUpdateOpts{Labels: map[string]string{
		infrav1.ServerRecycleLabelKey:          "true",
		infrav1.ServerRecycleClaimantLabelKey:  claimant,
		infrav1.ServerRecycleClaimedAtLabelKey: strconv.FormatInt(time.Now().Unix(), 10),
	}})
	require.NoError(t, err)
}

// Test_returnServerToRecycling_refusesForeignServer covers the delete-path counterpart of the ownership
// guard. findServer resolves a server by ProviderID, so a machine that lost its server to another
// claimant still finds it on deletion — and would shut down and re-pool hardware the new owner is
// running on.
func Test_returnServerToRecycling_refusesForeignServer(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	// A recyclable server that now belongs to worker-2.
	server := seedRecyclableServer(t, client, "worker-2", recycleTestType, recycleTestLocation, false)
	_, err := client.UpdateServer(context.Background(), server, hcloud.ServerUpdateOpts{Labels: map[string]string{
		infrav1.ServerRecycleLabelKey: "true",
		infrav1.MachineNameTagKey:     "worker-2",
	}})
	require.NoError(t, err)

	_, err = svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err, "refusing a foreign server is not an error, there is simply nothing to do")

	got, err := client.GetServer(context.Background(), server.ID)
	require.NoError(t, err)
	require.Equal(t, "worker-2", got.Labels[infrav1.MachineNameTagKey], "the owner label must be untouched")
	require.NotContains(t, got.Labels, infrav1.ServerRecycleAvailableLabelKey,
		"a server another machine is running on must not be advertised as available")
	require.NotEqual(t, hcloud.ServerStatusOff, got.Status, "the new owner's server must not be shut down")
}

// Test_assertRecycledServerStillOwned covers the gap that neither the claim verification nor the
// pre-rebuild check can see: findServer resolves a server by ProviderID and never inspects labels, so
// after finalize a machine keeps provisioning a server that another machine has taken over.
func Test_assertRecycledServerStillOwned(t *testing.T) {
	tests := []struct {
		name       string
		recycling  bool
		labels     map[string]string
		wantOwned  bool
		wantRemedy bool
	}{
		{
			name:      "still ours",
			recycling: true,
			labels:    map[string]string{infrav1.MachineNameTagKey: "worker-1"},
			wantOwned: true,
		},
		{
			name:       "taken over by another machine",
			recycling:  true,
			labels:     map[string]string{infrav1.MachineNameTagKey: "worker-2"},
			wantOwned:  false,
			wantRemedy: true,
		},
		{
			name:       "identity label gone entirely",
			recycling:  true,
			labels:     map[string]string{},
			wantOwned:  false,
			wantRemedy: true,
		},
		{
			name:      "recycling disabled: nothing else can claim the server, so no check",
			recycling: false,
			labels:    map[string]string{infrav1.MachineNameTagKey: "worker-2"},
			wantOwned: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
			svc := &Service{scope: newRecycleScope(t, "worker-1", client)}
			if !tt.recycling {
				svc.scope.HCloudMachine.Spec.Recycle = nil
			}

			owned, err := svc.assertRecycledServerStillOwned(context.Background(),
				&hcloud.Server{ID: 99, Labels: tt.labels})
			require.NoError(t, err)
			require.Equal(t, tt.wantOwned, owned)

			// Losing ownership must hand the machine to remediation, which is what actually returns the
			// server to the pool and stops this machine provisioning hardware it does not own.
			remediated := svc.scope.Machine.Annotations[clusterv1.RemediateMachineAnnotation]
			if tt.wantRemedy {
				require.Equal(t, "", remediated, "remediate-machine annotation must be present")
				require.Contains(t, svc.scope.Machine.Annotations, clusterv1.RemediateMachineAnnotation)
				require.Equal(t, infrav1.HCloudBootStateProvisioningFailed, svc.scope.HCloudMachine.Status.BootState)
				return
			}
			require.NotContains(t, svc.scope.Machine.Annotations, clusterv1.RemediateMachineAnnotation)
		})
	}
}

// Test_assertRecycledServerStillOwned_nilServer guards the callers that may hold no server at all.
func Test_assertRecycledServerStillOwned_nilServer(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}
	owned, err := svc.assertRecycledServerStillOwned(context.Background(), nil)
	require.NoError(t, err)
	require.True(t, owned)
}

// Test_serverMatchesRequest_placementGroup covers the sharpest edge of recycling: a rebuild does not
// move a server between placement groups, and hcloud offers no way to take one out. A control plane
// provisioned outside its spread group sits on the same physical host as its peers, so one host failure
// costs the etcd quorum — silently, because nothing else in the flow would notice.
func Test_serverMatchesRequest_placementGroup(t *testing.T) {
	group := &hcloud.PlacementGroup{ID: 7, Name: "cp-spread"}
	base := func(pg *hcloud.PlacementGroup) *hcloud.Server {
		return &hcloud.Server{
			ServerType:     &hcloud.ServerType{Name: recycleTestType},
			Location:       &hcloud.Location{Name: recycleTestLocation},
			PlacementGroup: pg,
		}
	}
	opts := func(pg *hcloud.PlacementGroup) hcloud.ServerCreateOpts {
		return hcloud.ServerCreateOpts{
			ServerType:     &hcloud.ServerType{Name: recycleTestType},
			Location:       &hcloud.Location{Name: recycleTestLocation},
			PlacementGroup: pg,
		}
	}
	tests := []struct {
		name   string
		server *hcloud.Server
		opts   hcloud.ServerCreateOpts
		want   bool
	}{
		{name: "group requested, server already in it", server: base(group), opts: opts(group), want: true},
		{name: "group requested, server in none: can be added before the rebuild", server: base(nil), opts: opts(group), want: true},
		{
			name:   "group requested, server in a DIFFERENT one: unusable, hcloud cannot move it out",
			server: base(&hcloud.PlacementGroup{ID: 99, Name: "other"}),
			opts:   opts(group),
			want:   false,
		},
		{name: "no group requested: an existing group constrains nothing the template asked for", server: base(group), opts: opts(nil), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, serverMatchesRequest(tt.server, tt.opts))
		})
	}
}

// Test_tryClaimRecyclableServer_addsServerToPlacementGroup verifies the repair half: a pool server in no
// group is put into the requested one before the rebuild, so a recycled control plane still ends up
// spread across physical hosts. hcloud applies this asynchronously and only to a stopped server, so the
// claim confirms the group before rebuilding.
func Test_tryClaimRecyclableServer_addsServerToPlacementGroup(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	group := &hcloud.PlacementGroup{ID: 7, Name: "cp-spread"}
	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
		infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
	}
	claimedNoGroup := &hcloud.Server{
		ID:         11,
		Status:     hcloud.ServerStatusOff,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels:     labels,
	}
	claimedInGroup := &hcloud.Server{
		ID:             11,
		Status:         hcloud.ServerStatusOff,
		ServerType:     &hcloud.ServerType{Name: recycleTestType},
		Location:       &hcloud.Location{Name: recycleTestLocation},
		Labels:         labels,
		PlacementGroup: group,
	}

	opts := recycleCreateOpts("worker-1")
	opts.PlacementGroup = group

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimedNoGroup}, nil).Once()
	// First read: the ownership check before the rebuild. Second read: confirming the placement group.
	mc.On("GetServer", mock.Anything, int64(11)).Return(claimedNoGroup, nil).Once()
	mc.On("AddServerToPlacementGroup", mock.Anything, mock.Anything, group).Return(nil).Once()
	mc.On("GetServer", mock.Anything, int64(11)).Return(claimedInGroup, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Return(hcloud.ServerRebuildResult{}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(claimedInGroup, nil).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed)
}

// Test_tryClaimRecyclableServer_waitsForShutdownBeforePlacementGroup covers the state a pool server is
// routinely in: returning a server issues an asynchronous ACPI shutdown and re-pools it straight away,
// so a quickly re-claimed server is still running — and hcloud refuses to move a running server into a
// placement group. The claim must ask it to stop and come back, not fail.
func Test_tryClaimRecyclableServer_waitsForShutdownBeforePlacementGroup(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	group := &hcloud.PlacementGroup{ID: 7, Name: "cp-spread"}
	stillRunning := &hcloud.Server{
		ID:         11,
		Status:     hcloud.ServerStatusRunning,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
			infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
		},
	}
	opts := recycleCreateOpts("worker-1")
	opts.PlacementGroup = group

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{stillRunning}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(11)).Return(stillRunning, nil).Once()
	mc.On("ShutdownServer", mock.Anything, mock.Anything).Return(nil).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.Nil(t, claimed)
	require.ErrorIs(t, err, errRecycleClaimPending, "the claim is kept and retried, not thrown away")
	// mc asserts on cleanup that neither AddServerToPlacementGroup nor RebuildServer ran on a running server.
}

// Test_tryClaimRecyclableServer_waitsForNetworkAttachment covers the same async trap as the placement
// group, one step later: hcloud attaches a server to a network asynchronously, and a server whose
// action is still running is locked and rejects the rebuild. Rebuilding on an unconfirmed attachment
// would also boot a node whose cloud-init never sees the private network.
func Test_tryClaimRecyclableServer_waitsForNetworkAttachment(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	claimed := &hcloud.Server{
		ID:         11,
		Status:     hcloud.ServerStatusOff,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
			infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
		},
	}
	opts := recycleCreateOpts("worker-1")
	opts.Networks = []*hcloud.Network{{ID: 4711}}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(11)).Return(claimed, nil).Once()
	mc.On("AttachServerToNetwork", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	// The confirming read still shows no private IP: the attach action has not landed.
	mc.On("GetServer", mock.Anything, int64(11)).Return(claimed, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimPending, "the claim is kept and retried, not thrown away")
	// mc asserts on cleanup that RebuildServer never ran on a server whose attachment was unconfirmed.
}

// Test_tryClaimRecyclableServer_rebuildsOnceAttached is the counterpart: with the private IP visible,
// the rebuild proceeds.
func Test_tryClaimRecyclableServer_rebuildsOnceAttached(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	labels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
		infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
	}
	claimed := &hcloud.Server{
		ID: 11, Status: hcloud.ServerStatusOff,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels:     labels,
	}
	attached := &hcloud.Server{
		ID: 11, Status: hcloud.ServerStatusOff,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels:     labels,
		PrivateNet: []hcloud.ServerPrivateNet{{Network: &hcloud.Network{ID: 4711}}},
	}
	opts := recycleCreateOpts("worker-1")
	opts.Networks = []*hcloud.Network{{ID: 4711}}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(11)).Return(claimed, nil).Once()
	mc.On("AttachServerToNetwork", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	mc.On("GetServer", mock.Anything, int64(11)).Return(attached, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Return(hcloud.ServerRebuildResult{}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(attached, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
}

// --- API failure paths -------------------------------------------------------------------------
//
// Recycling writes labels to hcloud in several steps, and a degraded API can interrupt it at any of
// them. What matters is not that each step succeeds, but that no interruption can leave a server in a
// state nothing recovers from: every abort has to leave either a claim (which the reaper ages out) or
// an available server. These tests execute those aborts instead of reasoning about them.

func recycleClaimedServer(t *testing.T) *hcloud.Server {
	t.Helper()
	return &hcloud.Server{
		ID:         21,
		Status:     hcloud.ServerStatusOff,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
			infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
		},
	}
}

func Test_tryClaimRecyclableServer_listFailurePropagates(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	mc.On("ListServers", mock.Anything, mock.Anything).Return(nil, errors.New("api down")).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Error(t, err)
	require.Nil(t, claimed)
	require.NotErrorIs(t, err, errRecycleClaimPending, "a failed list must not look like a written claim")
	// Nothing was written, so there is nothing to recover from.
}

func Test_tryClaimRecyclableServer_claimWriteFailureWritesNothingElse(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	candidate := &hcloud.Server{
		ID:         21,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
		},
	}
	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{candidate}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("api down")).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Error(t, err)
	require.Nil(t, claimed)
	// The claim may or may not have landed. Either way the next reconcile resolves it: if it landed,
	// pendingClaims finds it; if not, the server is still available. No requeue is promised here.
	require.NotErrorIs(t, err, errRecycleClaimPending)
}

func Test_completeClaim_verificationReadFailureKeepsTheClaim(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	// The ownership re-check before the rebuild fails...
	mc.On("GetServer", mock.Anything, int64(21)).Return(nil, errors.New("api down")).Once()
	// ...and so does the read that releaseReservation makes, so nothing is written at all.
	mc.On("GetServer", mock.Anything, int64(21)).Return(nil, errors.New("api down")).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Error(t, err)
	require.Nil(t, got)
	// The claim label survives untouched, so the next reconcile picks it up again — and if this machine
	// never comes back, the reaper ages it out. mc asserts no UpdateServer and no RebuildServer ran.
}

func Test_completeClaim_rebuildFailureReleasesTheClaim(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).
		Return(hcloud.ServerRebuildResult{}, errors.New("api down")).Once()
	// releaseReservation confirms we are still the claimant, then returns the server to the pool.
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	released := mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(claimed, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Error(t, err)
	require.Nil(t, got)
	require.NotNil(t, released, "the claim must be released so the server returns to the pool")
}

func Test_releaseReservation_updateFailureLeavesTheClaimForTheReaper(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)

	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("api down")).Once()

	// Best-effort by design: the failure is logged, not returned, and the stranded claim is the reaper's
	// problem rather than a server that silently leaves the pool for good.
	svc.releaseReservation(context.Background(), claimed)
}

func Test_reapAbandonedClaims_updateFailureIsSurvivable(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	stale := &hcloud.Server{
		ID: 22,
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleClaimantLabelKey:  "worker-dead",
			infrav1.ServerRecycleClaimedAtLabelKey: strconv.FormatInt(time.Now().Add(-2*recycleClaimTTL).Unix(), 10),
		},
	}
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("api down")).Once()

	reaped := svc.reapAbandonedClaims(context.Background(), []*hcloud.Server{stale})
	require.Zero(t, reaped, "a failed reap must not claim to have freed anything")
	// The next reconcile tries again; the claim stays visible until it succeeds.
}

func Test_returnServerToRecycling_shutdownFailureIsRetryable(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	server := &hcloud.Server{
		ID:     23,
		Status: hcloud.ServerStatusRunning,
		Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"},
	}
	mc.On("ShutdownServer", mock.Anything, mock.Anything).Return(errors.New("api down")).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.Error(t, err, "the error must surface so the deletion is retried")
	// Crucially the server keeps this machine's identity, so the next reconcile still finds it. mc
	// asserts that it was not re-pooled while still running.
}

// Test_completeClaim_releasesAClaimThatNoLongerFits covers a claim that outlives the request it was
// written for — the machine template changed between the two reconciles. The server must go back to the
// pool rather than be rebuilt into something the template no longer asks for.
func Test_completeClaim_releasesAClaimThatNoLongerFits(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)

	// The request now asks for a different server type than the claimed server has.
	opts := recycleCreateOpts("worker-1")
	opts.ServerType = &hcloud.ServerType{Name: "cpx52"}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(claimed, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err, "an outdated claim is released and provisioning falls through to a create")
	require.Nil(t, got)
	// mc asserts RebuildServer never ran on a server that no longer matches.
}

func Test_completeClaim_finalizeFailureReleasesTheClaim(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Return(hcloud.ServerRebuildResult{}, nil).Once()
	// Applying the machine identity fails, so the server has been rebuilt but is not owned by anyone.
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("api down")).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(claimed, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Error(t, err)
	require.Nil(t, got)
	// The server goes back to the pool. It carries this machine's bootstrap data until the next claim
	// rebuilds it, which is wasteful but not unsafe: no machine ever adopts it, because the identity
	// label was never applied.
}

func Test_rebuildRecyclableServer_rejectsNilImage(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	err := svc.rebuildRecyclableServer(context.Background(), &hcloud.Server{ID: 21}, recycleCreateOpts("worker-1"), nil, nil)
	require.ErrorContains(t, err, "image is nil")
}

// Test_tryClaimRecyclableServer_waitsForPlacementGroupToLand covers the asynchronous half: the server is
// off and the add is accepted, but the group is not visible yet. Rebuilding a server whose action is
// still running fails, so the claim must be kept and retried.
func Test_tryClaimRecyclableServer_waitsForPlacementGroupToLand(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)
	group := &hcloud.PlacementGroup{ID: 7, Name: "cp-spread"}

	opts := recycleCreateOpts("worker-1")
	opts.PlacementGroup = group

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("AddServerToPlacementGroup", mock.Anything, mock.Anything, group).Return(nil).Once()
	// Still not in the group on the confirming read.
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimPending)
	// mc asserts RebuildServer never ran while the action was still in flight.
}

func Test_finalizeReservation_handlesRequestWithoutLabels(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}
	server := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, false)

	opts := recycleCreateOpts("worker-1")
	opts.Labels = nil

	finalized, err := svc.finalizeReservation(context.Background(), server, opts)
	require.NoError(t, err)
	require.Equal(t, labelValueTrue, finalized.Labels[infrav1.ServerRecycleLabelKey],
		"the persistent recycle marker must survive even when the request carries no labels")
}

func Test_returnServerToRecycling_relabelFailureIsRetryable(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	server := &hcloud.Server{
		ID:     23,
		Status: hcloud.ServerStatusOff,
		Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"},
	}
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("api down")).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.Error(t, err, "the deletion must be retried rather than dropping the server out of the pool")
}

// Test_recycleMismatchReason checks that every rejection can explain itself. The explanation is what an
// operator sees when a prepared pool is ignored and a new server is billed instead, so a wrong or empty
// reason is as bad as a wrong decision.
func Test_recycleMismatchReason(t *testing.T) {
	group := &hcloud.PlacementGroup{ID: 7, Name: "cp-spread"}
	fits := &hcloud.Server{
		Name:       "pool-a",
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
	}
	tests := []struct {
		name    string
		server  *hcloud.Server
		opts    hcloud.ServerCreateOpts
		wantSub string
	}{
		{name: "matches", server: fits, opts: recycleCreateOpts("worker-1"), wantSub: ""},
		{
			name:    "wrong type",
			server:  &hcloud.Server{ServerType: &hcloud.ServerType{Name: "cx23"}, Location: fits.Location},
			opts:    recycleCreateOpts("worker-1"),
			wantSub: `server type is "cx23"`,
		},
		{
			name:    "no type at all",
			server:  &hcloud.Server{Location: fits.Location},
			opts:    recycleCreateOpts("worker-1"),
			wantSub: `server type is "<none>"`,
		},
		{
			name:    "wrong location",
			server:  &hcloud.Server{ServerType: fits.ServerType, Location: &hcloud.Location{Name: "hel1"}},
			opts:    recycleCreateOpts("worker-1"),
			wantSub: `location is "hel1"`,
		},
		{
			name: "in another placement group",
			server: &hcloud.Server{
				ServerType: fits.ServerType, Location: fits.Location,
				PlacementGroup: &hcloud.PlacementGroup{ID: 99, Name: "other"},
			},
			opts: func() hcloud.ServerCreateOpts {
				o := recycleCreateOpts("worker-1")
				o.PlacementGroup = group
				return o
			}(),
			wantSub: `already in placement group "other"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := recycleMismatchReason(tt.server, tt.opts)
			if tt.wantSub == "" {
				require.Empty(t, reason)
				require.True(t, serverMatchesRequest(tt.server, tt.opts), "decision and explanation must agree")
				return
			}
			require.Contains(t, reason, tt.wantSub)
			require.False(t, serverMatchesRequest(tt.server, tt.opts), "decision and explanation must agree")
		})
	}
}

// Test_tryClaimRecyclableServer_reportsWhyNothingMatched covers the operator-facing half: an unmatched
// pool must produce an explanation, not just a new server on the bill.
func Test_tryClaimRecyclableServer_reportsWhyNothingMatched(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	// Available and recyclable, but the wrong type.
	seedRecyclableServer(t, client, "pool-wrong", "cx23", recycleTestLocation, true)

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "an unmatched pool falls through to a normal create")

	// The mismatching server is left untouched — no claim written, still advertised as available — and
	// the reason it was skipped is the one recycleMismatchReason produces for it.
	available := listAvailableRecyclable(t, client)
	require.Len(t, available, 1)
	require.NotContains(t, available[0].Labels, infrav1.ServerRecycleClaimantLabelKey)
	require.Contains(t, recycleMismatchReason(available[0], recycleCreateOpts("worker-1")), "server type is")
}

func Test_capStrings(t *testing.T) {
	items := []string{"a", "b", "c", "d"}
	require.Equal(t, items, capStrings(items, 4))
	require.Equal(t, []string{"a", "b", "and 2 more"}, capStrings(items, 2))
	// The input must not be modified: it is reused by the caller for logging.
	require.Equal(t, []string{"a", "b", "c", "d"}, items)
}

// Test_claimAPICallBudget counts the hcloud calls one successful claim costs. Hetzner rate-limits per
// project, and this feature runs on every reconcile of every recycling machine, so the number is a
// property worth pinning: a change that quietly doubles it would degrade a whole cluster, not one
// machine. The count is asserted rather than merely logged so that adding a call has to be deliberate.
func Test_claimAPICallBudget(t *testing.T) {
	calls := map[string]int{}
	count := func(name string) func(mock.Arguments) {
		return func(mock.Arguments) { calls[name]++ }
	}

	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	available := &hcloud.Server{
		ID: 31, Status: hcloud.ServerStatusOff,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
		},
	}
	claimedLabels := map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
		infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
	}
	claimed := &hcloud.Server{
		ID: 31, Status: hcloud.ServerStatusOff,
		ServerType: available.ServerType, Location: available.Location, Labels: claimedLabels,
	}

	mc.On("ListServers", mock.Anything, mock.Anything).Run(count("ListServers")).Return([]*hcloud.Server{available}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Run(count("UpdateServer")).Return(claimed, nil).Once()

	// Reconcile 1: list the pool, write the claim, requeue.
	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimPending)

	mc.On("ListServers", mock.Anything, mock.Anything).Run(count("ListServers")).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(31)).Run(count("GetServer")).Return(claimed, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Run(count("RebuildServer")).
		Return(hcloud.ServerRebuildResult{}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Run(count("UpdateServer")).Return(claimed, nil).Once()

	// Reconcile 2: verify the claim, re-check ownership, rebuild, finalize.
	got, err = svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, got)

	total := 0
	for _, n := range calls {
		total += n
	}
	t.Logf("API calls for one claim (no network, no placement group): %v = %d total", calls, total)

	require.Equal(t, 2, calls["ListServers"], "one pool listing per reconcile")
	require.Equal(t, 2, calls["UpdateServer"], "one claim write, one finalize")
	require.Equal(t, 1, calls["GetServer"], "one ownership re-check before the destructive step")
	require.Equal(t, 1, calls["RebuildServer"])
	require.Equal(t, 6, total, "a claim costs six calls; raising this affects every recycling machine")
}
