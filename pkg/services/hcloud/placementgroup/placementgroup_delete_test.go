/*
Copyright 2026 The Kubernetes Authors.

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

package placementgroup

import (
	"context"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/record"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
)

func TestDeleteReturnsUnderlyingError(t *testing.T) {
	hcloudClient := mocks.NewClient(t)
	hcloudClient.On("DeletePlacementGroup", mock.Anything, int64(42)).
		Return(hcloud.Error{Code: hcloud.ErrorCodeLocked, Message: "placement group is locked"})

	hetznerCluster := &infrav1.HetznerCluster{}
	hetznerCluster.Status.HCloudPlacementGroups = []infrav1.HCloudPlacementGroupStatus{{ID: 42, Name: "md-0"}}

	svc := NewService(&scope.ClusterScope{
		HCloudClient:   hcloudClient,
		HetznerCluster: hetznerCluster,
		EventRecorder:  record.NewFakeRecorder(10),
	})

	err := svc.Delete(context.Background())
	require.Error(t, err)
	require.True(t, hcloud.IsError(err, hcloud.ErrorCodeLocked), "expected the hcloud error to be wrapped, got: %v", err)
	require.Contains(t, err.Error(), "placement group is locked")
}
