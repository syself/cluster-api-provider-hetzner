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
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func Test_tryClaimRecyclableServer_claimsAndRebuilds(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	image := &hcloud.Image{ID: 42, Name: "snapshot"}
	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), image, []byte("#cloud-config"))
	require.NoError(t, err)
	require.NotNil(t, claimed, "an available matching server must be claimed")

	// The claimed server carries the machine name (both as server name and label) and the persistent
	// recycle marker, and no longer advertises itself as available or reserved.
	require.Equal(t, "worker-1", claimed.Name)
	require.Equal(t, "worker-1", claimed.Labels[infrav1.MachineNameTagKey])
	require.Equal(t, "true", claimed.Labels[infrav1.ServerRecycleLabelKey])
	require.NotContains(t, claimed.Labels, infrav1.ServerRecycleAvailableLabelKey)
	require.NotContains(t, claimed.Labels, infrav1.ServerRecycleClaimantLabelKey)

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
	seedRecyclableServer(t, client, "pool-wrong-type", "cx22", recycleTestLocation, true)

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "a server of the wrong type must not be claimed")
	require.Len(t, listAvailableRecyclable(t, client), 1, "the unmatched server stays available")
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

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.Equal(t, low.ID, claimed.ID, "the lowest-ID candidate must be claimed first")

	// The higher-ID server is untouched and still available.
	remaining := listAvailableRecyclable(t, client)
	require.Len(t, remaining, 1)
	require.Equal(t, high.ID, remaining[0].ID)
}

// Test_tryClaimRecyclableServer_lostRace exercises the optimistic-reservation verification: after the
// label write, the re-read shows the server claimed by a different machine, so the candidate is skipped
// and no rebuild happens. The fake client serializes writes and cannot reproduce this, so a mock is
// scripted.
func Test_tryClaimRecyclableServer_lostRace(t *testing.T) {
	mc := mocks.NewClient(t)
	svc := &Service{scope: newRecycleScope(t, "worker-1", mc)}

	candidate := &hcloud.Server{
		ID:         7,
		ServerType: &hcloud.ServerType{Name: recycleTestType},
		Location:   &hcloud.Location{Name: recycleTestLocation},
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:          "true",
			infrav1.ServerRecycleAvailableLabelKey: "true",
		},
	}
	// Re-read after the reservation shows the server claimed by a different machine.
	otherClaimant := &hcloud.Server{
		ID: 7,
		Labels: map[string]string{
			infrav1.ServerRecycleLabelKey:         "true",
			infrav1.ServerRecycleClaimantLabelKey: "worker-2",
		},
	}
	mc.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{candidate}, nil).Once()
	mc.On("UpdateServer", mock.Anything, mock.Anything, mock.Anything).Return(candidate, nil).Once()
	mc.On("GetServer", mock.Anything, int64(7)).Return(otherClaimant, nil).Once()

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), recycleCreateOpts("worker-1"), &hcloud.Image{ID: 42}, nil)
	require.NoError(t, err)
	require.Nil(t, claimed, "losing the reservation race must fall through to a normal create")
	// mc asserts on cleanup that RebuildServer and UpdateServer(finalize) were never called beyond the reservation.
}

// Test_tryClaimRecyclableServer_rebuildFailureReleasesReservation verifies that when the rebuild fails
// after a server has been reserved, the reservation is released (the server returns to the available
// pool) instead of leaving a half-claimed server that a later reconcile could adopt un-rebuilt.
func Test_tryClaimRecyclableServer_rebuildFailureReleasesReservation(t *testing.T) {
	client := fakehcloudclient.NewHCloudClientFactory().NewClient("")
	svc := &Service{scope: newRecycleScope(t, "worker-1", client)}

	seedRecyclableServer(t, client, "pool-a", recycleTestType, recycleTestLocation, true)

	// A network in opts that does not exist in the fake makes the rebuild's network attach fail.
	opts := recycleCreateOpts("worker-1")
	opts.Networks = []*hcloud.Network{{ID: 999}}

	claimed, err := svc.tryClaimRecyclableServer(context.Background(), opts, &hcloud.Image{ID: 42}, []byte("#cloud-config"))
	require.Error(t, err)
	require.Nil(t, claimed)

	// The reservation was released: the server is available again and carries no claimant.
	available := listAvailableRecyclable(t, client)
	require.Len(t, available, 1, "the server must be returned to the available pool")
	require.NotContains(t, available[0].Labels, infrav1.ServerRecycleClaimantLabelKey)
	require.NotContains(t, available[0].Labels, infrav1.MachineNameTagKey, "an un-rebuilt server must not carry the machine identity")
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

	server := &hcloud.Server{ID: 5, Status: hcloud.ServerStatusRunning}
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
