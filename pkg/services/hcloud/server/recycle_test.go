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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
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
		name   string
		policy infrav1.ServerRecyclingPolicy
		want   bool
	}{
		{name: "unset", policy: "", want: false},
		{name: "disabled", policy: infrav1.ServerRecyclingDisabled, want: false},
		{name: "enabled", policy: infrav1.ServerRecyclingEnabled, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{scope: newRecycleScope(t, "m", fakehcloudclient.NewHCloudClientFactory().NewClient(""))}
			svc.scope.HCloudMachine.Spec.ServerRecycling = tt.policy
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

// Test_tryClaimRecyclableServer_claimsExactlyOneCandidate checks that a claim takes one of the
// matching servers and leaves the rest of the pool alone. Which one it takes is deliberately not
// asserted: the candidate is picked at random so that concurrent claims do not all target the same
// server.
func Test_tryClaimRecyclableServer_claimsExactlyOneCandidate(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	first := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)
	second := seedRecyclableServer(t, client, "pool-b", recycleTestType, recycleTestLocation, true)

	claimed, err := claimRecyclable(t, svc, recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.Contains(t, []int64{first.ID, second.ID}, claimed.ID)

	// Exactly the other server is untouched and still available.
	remaining := listAvailableRecyclable(t, client)
	require.Len(t, remaining, 1)
	require.NotEqual(t, claimed.ID, remaining[0].ID)
}

// Test_tryClaimRecyclableServer_losesClaimToLaterWriter is the race the two-reconcile split exists for:
// this machine writes a claim, another machine overwrites it before the verification runs, and the
// verification therefore reads the other machine's name. The loser must back off completely: no
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
	// mc fails the test if RebuildServer is called, or if an UpdateServer releases labels that now belong
	// to worker-2.
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

// Test_reapAbandonedClaims covers the failure mode the claim timestamp exists for: an HCloudMachine that
// goes away before completing its claim leaves a server that carries no available marker and is
// therefore invisible to every lookup. Without reaping, the pool would shrink by one server per abandoned
// claim.
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

	// A running server is powered off first and is not handed back yet.
	res, err := svc.returnServerToRecycling(context.Background(), claimed)
	require.NoError(t, err)
	require.Equal(t, recycleServerOffDelay, res.RequeueAfter)

	got, err := client.GetServer(context.Background(), claimed.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, hcloud.ServerStatusOff, got.Status)
	require.NotContains(t, got.Labels, infrav1.ServerRecycleAvailableLabelKey,
		"a server is not advertised before hcloud reports it as off")
	require.Equal(t, "worker-1", got.Labels[infrav1.MachineNameTagKey],
		"the identity stays until the server is off, so a retry still passes the ownership check")

	// The next reconcile reads it as off and returns it to the pool.
	res, err = svc.returnServerToRecycling(context.Background(), got)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)

	got, err = client.GetServer(context.Background(), claimed.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	// The machine-owned labels are stripped and the server is available again.
	require.Equal(t, "true", got.Labels[infrav1.ServerRecycleLabelKey])
	require.Equal(t, "true", got.Labels[infrav1.ServerRecycleAvailableLabelKey])
	require.NotContains(t, got.Labels, infrav1.MachineNameTagKey)
	require.Equal(t, hcloud.ServerStatusOff, got.Status)
}

// Test_returnServerToRecycling_detachesNetwork verifies that, when the cluster has a private network,
// returning a server also detaches it. A normal delete detaches implicitly via server deletion; since
// recycling keeps the server, an explicit detach is required so a parked server does not keep its private
// IP in the cluster's network. The fake client does not fill server.PrivateNet when it attaches a server, so a mock
// is used.
func Test_returnServerToRecycling_detachesNetwork(t *testing.T) {
	mc := mocks.NewClient(t)
	scp := newRecycleScope(t, "worker-1", mc)
	scp.HetznerCluster.Status.Network = &infrav1.NetworkStatus{ID: 4711}
	svc := &Service{scope: scp}

	server := &hcloud.Server{
		ID:         5,
		Status:     hcloud.ServerStatusRunning,
		Labels:     map[string]string{infrav1.MachineNameTagKey: "worker-1"},
		PrivateNet: []hcloud.ServerPrivateNet{{Network: &hcloud.Network{ID: 4711}}},
	}
	mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(nil).Once()
	mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(server, nil).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)

	// The detach happens on the reconcile that finds the server off.
	server.Status = hcloud.ServerStatusOff
	_, err = svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	// mc asserts on cleanup that PowerOffServer, DetachServerFromNetwork and UpdateServer were all called.
}

// Test_returnServerToRecycling_skipsDetachOfADetachedServer covers a retry after the detach went through
// but the relabel failed. The server is no longer attached, and hcloud answers a detach of a server that
// is not attached with a plain service_error, so the retry must not send one.
func Test_returnServerToRecycling_skipsDetachOfADetachedServer(t *testing.T) {
	mc := mocks.NewClient(t)
	scp := newRecycleScope(t, "worker-1", mc)
	scp.HetznerCluster.Status.Network = &infrav1.NetworkStatus{ID: 4711}
	svc := &Service{scope: scp}

	server := &hcloud.Server{
		ID:     5,
		Status: hcloud.ServerStatusOff,
		Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"},
	}
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(server, nil).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	// mc fails the test on cleanup if DetachServerFromNetwork was called.
}

// newRecycleScope builds a Service scope for a worker HCloudMachine with recycling enabled, wired to
// the given hcloud client.
func newRecycleScope(t *testing.T, machineName string, client hcloudclient.Client) *scope.MachineScope {
	t.Helper()
	hcloudMachine := &infrav1.HCloudMachine{
		ObjectMeta: metav1.ObjectMeta{Name: machineName, Namespace: "default"},
		Spec: infrav1.HCloudMachineSpec{
			Type:            recycleTestType,
			ImageName:       "snapshot",
			ServerRecycling: infrav1.ServerRecyclingEnabled,
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
// guard. findServer returns the server the ProviderID points to without checking its labels, so a
// machine that lost its server to another claimant still finds it on deletion, and would power off and
// re-pool hardware the new owner is running on.
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
	require.NotEqual(t, hcloud.ServerStatusOff, got.Status, "the new owner's server must not be powered off")
}

// Test_assertRecycledServerStillOwned covers the gap that neither the claim verification nor the
// pre-rebuild check can see: findServer returns the server the ProviderID points to without checking its
// labels, so after finalize a machine would keep provisioning a server that another machine has taken
// over.
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
				svc.scope.HCloudMachine.Spec.ServerRecycling = infrav1.ServerRecyclingDisabled
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

// Test_serverMatchesRequest_placementGroup checks that placement groups do not affect matching: a server
// in no group or in another group still matches, because the claim moves it into the requested group
// before the rebuild.
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
			name:   "group requested, server in a different one: can be moved before the rebuild",
			server: base(&hcloud.PlacementGroup{ID: 99, Name: "other"}),
			opts:   opts(group),
			want:   true,
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

// Test_tryClaimRecyclableServer_waitsForPowerOffBeforePlacementGroup covers a pool server that is still
// running, for example one an operator labelled by hand. hcloud refuses to move a running server into a
// placement group, so the claim must power it off and come back, not fail.
func Test_tryClaimRecyclableServer_waitsForPowerOffBeforePlacementGroup(t *testing.T) {
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
	mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(nil).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.Nil(t, claimed)
	require.ErrorIs(t, err, errRecycleClaimPending, "the claim is kept and retried, not thrown away")
	// mc fails the test if AddServerToPlacementGroup or RebuildServer is called on the running server.
}

// Test_tryClaimRecyclableServer_leavesAForeignPlacementGroup covers a candidate that still carries the
// placement group of the cluster that used it before. hcloud rejects adding a server that is already in
// a group, so the claim takes it out first and picks the claim up again on the next reconcile.
func Test_tryClaimRecyclableServer_leavesAForeignPlacementGroup(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	inForeignGroup := &hcloud.Server{
		ID:             11,
		Status:         hcloud.ServerStatusOff,
		ServerType:     &hcloud.ServerType{Name: recycleTestType},
		Location:       &hcloud.Location{Name: recycleTestLocation},
		PlacementGroup: &hcloud.PlacementGroup{ID: 99, Name: "other"},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
			infrav1.ServerRecycleClaimedAtLabelKey: "1700000000",
		},
	}
	opts := recycleCreateOpts("worker-1")
	opts.PlacementGroup = &hcloud.PlacementGroup{ID: 7, Name: "cp-spread"}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{inForeignGroup}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(11)).Return(inForeignGroup, nil).Once()
	mc.On("RemoveServerFromPlacementGroup", mock.Anything, mock.Anything).Return(nil).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.Nil(t, claimed)
	require.ErrorIs(t, err, errRecycleClaimPending, "the claim is kept and retried, not thrown away")
	// mc fails the test if the server is added to the new group or rebuilt in the same reconcile that
	// asked hcloud to take it out of the old one.
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
	// mc fails the test if RebuildServer is called on a server whose attachment is unconfirmed.
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
	// The claim label survives untouched, so the next reconcile picks it up again; if this machine goes
	// away instead, the reaper ages the claim out. mc fails the test if UpdateServer or RebuildServer is
	// called.
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
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(claimed, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Error(t, err)
	require.Nil(t, got)
	// mc asserts on cleanup that the claim was released, so the server returns to the pool.
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
		ID:     22,
		Status: hcloud.ServerStatusOff,
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

// Test_reapAbandonedClaims_powersOffARunningServer covers a claim that was abandoned after it had started
// the server, for example by joining a placement group. The server must not keep running in the pool.
func Test_reapAbandonedClaims_powersOffARunningServer(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	stale := &hcloud.Server{
		ID:     22,
		Status: hcloud.ServerStatusRunning,
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleClaimantLabelKey:  "worker-dead",
			infrav1.ServerRecycleClaimedAtLabelKey: strconv.FormatInt(time.Now().Add(-2*recycleClaimTTL).Unix(), 10),
		},
	}
	mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(stale, nil).Once()

	require.Equal(t, 1, svc.reapAbandonedClaims(context.Background(), []*hcloud.Server{stale}))
}

// Test_releaseReservation_powersOffARunningServer covers a claim given up after it had started the server.
func Test_releaseReservation_powersOffARunningServer(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)
	claimed.Status = hcloud.ServerStatusRunning

	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(claimed, nil).Once()

	svc.releaseReservation(context.Background(), claimed)
	// mc asserts on cleanup that the server was powered off and handed back.
}

func Test_returnServerToRecycling_powerOffFailureIsRetryable(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	server := &hcloud.Server{
		ID:     23,
		Status: hcloud.ServerStatusRunning,
		Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"},
	}
	mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(errors.New("api down")).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.Error(t, err, "the error must surface so the deletion is retried")
	// The server keeps this machine's identity, so the retry still passes the ownership check. mc fails
	// the test if the server is re-pooled while it is still running.
}

// Test_completeClaim_releasesAClaimThatNoLongerFits covers a claim that outlives the request it was
// written for, for example because the server's type was changed meanwhile. The server must go back to
// the pool rather than be rebuilt into something the request does not ask for.
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

// Test_completeClaim_finalizeFailureKeepsTheClaim covers a finalize that fails after the rebuild. hcloud
// starts every server it rebuilds, so the server already boots this machine's node; handing it back to
// the pool would leave that node running without a Machine.
func Test_completeClaim_finalizeFailureKeepsTheClaim(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Return(hcloud.ServerRebuildResult{}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("api down")).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Error(t, err)
	require.Nil(t, got)
	require.NotErrorIs(t, err, errRecycleClaimFailed, "the claim is not given up")
	// mc fails the test if the claim is released: that would need another GetServer and UpdateServer.
}

// Test_completeClaim_finishesAFinalizeWithoutRebuildingAgain covers the reconcile after a failed finalize.
// The server was rebuilt by this claim and is booting this machine's node, so the claim only applies the
// identity; a second rebuild would wipe the node while it joins.
func Test_completeClaim_finishesAFinalizeWithoutRebuildingAgain(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)
	claimed.Status = hcloud.ServerStatusRunning
	// By now the node can have a volume of its own attached, which has to stay.
	claimed.Volumes = []*hcloud.Volume{{ID: 3}}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("ListServerActions", mock.Anything, mock.Anything, mock.Anything).Return([]*hcloud.Action{
		{ID: 9, Command: rebuildServerCommand, Status: hcloud.ActionStatusSuccess, Started: time.Unix(1700000005, 0)},
	}, nil).Once()
	finalized := &hcloud.Server{ID: 21, Name: "worker-1", Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"}}
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(finalized, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	// mc fails the test if RebuildServer or DetachVolume is called.
}

func Test_rebuiltByThisClaim(t *testing.T) {
	claimedAt := int64(1700000000)
	tests := []struct {
		name    string
		actions []*hcloud.Action
		want    bool
	}{
		{name: "no actions", want: false},
		{
			name:    "rebuild of an earlier claim",
			actions: []*hcloud.Action{{Command: rebuildServerCommand, Status: hcloud.ActionStatusSuccess, Started: time.Unix(claimedAt-60, 0)}},
			want:    false,
		},
		{
			name:    "failed rebuild of this claim",
			actions: []*hcloud.Action{{Command: rebuildServerCommand, Status: hcloud.ActionStatusError, Started: time.Unix(claimedAt+5, 0)}},
			want:    false,
		},
		{
			name:    "other action of this claim",
			actions: []*hcloud.Action{{Command: "add_to_placement_group", Status: hcloud.ActionStatusSuccess, Started: time.Unix(claimedAt+5, 0)}},
			want:    false,
		},
		{
			name: "rebuild of this claim",
			actions: []*hcloud.Action{
				{Command: "start_server", Status: hcloud.ActionStatusSuccess, Started: time.Unix(claimedAt+30, 0)},
				{Command: rebuildServerCommand, Status: hcloud.ActionStatusRunning, Started: time.Unix(claimedAt+5, 0)},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := mocks.NewClient(t)
			svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
			claimed := recycleClaimedServer(t)
			claimed.Labels[infrav1.ServerRecycleClaimedAtLabelKey] = strconv.FormatInt(claimedAt, 10)
			mc.On("ListServerActions", mock.Anything, mock.Anything, mock.Anything).Return(tt.actions, nil).Once()

			got, err := svc.rebuiltByThisClaim(context.Background(), claimed)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	t.Run("claim without a timestamp", func(t *testing.T) {
		mc := mocks.NewClient(t)
		svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
		claimed := recycleClaimedServer(t)
		delete(claimed.Labels, infrav1.ServerRecycleClaimedAtLabelKey)

		got, err := svc.rebuiltByThisClaim(context.Background(), claimed)
		require.NoError(t, err)
		require.False(t, got, "without a timestamp nothing can be attributed to this claim")
		// mc fails the test if the action log is read.
	})
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
			// The claim moves the server out of the group it carries, so this is not a mismatch.
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
			wantSub: "",
		},
		{
			// hcloud refuses to rebuild such a server, so every claim on it would fail.
			name: "rebuild protection",
			server: &hcloud.Server{
				ServerType: fits.ServerType, Location: fits.Location,
				Protection: hcloud.ServerProtection{Rebuild: true},
			},
			opts:    recycleCreateOpts("worker-1"),
			wantSub: "rebuild protection",
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
	scp := newRecycleScope(t, "worker-1", client)
	recorder := record.NewFakeRecorder(10)
	scp.EventRecorder = recorder
	svc := &Service{scope: scp}

	// Available and recyclable, but the wrong type.
	seedRecyclableServer(t, client, "pool-wrong", "cx23", recycleTestLocation, true)

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "an unmatched pool falls through to a normal create")

	// The machine records why the server was skipped.
	require.Len(t, recorder.Events, 1)
	event := <-recorder.Events
	require.Contains(t, event, "NoMatchingRecyclableServer")
	require.Contains(t, event, `server type is "cx23"`)

	// The mismatching server is left untouched: no claim written, still advertised as available.
	available := listAvailableRecyclable(t, client)
	require.Len(t, available, 1)
	require.NotContains(t, available[0].Labels, infrav1.ServerRecycleClaimantLabelKey)
}

func Test_capStrings(t *testing.T) {
	items := []string{"a", "b", "c", "d"}
	require.Equal(t, items, capStrings(items, 4))
	require.Equal(t, []string{"a", "b", "and 2 more"}, capStrings(items, 2))
	// The input must not be modified.
	require.Equal(t, []string{"a", "b", "c", "d"}, items)
}

// Test_claimAPICallBudget counts the hcloud calls one successful claim costs inside the recycling code.
// Hetzner rate-limits per project, and every recycling machine runs this until it has a server, so the
// number is a property worth pinning: a change that quietly doubles it would degrade a whole cluster, not
// one machine. The count is asserted rather than merely logged so that adding a call has to be
// deliberate. The lookups every reconcile without a server makes before it, such as the image, are not
// counted here.
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

// seedFinalizedServer creates a server that a finalize for machineName has already applied its identity
// to: the persistent recycle marker and the machine labels, but no claim and no available marker.
func seedFinalizedServer(t *testing.T, client hcloudclient.Client, machineName string) *hcloud.Server {
	t.Helper()
	res, err := client.CreateServer(context.Background(), hcloud.ServerCreateOpts{
		Name: machineName,
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey: labelValueTrue,
			infrav1.MachineNameTagKey:     machineName,
		},
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
	})
	require.NoError(t, err)
	return res.Server
}

// Test_tryClaimRecyclableServer_adoptsItsFinalizedServer covers a finalize that reached hcloud while the
// ProviderID never reached the HCloudMachine. The server already carries the machine's identity and no
// claim, so the next attempt has to take it instead of claiming a second server.
func Test_tryClaimRecyclableServer_adoptsItsFinalizedServer(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	finalized := seedFinalizedServer(t, client, "worker-1")
	spare := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, finalized.ID, got.ID, "the server finalized for this machine is adopted")

	spareNow, err := client.GetServer(context.Background(), spare.ID)
	require.NoError(t, err)
	require.Equal(t, labelValueTrue, spareNow.Labels[infrav1.ServerRecycleAvailableLabelKey], "no second server is claimed")
	require.NotContains(t, spareNow.Labels, infrav1.ServerRecycleClaimantLabelKey)
}

// Test_tryClaimRecyclableServer_adoptionReleasesLeftoverClaims checks that a claim the machine still holds
// goes back to the pool when it adopts its finalized server, instead of waiting for the reaper.
func Test_tryClaimRecyclableServer_adoptionReleasesLeftoverClaims(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	finalized := seedFinalizedServer(t, client, "worker-1")
	leftover := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, false)
	_, err := client.UpdateServer(context.Background(), leftover, hcloud.ServerUpdateOpts{Labels: map[string]string{
		infrav1.ServerRecycleLabelKey:          labelValueTrue,
		infrav1.ServerRecycleClaimantLabelKey:  "worker-1",
		infrav1.ServerRecycleClaimedAtLabelKey: strconv.FormatInt(time.Now().Unix(), 10),
	}})
	require.NoError(t, err)

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, finalized.ID, got.ID)

	leftoverNow, err := client.GetServer(context.Background(), leftover.ID)
	require.NoError(t, err)
	require.Equal(t, labelValueTrue, leftoverNow.Labels[infrav1.ServerRecycleAvailableLabelKey])
	require.NotContains(t, leftoverNow.Labels, infrav1.ServerRecycleClaimantLabelKey)
	require.Nil(t, leftoverNow.Image, "the leftover claim was never rebuilt")
}

// Test_tryClaimRecyclableServer_doesNotAdoptAnotherMachinesServer makes sure the adoption only takes a
// server with this machine's identity.
func Test_tryClaimRecyclableServer_doesNotAdoptAnotherMachinesServer(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	foreign := seedFinalizedServer(t, client, "worker-2")
	pool := seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimPending, "an available server is claimed as usual")

	poolNow, err := client.GetServer(context.Background(), pool.ID)
	require.NoError(t, err)
	require.Equal(t, "worker-1", poolNow.Labels[infrav1.ServerRecycleClaimantLabelKey])

	foreignNow, err := client.GetServer(context.Background(), foreign.ID)
	require.NoError(t, err)
	require.Equal(t, "worker-2", foreignNow.Labels[infrav1.MachineNameTagKey], "the other machine's server is left alone")
	require.NotContains(t, foreignNow.Labels, infrav1.ServerRecycleClaimantLabelKey)
}

// Test_completeClaim_failuresInTheRebuildStep makes every hcloud call of the rebuild step fail once. A
// locked server and a rate limit keep the claim: the server is still this machine's, and the call works
// later, for example once the power off sent on the previous reconcile has finished. Any other failure
// gives the claim up and hands the server back to the pool.
func Test_completeClaim_failuresInTheRebuildStep(t *testing.T) {
	group := &hcloud.PlacementGroup{ID: 7}
	network := &hcloud.Network{ID: 4711}

	// Each step prepares a claim and a request that lead straight to one hcloud call, lets the calls
	// before it succeed and makes that call fail with fail.
	type setupFunc func(mc *mocks.Client, claimed *hcloud.Server, opts *hcloud.ServerCreateOpts, fail error)
	steps := []struct {
		name  string
		setup setupFunc
	}{
		{"ownership check", func(mc *mocks.Client, claimed *hcloud.Server, _ *hcloud.ServerCreateOpts, fail error) {
			mc.On("GetServer", mock.Anything, claimed.ID).Return(nil, fail).Once()
		}},
		{"leaving another placement group", func(mc *mocks.Client, claimed *hcloud.Server, opts *hcloud.ServerCreateOpts, fail error) {
			claimed.PlacementGroup = &hcloud.PlacementGroup{ID: 8}
			opts.PlacementGroup = group
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("RemoveServerFromPlacementGroup", mock.Anything, mock.Anything).Return(fail).Once()
		}},
		{"power off", func(mc *mocks.Client, claimed *hcloud.Server, opts *hcloud.ServerCreateOpts, fail error) {
			claimed.Status = hcloud.ServerStatusRunning
			opts.PlacementGroup = group
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(fail).Once()
		}},
		{"joining the placement group", func(mc *mocks.Client, claimed *hcloud.Server, opts *hcloud.ServerCreateOpts, fail error) {
			opts.PlacementGroup = group
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("AddServerToPlacementGroup", mock.Anything, mock.Anything, group).Return(fail).Once()
		}},
		{"confirming the placement group", func(mc *mocks.Client, claimed *hcloud.Server, opts *hcloud.ServerCreateOpts, fail error) {
			opts.PlacementGroup = group
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("AddServerToPlacementGroup", mock.Anything, mock.Anything, group).Return(nil).Once()
			mc.On("GetServer", mock.Anything, claimed.ID).Return(nil, fail).Once()
		}},
		{"leaving a network it does not use", func(mc *mocks.Client, claimed *hcloud.Server, _ *hcloud.ServerCreateOpts, fail error) {
			claimed.PrivateNet = []hcloud.ServerPrivateNet{{Network: &hcloud.Network{ID: 999}}}
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, mock.Anything).Return(fail).Once()
		}},
		{"attaching the network", func(mc *mocks.Client, claimed *hcloud.Server, opts *hcloud.ServerCreateOpts, fail error) {
			opts.Networks = []*hcloud.Network{network}
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("AttachServerToNetwork", mock.Anything, mock.Anything, mock.Anything).Return(fail).Once()
		}},
		{"confirming the network", func(mc *mocks.Client, claimed *hcloud.Server, opts *hcloud.ServerCreateOpts, fail error) {
			opts.Networks = []*hcloud.Network{network}
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("AttachServerToNetwork", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			mc.On("GetServer", mock.Anything, claimed.ID).Return(nil, fail).Once()
		}},
		{"detaching a volume", func(mc *mocks.Client, claimed *hcloud.Server, _ *hcloud.ServerCreateOpts, fail error) {
			claimed.Volumes = []*hcloud.Volume{{ID: 3}}
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("DetachVolume", mock.Anything, mock.Anything).Return(fail).Once()
		}},
		{"rebuild", func(mc *mocks.Client, claimed *hcloud.Server, _ *hcloud.ServerCreateOpts, fail error) {
			mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
			mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Return(hcloud.ServerRebuildResult{}, fail).Once()
		}},
	}

	failures := []struct {
		name     string
		err      error
		releases bool
		check    func(t *testing.T, svc *Service, err error)
	}{
		{
			name: "locked",
			err:  hcloud.Error{Code: hcloud.ErrorCodeLocked, Message: "server is locked"},
			check: func(t *testing.T, _ *Service, err error) {
				t.Helper()
				require.ErrorIs(t, err, errRecycleClaimPending, "the claim is kept and picked up on the next reconcile")
				require.ErrorIs(t, err, errRecycleClaimWaiting, "the next look waits for hcloud instead of polling every second")
			},
		},
		{
			name: "rate limit",
			err:  hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded, Message: "limit reached"},
			check: func(t *testing.T, svc *Service, err error) {
				t.Helper()
				require.True(t, hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded))
				require.NotErrorIs(t, err, errRecycleClaimPending, "the wait is the controller's rate-limit pause, not the short claim requeue")
				require.NotErrorIs(t, err, errRecycleClaimFailed, "the claim is kept")
				require.True(t, conditions.IsTrue(svc.scope.HCloudMachine, infrav1.HCloudRateLimitExceededCondition),
					"the machine is marked, so the controller waits out the rate limit")
			},
		},
		{
			name:     "other error",
			err:      hcloud.Error{Code: hcloud.ErrorCodeServerError, Message: "internal error"},
			releases: true,
			check: func(t *testing.T, _ *Service, err error) {
				t.Helper()
				require.ErrorIs(t, err, errRecycleClaimFailed, "the claim is given up, and the machine waits before it claims again")
			},
		},
	}

	releasesToPool := mock.MatchedBy(func(o hcloud.ServerUpdateOpts) bool {
		return o.Labels[infrav1.ServerRecycleAvailableLabelKey] == labelValueTrue &&
			o.Labels[infrav1.ServerRecycleClaimantLabelKey] == ""
	})

	for _, step := range steps {
		for _, failure := range failures {
			t.Run(step.name+"/"+failure.name, func(t *testing.T) {
				mc := mocks.NewClient(t)
				svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
				claimed := recycleClaimedServer(t)
				opts := recycleCreateOpts("worker-1")
				step.setup(mc, claimed, &opts, failure.err)
				if failure.releases {
					// The release checks the claim once more, powers a running server off and then hands
					// the server back.
					mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
					if claimed.Status != hcloud.ServerStatusOff {
						mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(nil).Once()
					}
					mc.On("UpdateServer", mock.Anything, mock.Anything, releasesToPool).Return(claimed, nil).Once()
				}

				got, err := svc.completeClaim(context.Background(), claimed, opts, &hcloud.Image{ID: 42}, nil)
				require.Nil(t, got)
				failure.check(t, svc, err)
				// mc asserts on cleanup that the server was handed back when the failure gives the claim
				// up, and fails the test if it is handed back otherwise or if a later step runs.
			})
		}
	}
}

// Test_tryClaimRecyclableServer_detachesNetworksItDoesNotUse covers a server that went back to the pool
// still attached to another cluster's network, which happens when a claim fails after the attach. Rebuild
// keeps attachments, so without the detach the node would get a private IP in both networks.
func Test_tryClaimRecyclableServer_detachesNetworksItDoesNotUse(t *testing.T) {
	detaches := func(id int64) any {
		return mock.MatchedBy(func(o hcloud.ServerDetachFromNetworkOpts) bool { return o.Network != nil && o.Network.ID == id })
	}

	t.Run("keeps the requested network", func(t *testing.T) {
		mc := mocks.NewClient(t)
		svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
		claimed := recycleClaimedServer(t)
		claimed.PrivateNet = []hcloud.ServerPrivateNet{
			{Network: &hcloud.Network{ID: 4711}},
			{Network: &hcloud.Network{ID: 999}},
		}
		opts := recycleCreateOpts("worker-1")
		opts.Networks = []*hcloud.Network{{ID: 4711}}

		mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
		mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
		mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, detaches(999)).Return(nil).Once()

		got, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
		require.Nil(t, got)
		require.ErrorIs(t, err, errRecycleClaimPending, "the claim is kept until the detach has landed")
		// mc fails the test if network 4711 is detached, or if anything is attached or rebuilt in the
		// same reconcile.
	})

	t.Run("cluster without a private network", func(t *testing.T) {
		mc := mocks.NewClient(t)
		svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
		claimed := recycleClaimedServer(t)
		claimed.PrivateNet = []hcloud.ServerPrivateNet{{Network: &hcloud.Network{ID: 999}}}

		mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
		mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
		mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, detaches(999)).Return(nil).Once()

		got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
		require.Nil(t, got)
		require.ErrorIs(t, err, errRecycleClaimPending)
	})
}

// Test_recyclingEventsReachTheScopeRecorder pins that recycling records its events through the scope's
// recorder. The package-level recorder of sigs.k8s.io/cluster-api/util/record drops every event unless
// InitFromRecorder has been called, and CAPH does not call it.
func Test_recyclingEventsReachTheScopeRecorder(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	scp := newRecycleScope(t, "worker-1", client)
	recorder := record.NewFakeRecorder(10)
	scp.EventRecorder = recorder
	svc := &Service{scope: scp}

	seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)
	claimed, err := claimRecyclable(t, svc, recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed)

	var events []string
	for len(recorder.Events) > 0 {
		events = append(events, <-recorder.Events)
	}
	require.True(t, strings.Contains(strings.Join(events, "\n"), "Normal RecycledServer"),
		"a claim must record a RecycledServer event on the HCloudMachine, got %q", events)
}

// Test_tryClaimRecyclableServer_detachStillRunningKeepsTheClaim covers what hcloud does when the next
// reconcile asks again for a detach that is still running: the listing still shows the network, and the
// answer is resource_already_detaching rather than locked. Seen on the real API.
func Test_tryClaimRecyclableServer_detachStillRunningKeepsTheClaim(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)
	claimed.PrivateNet = []hcloud.ServerPrivateNet{{Network: &hcloud.Network{ID: 999}}}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, mock.Anything).
		Return(hcloud.Error{Code: errorCodeResourceAlreadyDetaching, Message: "resource is already detaching from network"}).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimWaiting, "the claim is kept until the detach has landed")
	// mc fails the test if UpdateServer is called, which would release the claim.
}

// Test_returnServerToRecycling_detachStillRunning covers a return that is retried while the detach of the
// earlier attempt still runs. The server is on its way out of the network, so the relabel goes ahead.
func Test_returnServerToRecycling_detachStillRunning(t *testing.T) {
	mc := mocks.NewClient(t)
	scp := newRecycleScope(t, "worker-1", mc)
	scp.HetznerCluster.Status.Network = &infrav1.NetworkStatus{ID: 4711}
	svc := &Service{scope: scp}

	server := &hcloud.Server{
		ID:         5,
		Status:     hcloud.ServerStatusOff,
		Labels:     map[string]string{infrav1.MachineNameTagKey: "worker-1"},
		PrivateNet: []hcloud.ServerPrivateNet{{Network: &hcloud.Network{ID: 4711}}},
	}
	mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, mock.Anything).
		Return(hcloud.Error{Code: errorCodeResourceAlreadyDetaching, Message: "resource is already detaching from network"}).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(server, nil).Once()

	res, err := svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
}

func Test_claimRequeueDelay(t *testing.T) {
	require.Equal(t, recycleClaimVerifyDelay, claimRequeueDelay(fmt.Errorf("claimed server 1: %w", errRecycleClaimPending)),
		"a claim that was just written is verified after the short delay")
	require.Equal(t, recycleClaimWaitDelay, claimRequeueDelay(fmt.Errorf("claimed server 1: %w", errRecycleClaimWaiting)),
		"a claim that waits for hcloud looks again after the longer delay")
}

// Test_tryClaimRecyclableServer_joinSentAgainWhileItLands covers a listing taken while an earlier join was
// still running: it shows no group, so the join is sent again, and hcloud answers already_in_placement_group.
// Seen on the real API. The confirmation decides, and the claim goes on.
func Test_tryClaimRecyclableServer_joinSentAgainWhileItLands(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	group := &hcloud.PlacementGroup{ID: 7}
	claimed := recycleClaimedServer(t)
	opts := recycleCreateOpts("worker-1")
	opts.PlacementGroup = group
	joined := recycleClaimedServer(t)
	joined.PlacementGroup = group

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("AddServerToPlacementGroup", mock.Anything, mock.Anything, group).
		Return(hcloud.Error{Code: errorCodeAlreadyInPlacementGroup, Message: "server is already part of a placement group"}).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(joined, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Return(hcloud.ServerRebuildResult{}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(joined, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
}

// Test_tryClaimRecyclableServer_changesNetworksBeforeThePlacementGroup pins the order of the preparation.
// hcloud starts a server when it joins a placement group, and the server then boots its old disk, so the
// join comes last, right before the rebuild, and nothing else is changed in between.
func Test_tryClaimRecyclableServer_changesNetworksBeforeThePlacementGroup(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)
	claimed.PlacementGroup = &hcloud.PlacementGroup{ID: 8}
	claimed.PrivateNet = []hcloud.ServerPrivateNet{{Network: &hcloud.Network{ID: 999}}}
	opts := recycleCreateOpts("worker-1")
	opts.PlacementGroup = &hcloud.PlacementGroup{ID: 7}
	opts.Networks = []*hcloud.Network{{ID: 4711}}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, int64(21)).Return(claimed, nil).Once()
	mc.On("DetachServerFromNetwork", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimWaiting)
	// mc fails the test if the placement group is touched before the networks are done.
}

// Test_returnServerToRecycling_waitsForAPowerOffInProgress covers the reconcile after the power off: hcloud
// takes about ten seconds, the server is stopping meanwhile, and another power off would be answered with
// locked. Seen on the real API. The return only looks again later.
func Test_returnServerToRecycling_waitsForAPowerOffInProgress(t *testing.T) {
	t.Run("stopping", func(t *testing.T) {
		mc := mocks.NewClient(t)
		svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
		server := &hcloud.Server{ID: 5, Status: hcloud.ServerStatusStopping, Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"}}

		res, err := svc.returnServerToRecycling(context.Background(), server)
		require.NoError(t, err)
		require.Equal(t, recycleServerOffDelay, res.RequeueAfter)
		// mc fails the test if the power off is sent again or the server is relabelled.
	})

	t.Run("locked", func(t *testing.T) {
		mc := mocks.NewClient(t)
		svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
		server := &hcloud.Server{ID: 5, Status: hcloud.ServerStatusRunning, Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"}}
		mc.On("PowerOffServer", mock.Anything, mock.Anything).Return(hcloud.Error{Code: hcloud.ErrorCodeLocked, Message: "locked"}).Once()

		res, err := svc.returnServerToRecycling(context.Background(), server)
		require.NoError(t, err, "a power off that is still running is not an error")
		require.Equal(t, recycleServerOffDelay, res.RequeueAfter)
	})
}

// Test_returnServerToRecycling_alreadyReturned covers a delete that is retried after the server was
// returned and only removing the finalizer failed. Nothing is left to do, and nothing is wrong.
func Test_returnServerToRecycling_alreadyReturned(t *testing.T) {
	mc := mocks.NewClient(t)
	scp := newRecycleScope(t, "worker-1", mc)
	recorder := record.NewFakeRecorder(10)
	scp.EventRecorder = recorder
	svc := &Service{scope: scp}
	server := &hcloud.Server{
		ID:     5,
		Status: hcloud.ServerStatusOff,
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          labelValueTrue,
			infrav1.ServerRecycleAvailableLabelKey: labelValueTrue,
		},
	}

	res, err := svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Empty(t, recorder.Events, "no warning about a foreign owner")
	// mc fails the test if the server is touched.
}

// Test_tryClaimRecyclableServer_detachesVolumesBeforeTheRebuild covers a pool server that still has
// volumes attached. A rebuild keeps them, so the new node could mount them and read another cluster's
// data. The claim detaches them and rebuilds only once hcloud lists none.
func Test_tryClaimRecyclableServer_detachesVolumesBeforeTheRebuild(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)
	claimed.Volumes = []*hcloud.Volume{{ID: 3}, {ID: 4}}

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 3}).Return(nil).Once()
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 4}).Return(nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimWaiting, "the claim is kept and looks again once hcloud has detached the volumes")

	// On the next look hcloud is still detaching one of them, and answers its detach with locked.
	claimed.Volumes = []*hcloud.Volume{{ID: 4}}
	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 4}).
		Return(hcloud.Error{Code: hcloud.ErrorCodeLocked, Message: "cannot perform operation because resource is locked"}).Once()

	got, err = svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimWaiting)

	// Once hcloud lists no volume, the server is rebuilt.
	claimed.Volumes = nil
	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
	mc.On("RebuildServer", mock.Anything, mock.Anything, mock.Anything).Return(hcloud.ServerRebuildResult{}, nil).Once()
	finalized := &hcloud.Server{ID: claimed.ID, Name: "worker-1", Labels: map[string]string{infrav1.MachineNameTagKey: "worker-1"}}
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(finalized, nil).Once()

	got, err = svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	// mc fails the test if RebuildServer is called while a volume is still listed, because it only
	// expects the rebuild on the last pass.
}

// Test_returnServerToRecycling_detachesVolumes covers a server whose volumes are still attached when its
// machine is deleted, which happens when the cluster is deleted: CAPI does not wait for volumes to be
// detached then. The server is marked available only once hcloud lists no volume on it.
func Test_returnServerToRecycling_detachesVolumes(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	server := &hcloud.Server{
		ID:      5,
		Status:  hcloud.ServerStatusOff,
		Labels:  map[string]string{infrav1.MachineNameTagKey: "worker-1"},
		Volumes: []*hcloud.Volume{{ID: 3}, {ID: 4}},
	}
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 3}).Return(nil).Once()
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 4}).Return(nil).Once()

	res, err := svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	require.Equal(t, recycleVolumeDetachDelay, res.RequeueAfter)

	// On the next look hcloud is still detaching one of them, and answers its detach with locked.
	server.Volumes = []*hcloud.Volume{{ID: 4}}
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 4}).
		Return(hcloud.Error{Code: hcloud.ErrorCodeLocked, Message: "cannot perform operation because resource is locked"}).Once()

	res, err = svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	require.Equal(t, recycleVolumeDetachDelay, res.RequeueAfter)

	// Once hcloud lists no volume, the server goes back to the pool.
	server.Volumes = nil
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(server, nil).Once()

	res, err = svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	// mc fails the test if the server is relabelled while a volume is still listed, because it only
	// expects UpdateServer on the last pass.
}

func Test_returnServerToRecycling_volumeDetachFailureIsRetryable(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	server := &hcloud.Server{
		ID:      5,
		Status:  hcloud.ServerStatusOff,
		Labels:  map[string]string{infrav1.MachineNameTagKey: "worker-1"},
		Volumes: []*hcloud.Volume{{ID: 3}},
	}
	mc.On("DetachVolume", mock.Anything, mock.Anything).
		Return(hcloud.Error{Code: hcloud.ErrorCodeServerError, Message: "internal error"}).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.Error(t, err, "the error must surface so the deletion is retried")
	// mc fails the test if the server is re-pooled with the volume still attached.
}

// errVolumeNotAttached is what hcloud answers for a detach of a volume that is not attached.
var errVolumeNotAttached = hcloud.Error{Code: hcloud.ErrorCodeServiceError, Message: "volume not attached to a server"}

// Test_returnServerToRecycling_volumeDetachedInTheMeantime covers a detach that finished between reading
// the server and sending the detach again. hcloud answers with a plain service_error, which must not fail
// the deletion when the server no longer lists the volume.
func Test_returnServerToRecycling_volumeDetachedInTheMeantime(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	server := &hcloud.Server{
		ID:      5,
		Status:  hcloud.ServerStatusOff,
		Labels:  map[string]string{infrav1.MachineNameTagKey: "worker-1"},
		Volumes: []*hcloud.Volume{{ID: 3}},
	}
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 3}).Return(errVolumeNotAttached).Once()
	mc.On("GetServer", mock.Anything, int64(5)).Return(&hcloud.Server{ID: 5, Status: hcloud.ServerStatusOff}, nil).Once()

	res, err := svc.returnServerToRecycling(context.Background(), server)
	require.NoError(t, err)
	require.Equal(t, recycleVolumeDetachDelay, res.RequeueAfter, "the next look starts from a fresh listing")
}

// Test_returnServerToRecycling_volumeDetachServiceErrorIsRetryable covers a service_error for a volume
// that hcloud still lists on the server. That is a real failure and has to surface.
func Test_returnServerToRecycling_volumeDetachServiceErrorIsRetryable(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	server := &hcloud.Server{
		ID:      5,
		Status:  hcloud.ServerStatusOff,
		Labels:  map[string]string{infrav1.MachineNameTagKey: "worker-1"},
		Volumes: []*hcloud.Volume{{ID: 3}},
	}
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 3}).Return(errVolumeNotAttached).Once()
	mc.On("GetServer", mock.Anything, int64(5)).Return(server, nil).Once()

	_, err := svc.returnServerToRecycling(context.Background(), server)
	require.Error(t, err)
	require.True(t, hcloud.IsError(err, hcloud.ErrorCodeServiceError))
	// mc fails the test if the server is re-pooled with the volume still attached.
}

// Test_tryClaimRecyclableServer_volumeDetachedInTheMeantimeKeepsTheClaim covers the same race in the
// claim. Giving the claim up would wait a minute and record a warning for nothing.
func Test_tryClaimRecyclableServer_volumeDetachedInTheMeantimeKeepsTheClaim(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}
	claimed := recycleClaimedServer(t)
	claimed.Volumes = []*hcloud.Volume{{ID: 3}}
	detached := recycleClaimedServer(t)

	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{claimed}, nil).Once()
	mc.On("GetServer", mock.Anything, claimed.ID).Return(claimed, nil).Once()
	mc.On("DetachVolume", mock.Anything, &hcloud.Volume{ID: 3}).Return(errVolumeNotAttached).Once()
	mc.On("GetServer", mock.Anything, claimed.ID).Return(detached, nil).Once()

	got, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.Nil(t, got)
	require.ErrorIs(t, err, errRecycleClaimWaiting, "the claim is kept and rebuilds on the next look")
	require.NotErrorIs(t, err, errRecycleClaimFailed)
	// mc fails the test if the claim is released, which would need an UpdateServer.
}
