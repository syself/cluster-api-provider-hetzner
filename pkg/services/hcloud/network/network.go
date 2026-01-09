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

// Package network implements the lifecycle of HCloud networks.
package network

import (
	"context"
	"fmt"
	"net"
	"slices"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// Service struct contains cluster scope to reconcile networks.
type Service struct {
	scope *scope.ClusterScope
}

// NewService creates a new service object.
func NewService(scope *scope.ClusterScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements life cycle of networks.
func (s *Service) Reconcile(ctx context.Context) (err error) {
	// delete the deprecated condition from existing cluster objects
	conditions.Delete(s.scope.HetznerCluster, infrav1.DeprecatedNetworkAttachedCondition)

	if !s.scope.HetznerCluster.Spec.HCloudNetwork.Enabled {
		return nil
	}

	defer func() {
		if err != nil {
			conditions.MarkFalse(
				s.scope.HetznerCluster,
				infrav1.NetworkReadyCondition,
				infrav1.NetworkReconcileFailedReason,
				clusterv1.ConditionSeverityWarning,
				"%s",
				err.Error(),
			)
		}
	}()

	network, err := s.findNetwork(ctx)
	if err != nil {
		return fmt.Errorf("failed to find network: %w", err)
	}

	if network == nil {
		network, err = s.createNetwork(ctx)
		if err != nil {
			return fmt.Errorf("failed to create network: %w", err)
		}
	}

	if len(network.Subnets) > 1 {
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.NetworkReadyCondition,
			infrav1.MultipleSubnetsExistReason,
			clusterv1.ConditionSeverityWarning,
			"multiple subnets exist",
		)
		record.Warnf(s.scope.HetznerCluster, "MultipleSubnetsExist", "Multiple subnets exist")
		return nil
	}

	conditions.MarkTrue(s.scope.HetznerCluster, infrav1.NetworkReadyCondition)
	s.scope.HetznerCluster.Status.Network = statusFromHCloudNetwork(network)

	return nil
}

func (s *Service) createNetwork(ctx context.Context) (*hcloud.Network, error) {
	opts, err := s.createOpts()
	if err != nil {
		return nil, fmt.Errorf("failed to create NetworkCreateOpts: %w", err)
	}

	resp, err := s.scope.HCloudClient.CreateNetwork(ctx, opts)
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "CreateNetwork")
		record.Warnf(s.scope.HetznerCluster, "NetworkCreatedFailed", "Failed to create network with opts %s", opts)
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	record.Eventf(s.scope.HetznerCluster, "NetworkCreated", "Created network with opts %+v", opts)
	return resp, nil
}

func (s *Service) createOpts() (hcloud.NetworkCreateOpts, error) {
	spec := s.scope.HetznerCluster.Spec.HCloudNetwork

	if spec.CIDRBlock == nil || spec.SubnetCIDRBlock == nil || spec.NetworkZone == nil {
		return hcloud.NetworkCreateOpts{}, fmt.Errorf("nil CIDRs or NetworkZone given")
	}

	_, network, err := net.ParseCIDR(*spec.CIDRBlock)
	if err != nil {
		return hcloud.NetworkCreateOpts{}, fmt.Errorf("invalid network %q: %w", *spec.CIDRBlock, err)
	}

	_, subnet, err := net.ParseCIDR(*spec.SubnetCIDRBlock)
	if err != nil {
		return hcloud.NetworkCreateOpts{}, fmt.Errorf("invalid network %q: %w", *spec.SubnetCIDRBlock, err)
	}

	return hcloud.NetworkCreateOpts{
		Name:    s.scope.HetznerCluster.Name,
		IPRange: network,
		Labels:  s.labels(),
		Subnets: []hcloud.NetworkSubnet{
			{
				IPRange:     subnet,
				NetworkZone: hcloud.NetworkZone(*spec.NetworkZone),
				Type:        hcloud.NetworkSubnetTypeCloud,
			},
		},
	}, nil
}

// Delete implements deletion of the network.
func (s *Service) Delete(ctx context.Context) error {
	hetznerCluster := s.scope.HetznerCluster

	if hetznerCluster.Status.Network == nil {
		// nothing to delete
		return nil
	}

	// only delete the network if it is owned by us
	if hetznerCluster.Status.Network.Labels[hetznerCluster.ClusterTagKey()] != string(infrav1.ResourceLifecycleOwned) {
		s.scope.V(1).Info("network is not owned by us", "id", hetznerCluster.Status.Network.ID, "labels", hetznerCluster.Status.Network.Labels)
		return nil
	}

	id := hetznerCluster.Status.Network.ID

	if err := s.scope.HCloudClient.DeleteNetwork(ctx, &hcloud.Network{ID: id}); err != nil {
		hcloudutil.HandleRateLimitExceeded(hetznerCluster, err, "DeleteNetwork")
		// if resource has been deleted already then do nothing
		if hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			s.scope.V(1).Info("deleting network failed - not found", "id", id)
			return nil
		}
		record.Warnf(hetznerCluster, "NetworkDeleteFailed", "Failed to delete network with ID %v", id)
		return fmt.Errorf("failed to delete network: %w", err)
	}

	record.Eventf(hetznerCluster, "NetworkDeleted", "Deleted network with ID %v", id)
	return nil
}

func (s *Service) findNetworkByID(ctx context.Context, id int64) (*hcloud.Network, error) {
	network, err := s.scope.HCloudClient.GetNetwork(ctx, id)
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "GetNetwork")
		return nil, fmt.Errorf("failed to get network %d: %w", id, err)
	}

	return network, nil
}

func (s *Service) findNetwork(ctx context.Context) (*hcloud.Network, error) {
	// if an ID was provided we want to use the existing Network.
	id := s.scope.HetznerCluster.Spec.HCloudNetwork.ID
	if id != nil {
		network, err := s.findNetworkByID(ctx, *id)
		if err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "GetNetwork")
			return nil, fmt.Errorf("failed to find network with id %d: %w", *id, err)
		}

		if network != nil {
			s.scope.V(1).Info("found network", "id", network.ID, "name", network.Name, "labels", network.Labels)
			return network, nil
		}
	}

	opts := hcloud.NetworkListOpts{}
	opts.LabelSelector = utils.LabelsToLabelSelector(s.labels())

	networks, err := s.scope.HCloudClient.ListNetworks(ctx, opts)
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "ListNetworks")
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	if len(networks) > 1 {
		return nil, fmt.Errorf("found multiple networks with opts %v - not allowed", opts)
	}

	if len(networks) == 0 {
		return nil, nil
	}

	if len(networks[0].Subnets) > 1 {
		return nil, fmt.Errorf("multiple subnets not allowed")
	}

	return networks[0], nil
}

func statusFromHCloudNetwork(network *hcloud.Network) *infrav1.NetworkStatus {
	attachedServerIDs := make([]int64, 0, len(network.Servers))
	for _, s := range network.Servers {
		attachedServerIDs = append(attachedServerIDs, s.ID)
	}
	// The server IDs are not sorted by the API, but we want to have a
	// deterministic order to avoid unnecessary updates to the HetznerCluster resource.
	slices.Sort(attachedServerIDs)

	return &infrav1.NetworkStatus{
		ID:              network.ID,
		Labels:          network.Labels,
		AttachedServers: attachedServerIDs,
	}
}

func (s *Service) labels() map[string]string {
	clusterTagKey := s.scope.HetznerCluster.ClusterTagKey()
	return map[string]string{
		clusterTagKey: string(infrav1.ResourceLifecycleOwned),
	}
}
