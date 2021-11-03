// Package network implements the lifecycle of Hcloud networks
package network

import (
	"context"
	"net"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
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

func apiToStatus(network *hcloud.Network) *infrav1.NetworkStatus {
	var subnets = make([]infrav1.SubnetSpec, len(network.Subnets))
	for pos, n := range network.Subnets {
		subnets[pos].NetworkZone = infrav1.HCloudNetworkZone(n.NetworkZone)
		subnets[pos].CIDRBlock = n.IPRange.String()
	}

	attachedServerIDs := make([]int, 0, len(network.Servers))
	for _, s := range network.Servers {
		attachedServerIDs = append(attachedServerIDs, s.ID)
	}

	var status infrav1.NetworkStatus
	status.ID = network.ID
	status.CIDRBlock = network.IPRange.String()
	status.Subnets = subnets
	status.Labels = network.Labels
	status.AttachedServer = attachedServerIDs
	return &status
}

func (s *Service) labels() map[string]string {
	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
	return map[string]string{
		clusterTagKey: string(infrav1.ResourceLifecycleOwned),
	}
}

// Reconcile implements life cycle of networks.
func (s *Service) Reconcile(ctx context.Context) (err error) {
	log.Info("Reconciling network")
	if !s.scope.HetznerCluster.Spec.NetworkSpec.NetworkEnabled {
		return nil
	}

	// update current status
	networkStatus, err := s.actualStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to refresh networks")
	}
	s.scope.HetznerCluster.Status.Network = networkStatus

	if networkStatus != nil {
		return nil
	}

	// set defaults if nothing is set
	if s.scope.HetznerCluster.Spec.NetworkSpec.CIDRBlock == "" {
		s.scope.HetznerCluster.Spec.NetworkSpec.CIDRBlock = "10.0.0.0/16"
	}
	if len(s.scope.HetznerCluster.Spec.NetworkSpec.Subnets) == 0 {
		s.scope.HetznerCluster.Spec.NetworkSpec.Subnets = []infrav1.SubnetSpec{
			{
				NetworkZone: "eu-central",

				CIDRBlock: "10.0.0.0/24",
			},
		}
	}

	// create network
	networkStatus, err = s.createNetwork(ctx, &s.scope.HetznerCluster.Spec.NetworkSpec)
	if err != nil {
		return errors.Wrap(err, "failed to create network")
	}
	s.scope.HetznerCluster.Status.Network = networkStatus
	return nil
}

func (s *Service) createNetwork(ctx context.Context, spec *infrav1.NetworkSpec) (*infrav1.NetworkStatus, error) {
	hc := s.scope.HetznerCluster

	s.scope.V(2).Info("Create a new network", "cidrBlock", spec.CIDRBlock, "subnets", spec.Subnets)
	_, network, err := net.ParseCIDR(spec.CIDRBlock)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid network '%s'", spec.CIDRBlock)
	}

	var subnets = make([]hcloud.NetworkSubnet, len(spec.Subnets))
	for pos, sn := range spec.Subnets {
		_, subnet, err := net.ParseCIDR(sn.CIDRBlock)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid network '%s'", sn.CIDRBlock)
		}
		subnets[pos].IPRange = subnet
		subnets[pos].NetworkZone = hcloud.NetworkZone(sn.NetworkZone)
		subnets[pos].Type = hcloud.NetworkSubnetTypeServer
	}

	opts := hcloud.NetworkCreateOpts{
		Name:    hc.Name,
		IPRange: network,
		Labels:  s.labels(),
		Subnets: subnets,
	}

	s.scope.V(1).Info("Create a new network", "opts", opts)

	respNetworkCreate, _, err := s.scope.HCloudClient().CreateNetwork(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "error creating network")
	}

	return apiToStatus(respNetworkCreate), nil
}

func (s *Service) deleteNetwork(ctx context.Context, status *infrav1.NetworkStatus) error {
	// ensure deleted network is actually owned by us
	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
	if status.Labels == nil || infrav1.ResourceLifecycle(status.Labels[clusterTagKey]) != infrav1.ResourceLifecycleOwned {
		s.scope.V(3).Info("Ignore request to delete network, as it is not owned", "id", status.ID, "cidrBlock", status.CIDRBlock)
		return nil
	}
	_, err := s.scope.HCloudClient().DeleteNetwork(ctx, &hcloud.Network{ID: status.ID})
	s.scope.V(2).Info("Delete network", "id", status.ID, "cidrBlock", status.CIDRBlock)
	return err
}

// Delete implements deletion of networks.
func (s *Service) Delete(ctx context.Context) (err error) {
	// update current status
	networkStatus, err := s.actualStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to refresh networks")
	}

	if networkStatus == nil {
		return nil
	}

	if err := s.deleteNetwork(ctx, networkStatus); err != nil {
		return errors.Wrap(err, "failed to delete network")
	}

	return nil
}

func (s *Service) actualStatus(ctx context.Context) (*infrav1.NetworkStatus, error) {
	opts := hcloud.NetworkListOpts{}
	opts.LabelSelector = utils.LabelsToLabelSelector(s.labels())
	networks, err := s.scope.HCloudClient().ListNetworks(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, n := range networks {
		return apiToStatus(n), nil
	}

	return nil, nil
}
