// Package network implements the lifecycle of HCloud networks
package network

import (
	"context"
	"fmt"
	"net"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"sigs.k8s.io/cluster-api/util/record"
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
	s.scope.Info("Reconciling network", "spec", s.scope.HetznerCluster.Spec.HCloudNetworkSpec)
	if !s.scope.HetznerCluster.Spec.HCloudNetworkSpec.NetworkEnabled {
		return nil
	}

	network, err := s.findNetwork(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find network")
	}
	if network == nil {
		// create network
		network, err = s.createNetwork(ctx, &s.scope.HetznerCluster.Spec.HCloudNetworkSpec)
		if err != nil {
			return errors.Wrap(err, "failed to create network")
		}
	}

	s.scope.HetznerCluster.Status.Network = apiToStatus(network)
	return nil
}

func (s *Service) createNetwork(ctx context.Context, spec *infrav1.HCloudNetworkSpec) (*hcloud.Network, error) {
	hc := s.scope.HetznerCluster

	_, network, err := net.ParseCIDR(spec.CIDRBlock)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid network '%s'", spec.CIDRBlock)
	}

	_, subnet, err := net.ParseCIDR(s.scope.HetznerCluster.Spec.HCloudNetworkSpec.CIDRBlock)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid network '%s'", s.scope.HetznerCluster.Spec.HCloudNetworkSpec.CIDRBlock)
	}

	opts := hcloud.NetworkCreateOpts{
		Name:    hc.Name,
		IPRange: network,
		Labels:  s.labels(),
		Subnets: []hcloud.NetworkSubnet{
			{
				IPRange:     subnet,
				NetworkZone: hcloud.NetworkZone(s.scope.HetznerCluster.Spec.HCloudNetworkSpec.NetworkZone),
				Type:        hcloud.NetworkSubnetTypeServer,
			},
		},
	}

	respNetworkCreate, _, err := s.scope.HCloudClient().CreateNetwork(ctx, opts)
	if err != nil {
		record.Warnf(
			s.scope.HetznerCluster,
			"NetworkCreatedFailed",
			"Failed to create network with opts %s",
			opts)
		return nil, errors.Wrap(err, "error creating network")
	}

	record.Eventf(
		s.scope.HetznerCluster,
		"NetworkCreated",
		"Created network with opts %s",
		opts)

	return respNetworkCreate, nil
}

// Delete implements deletion of networks.
func (s *Service) Delete(ctx context.Context) error {
	_, err := s.scope.HCloudClient().DeleteNetwork(ctx, &hcloud.Network{ID: s.scope.HetznerCluster.Status.Network.ID})
	if err != nil {
		// If resource has been deleted already then do nothing
		if hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			s.scope.V(1).Info("deleting network failed - not found", "id", s.scope.HetznerCluster.Status.Network.ID)
			return nil
		}
		record.Warnf(
			s.scope.HetznerCluster,
			"NetworkDeleteFailed",
			"Failed to delete network with ID %v",
			s.scope.HetznerCluster.Status.Network.ID)
		return err
	}
	record.Eventf(
		s.scope.HetznerCluster,
		"NetworkDeleted",
		"Deleted network with ID %v",
		s.scope.HetznerCluster.Status.Network.ID)
	s.scope.V(1).Info("Delete network", "id", s.scope.HetznerCluster.Status.Network.ID)

	return nil
}

func (s *Service) findNetwork(ctx context.Context) (*hcloud.Network, error) {
	opts := hcloud.NetworkListOpts{}
	opts.LabelSelector = utils.LabelsToLabelSelector(s.labels())
	networks, err := s.scope.HCloudClient().ListNetworks(ctx, opts)
	if err != nil {
		return nil, err
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

func apiToStatus(network *hcloud.Network) *infrav1.NetworkStatus {
	attachedServerIDs := make([]int, 0, len(network.Servers))
	for _, s := range network.Servers {
		attachedServerIDs = append(attachedServerIDs, s.ID)
	}

	return &infrav1.NetworkStatus{
		ID:             network.ID,
		Labels:         network.Labels,
		AttachedServer: attachedServerIDs,
	}
}

func (s *Service) labels() map[string]string {
	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
	return map[string]string{
		clusterTagKey: string(infrav1.ResourceLifecycleOwned),
	}
}
