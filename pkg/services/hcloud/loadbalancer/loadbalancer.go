// Package loadbalancer implements the lifecycle of HCloud load balancers
package loadbalancer

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"k8s.io/apiserver/pkg/storage/names"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Service is a struct with the cluster scope to reconcile load balancers.
type Service struct {
	scope *scope.ClusterScope
}

// NewService creates a new service object.
func NewService(scope *scope.ClusterScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements the life cycle of HCloud load balancers.
func (s *Service) Reconcile(ctx context.Context) (err error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconcile load balancer")

	// find load balancer
	lb, err := findLoadBalancer(s.scope)
	if err != nil {
		return errors.Wrap(err, "failed to find load balancer")
	}

	if lb == nil {
		lb, err = s.createLoadBalancer(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create load balancer")
		}
	}

	// reconcile network attachement
	if err := s.reconcileNetworkAttachement(ctx, lb); err != nil {
		return errors.Wrap(err, "failed to reconcile network attachement")
	}

	// update current status
	lbStatus := s.apiToStatus(lb)
	s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = &lbStatus

	return nil
}

func (s *Service) reconcileNetworkAttachement(ctx context.Context, lb *hcloud.LoadBalancer) error {
	// Nothing to do if already attached to network
	if len(lb.PrivateNet) > 0 {
		return nil
	}

	// attach load balancer to network
	if s.scope.HetznerCluster.Status.Network == nil {
		conditions.MarkFalse(s.scope.HetznerCluster,
			infrav1.LoadBalancerAttachedToNetworkCondition,
			infrav1.LoadBalancerNoNetworkFoundReason,
			clusterv1.ConditionSeverityInfo,
			"no network found")
		return nil
	}

	if _, _, err := s.scope.HCloudClient().AttachLoadBalancerToNetwork(ctx, lb, hcloud.LoadBalancerAttachToNetworkOpts{
		Network: &hcloud.Network{
			ID: s.scope.HetznerCluster.Status.Network.ID,
		},
	}); err != nil {
		// In case lb is already attached don't raise an error
		if hcloud.IsError(err, hcloud.ErrorCodeLoadBalancerAlreadyAttached) {
			return nil
		}
		record.Warnf(
			s.scope.HetznerCluster,
			"FailedAttachLoadBalancer",
			"Failed to attach load balancer to network: %s",
			err)
		conditions.MarkFalse(s.scope.HetznerCluster,
			infrav1.LoadBalancerAttachedToNetworkCondition,
			infrav1.LoadBalancerAttachFailedReason,
			clusterv1.ConditionSeverityError,
			err.Error())
		return errors.Wrap(err, "failed to attach load balancer to network")
	}

	conditions.MarkTrue(s.scope.HetznerCluster, infrav1.LoadBalancerAttachedToNetworkCondition)

	return nil
}

func (s *Service) createLoadBalancer(ctx context.Context) (*hcloud.LoadBalancer, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Create a new loadbalancer", "algorithm type", s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Algorithm)

	// gather algorithm type
	var algType hcloud.LoadBalancerAlgorithmType
	switch s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Algorithm {
	case infrav1.LoadBalancerAlgorithmTypeLeastConnections:
		algType = hcloud.LoadBalancerAlgorithmTypeRoundRobin
	case infrav1.LoadBalancerAlgorithmTypeRoundRobin:
		algType = hcloud.LoadBalancerAlgorithmTypeLeastConnections
	default:
		return nil, fmt.Errorf("error invalid load balancer algorithm type: %s", s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Algorithm)
	}

	name := names.SimpleNameGenerator.GenerateName(s.scope.HetznerCluster.Name + "-kube-apiserver-")
	if s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name != nil {
		name = *s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name
	}

	// Get the Hetzner cloud object of load balancer type
	loadBalancerType, _, err := s.scope.HCloudClient().GetLoadBalancerTypeByName(ctx, s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Type)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find load balancer type")
	}

	var proxyprotocol bool

	// The first service in the list is the one of kubeAPI
	kubeAPISpec := s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Services[0]

	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)

	var network *hcloud.Network
	if s.scope.HetznerCluster.Status.Network != nil {
		network = &hcloud.Network{
			ID: s.scope.HetznerCluster.Status.Network.ID,
		}
	}
	opts := hcloud.LoadBalancerCreateOpts{
		LoadBalancerType: loadBalancerType,
		Name:             name,
		Algorithm: &hcloud.LoadBalancerAlgorithm{
			Type: algType,
		},
		Location: &hcloud.Location{
			Name: string(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Region),
		},
		Network: network,
		Labels: map[string]string{
			clusterTagKey: string(infrav1.ResourceLifecycleOwned),
		},
		Services: []hcloud.LoadBalancerCreateOptsService{
			{
				Protocol:        hcloud.LoadBalancerServiceProtocol(kubeAPISpec.Protocol),
				ListenPort:      &kubeAPISpec.ListenPort,
				DestinationPort: &kubeAPISpec.DestinationPort,
				Proxyprotocol:   &proxyprotocol,
			},
		},
	}

	res, _, err := s.scope.HCloudClient().CreateLoadBalancer(ctx, opts)
	if err != nil {
		record.Warnf(
			s.scope.HetznerCluster,
			"FailedCreateLoadBalancer",
			"Failed to create load balancer: %s",
			err)
		return nil, errors.Wrap(err, "error creating load balancer")
	}

	// If there is more than one service in the specs, add them here one after another
	// Adding all at the same time on creation led to an error that the source port is already in use
	if len(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Services) > 1 {
		for _, spec := range s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Services[1:] {
			serviceOpts := hcloud.LoadBalancerAddServiceOpts{
				Protocol:        hcloud.LoadBalancerServiceProtocol(spec.Protocol),
				ListenPort:      &spec.ListenPort,
				DestinationPort: &spec.DestinationPort,
				Proxyprotocol:   &proxyprotocol,
			}
			_, _, err := s.scope.HCloudClient().AddServiceToLoadBalancer(ctx, res.LoadBalancer, serviceOpts)
			if err != nil {
				return nil, fmt.Errorf("error adding service to load balancer: %s", err)
			}
		}
	}
	record.Eventf(s.scope.HetznerCluster, "CreateLoadBalancer", "Created load balancer")
	return res.LoadBalancer, nil
}

// Delete implements the deletion of HCloud load balancers.
func (s *Service) Delete(ctx context.Context) (err error) {
	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		// nothing to do
		return nil
	}

	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Protected {
		record.Eventf(s.scope.HetznerCluster, "LoadBalancerProtectedFromDeletion", "Failed to delete load balancer as it is protected")
		return nil
	}

	if _, err := s.scope.HCloudClient().DeleteLoadBalancer(ctx, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID); err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			s.scope.V(1).Info("deleting load balancer failed - not found", "id", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
			return nil
		}
		record.Eventf(s.scope.HetznerCluster, "FailedLoadBalancerDelete", "Failed to delete load balancer: %s", err)
		return errors.Wrap(err, "failed to delete load balancer")
	}

	// Delete lb information from cluster status
	s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = nil

	record.Eventf(s.scope.HetznerCluster, "DeleteLoadBalancer", "Deleted load balancer")
	return nil
}

func findLoadBalancer(scope *scope.ClusterScope) (*hcloud.LoadBalancer, error) {
	clusterTagKey := infrav1.ClusterTagKey(scope.HetznerCluster.Name)
	loadBalancers, err := scope.HCloudClient().ListLoadBalancers(scope.Ctx, hcloud.LoadBalancerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: utils.LabelsToLabelSelector(map[string]string{
				clusterTagKey: string(infrav1.ResourceLifecycleOwned),
			}),
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list load balancers")
	}

	if len(loadBalancers) > 1 {
		return nil, fmt.Errorf("found %v loadbalancers in HCloud", len(loadBalancers))
	} else if len(loadBalancers) == 0 {
		return nil, nil
	}

	return loadBalancers[0], nil
}

// gets the information of the Hetzner load balancer object and returns it in our status object.
func (s *Service) apiToStatus(lb *hcloud.LoadBalancer) infrav1.LoadBalancerStatus {
	ipv4 := lb.PublicNet.IPv4.IP.String()
	ipv6 := lb.PublicNet.IPv6.IP.String()

	var internalIP string
	if s.scope.HetznerCluster.Status.Network != nil && len(lb.PrivateNet) > 0 {
		internalIP = lb.PrivateNet[0].IP.String()
	}

	targetIDs := make([]int, 0, len(lb.Targets))
	for _, server := range lb.Targets {
		targetIDs = append(targetIDs, server.Server.Server.ID)
	}

	return infrav1.LoadBalancerStatus{
		ID:         lb.ID,
		IPv4:       ipv4,
		IPv6:       ipv6,
		InternalIP: internalIP,
		Target:     targetIDs,
		Protected:  lb.Protection.Delete,
	}
}
