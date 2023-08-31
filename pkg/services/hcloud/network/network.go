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

	"github.com/hetznercloud/hcloud-go/hcloud"
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
	if !s.scope.HetznerCluster.Spec.HCloudNetwork.Enabled {
		return nil
	}

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

	conditions.MarkTrue(s.scope.HetznerCluster, infrav1.NetworkAttached)
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

	record.Eventf(s.scope.HetznerCluster, "NetworkCreated", "Created network with opts %s", opts)
	return resp, nil
}

func (s *Service) createOpts() (hcloud.NetworkCreateOpts, error) {
	spec := s.scope.HetznerCluster.Spec.HCloudNetwork

	_, network, err := net.ParseCIDR(spec.CIDRBlock)
	if err != nil {
		return hcloud.NetworkCreateOpts{}, fmt.Errorf("invalid network %q: %w", spec.CIDRBlock, err)
	}

	_, subnet, err := net.ParseCIDR(spec.SubnetCIDRBlock)
	if err != nil {
		return hcloud.NetworkCreateOpts{}, fmt.Errorf("invalid network %q: %w", spec.SubnetCIDRBlock, err)
	}

	return hcloud.NetworkCreateOpts{
		Name:    s.scope.HetznerCluster.Name,
		IPRange: network,
		Labels:  s.labels(),
		Subnets: []hcloud.NetworkSubnet{
			{
				IPRange:     subnet,
				NetworkZone: hcloud.NetworkZone(spec.NetworkZone),
				Type:        hcloud.NetworkSubnetTypeServer,
			},
		},
	}, nil
}

// Delete implements deletion of the network.
func (s *Service) Delete(ctx context.Context) error {
	if s.scope.HetznerCluster.Status.Network == nil {
		// nothing to delete
		return nil
	}

	id := s.scope.HetznerCluster.Status.Network.ID

	if err := s.scope.HCloudClient.DeleteNetwork(ctx, &hcloud.Network{ID: id}); err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeleteNetwork")
		// if resource has been deleted already then do nothing
		if hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			s.scope.V(1).Info("deleting network failed - not found", "id", id)
			return nil
		}
		record.Warnf(s.scope.HetznerCluster, "NetworkDeleteFailed", "Failed to delete network with ID %v", id)
		return fmt.Errorf("failed to delete network: %w", err)
	}

	record.Eventf(s.scope.HetznerCluster, "NetworkDeleted", "Deleted network with ID %v", id)
	return nil
}

func (s *Service) findNetwork(ctx context.Context) (*hcloud.Network, error) {
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
	attachedServerIDs := make([]int, 0, len(network.Servers))
	for _, s := range network.Servers {
		attachedServerIDs = append(attachedServerIDs, s.ID)
	}

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
