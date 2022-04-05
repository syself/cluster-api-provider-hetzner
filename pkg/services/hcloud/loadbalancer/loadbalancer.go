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
	kerrors "k8s.io/apimachinery/pkg/util/errors"
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
	log.V(1).Info("Reconcile load balancer")

	// find load balancer
	lb, err := s.findLoadBalancer(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find load balancer")
	}

	if lb == nil {
		lb, err = s.createLoadBalancer(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create load balancer")
		}
	}

	// update current status
	lbStatus, err := apiToStatus(lb, s.scope.HetznerCluster.Status.Network != nil)
	if err != nil {
		return errors.Wrap(err, "failed to get status from api object")
	}

	s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = &lbStatus

	// Check whether load balancer name, algorithm or type has been changed
	if err := s.reconcileLBProperties(ctx, lb); err != nil {
		return errors.Wrap(err, "failed to reconcile load balancer properties")
	}

	// reconcile network attachement
	if err := s.reconcileNetworkAttachement(ctx, lb); err != nil {
		return errors.Wrap(err, "failed to reconcile network attachement")
	}

	// reconcile targets
	if err := s.reconcileServices(ctx, lb); err != nil {
		return errors.Wrap(err, "failed to reconcile targets")
	}

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

	if _, err := s.scope.HCloudClient.AttachLoadBalancerToNetwork(ctx, lb, hcloud.LoadBalancerAttachToNetworkOpts{
		Network: &hcloud.Network{
			ID: s.scope.HetznerCluster.Status.Network.ID,
		},
	}); err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerCluster, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerCluster,
				"RateLimitExceeded",
				"exceeded rate limit with calling hcloud function AttachServerToNetwork",
			)
		}
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

func (s *Service) reconcileLBProperties(ctx context.Context, lb *hcloud.LoadBalancer) error {
	var multierr []error
	// Check if type has been updated
	if s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Type != lb.LoadBalancerType.Name {
		if _, err := s.scope.HCloudClient.ChangeLoadBalancerType(ctx, lb, hcloud.LoadBalancerChangeTypeOpts{
			LoadBalancerType: &hcloud.LoadBalancerType{
				Name: s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Type,
			},
		}); err != nil {
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerCluster, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerCluster,
					"RateLimitExceeded",
					"exceeded rate limit with calling hcloud function ChangeLoadBalancerType",
				)
				return errors.Wrap(err, "rate limit exceeded while changing lb type")
			}
			multierr = append(multierr, errors.Wrap(err, "failed to change load balancer type"))
		}
		record.Eventf(s.scope.HetznerCluster, "ChangeLoadBalancerType", "Changed load balancer type")
	}

	// Check if algorithm has been updated
	if string(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Algorithm) != string(lb.Algorithm.Type) {
		if _, err := s.scope.HCloudClient.ChangeLoadBalancerAlgorithm(ctx, lb, hcloud.LoadBalancerChangeAlgorithmOpts{
			Type: hcloud.LoadBalancerAlgorithmType(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Algorithm),
		}); err != nil {
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerCluster, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerCluster,
					"RateLimitExceeded",
					"exceeded rate limit with calling hcloud function ChangeLoadBalancerAlgorithm",
				)
				return errors.Wrap(err, "rate limit exceeded while changing lb algorithm")
			}
			multierr = append(multierr, errors.Wrap(err, "failed to change load balancer algorithm"))
		}
		record.Eventf(s.scope.HetznerCluster, "ChangeLoadBalancerAlgorithm", "Changed load balancer algorithm")
	}

	// Check if name has been updated
	if s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name != nil &&
		*s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name != lb.Name {
		if _, err := s.scope.HCloudClient.UpdateLoadBalancer(ctx, lb, hcloud.LoadBalancerUpdateOpts{
			Name: *s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name,
		}); err != nil {
			multierr = append(multierr, errors.Wrap(err, "failed to update load balancer name"))
		}
		record.Eventf(s.scope.HetznerCluster, "ChangeLoadBalancerName", "Changed load balancer name")
	}

	return kerrors.NewAggregate(multierr)
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func (s *Service) reconcileServices(ctx context.Context, lb *hcloud.LoadBalancer) error {
	// Build slices and maps to make diffs
	lbServiceListenPorts := make([]int, 0, max(len(lb.Services)-1, 0))
	specServiceListenPorts := make([]int, len(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices))
	specServiceListenPortsMap := make(map[int]infrav1.LoadBalancerServiceSpec, len(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices))

	for _, service := range lb.Services {
		// Do nothing for kubeAPI service
		if service.ListenPort == int(s.scope.HetznerCluster.Spec.ControlPlaneEndpoint.Port) {
			continue
		}
		lbServiceListenPorts = append(lbServiceListenPorts, service.ListenPort)
	}

	for i, serviceInSpec := range s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices {
		specServiceListenPorts[i] = serviceInSpec.ListenPort
		specServiceListenPortsMap[serviceInSpec.ListenPort] = serviceInSpec
	}

	toCreate, toDelete := utils.DifferenceOfIntSlices(specServiceListenPorts, lbServiceListenPorts)

	// Delete services which are registered for lb but are not in specs
	var multierr []error

	for _, listenPort := range toDelete {
		if _, ok := specServiceListenPortsMap[listenPort]; !ok {
			if _, err := s.scope.HCloudClient.DeleteServiceFromLoadBalancer(ctx, lb, listenPort); err != nil {
				multierr = append(multierr, fmt.Errorf("error deleting service from load balancer: %s", err))
			}
		}
	}

	// Create services which are in specs and not yet in API
	for i, listenPort := range toCreate {
		proxyProtocol := false
		destinationPort := specServiceListenPortsMap[listenPort].DestinationPort
		serviceOpts := hcloud.LoadBalancerAddServiceOpts{
			Protocol:        hcloud.LoadBalancerServiceProtocol(specServiceListenPortsMap[listenPort].Protocol),
			ListenPort:      &toCreate[i],
			DestinationPort: &destinationPort,
			Proxyprotocol:   &proxyProtocol,
		}
		if _, err := s.scope.HCloudClient.AddServiceToLoadBalancer(ctx, lb, serviceOpts); err != nil {
			multierr = append(multierr, fmt.Errorf("error adding service to load balancer: %s", err))
		}
	}

	return kerrors.NewAggregate(multierr)
}

func (s *Service) createLoadBalancer(ctx context.Context) (*hcloud.LoadBalancer, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Create a new loadbalancer", "algorithm type", s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Algorithm)

	res, err := s.scope.HCloudClient.CreateLoadBalancer(ctx, buildLoadBalancerCreateOpts(s.scope.HetznerCluster))
	if err != nil {
		record.Warnf(
			s.scope.HetznerCluster,
			"FailedCreateLoadBalancer",
			"Failed to create load balancer: %s",
			err)
		return nil, errors.Wrap(err, "error creating load balancer")
	}

	record.Eventf(s.scope.HetznerCluster, "CreateLoadBalancer", "Created load balancer")
	return res.LoadBalancer, nil
}

func buildLoadBalancerCreateOpts(hc *infrav1.HetznerCluster) hcloud.LoadBalancerCreateOpts {
	// gather algorithm type
	algorithmType := hc.Spec.ControlPlaneLoadBalancer.Algorithm.HCloudAlgorithmType()

	// Set name
	name := utils.GenerateName(
		hc.Spec.ControlPlaneLoadBalancer.Name,
		fmt.Sprintf("%s-kube-apiserver-", hc.Name),
	)

	proxyprotocol := false

	clusterTagKey := infrav1.ClusterTagKey(hc.Name)

	var network *hcloud.Network
	if hc.Status.Network != nil {
		network = &hcloud.Network{
			ID: hc.Status.Network.ID,
		}
	}

	listenPort := int(hc.Spec.ControlPlaneEndpoint.Port)

	return hcloud.LoadBalancerCreateOpts{
		LoadBalancerType: &hcloud.LoadBalancerType{
			Name: hc.Spec.ControlPlaneLoadBalancer.Type,
		},
		Name: name,
		Algorithm: &hcloud.LoadBalancerAlgorithm{
			Type: algorithmType,
		},
		Location: &hcloud.Location{
			Name: string(hc.Spec.ControlPlaneLoadBalancer.Region),
		},
		Network: network,
		Labels: map[string]string{
			clusterTagKey: string(infrav1.ResourceLifecycleOwned),
		},
		Services: []hcloud.LoadBalancerCreateOptsService{
			{
				Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
				ListenPort:      &listenPort,
				DestinationPort: &hc.Spec.ControlPlaneLoadBalancer.Port,
				Proxyprotocol:   &proxyprotocol,
			},
		},
	}
}

// Delete implements the deletion of HCloud load balancers.
func (s *Service) Delete(ctx context.Context) (err error) {
	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		// nothing to do
		return nil
	}

	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Protected {
		record.Eventf(s.scope.HetznerCluster, "LoadBalancerProtectedFromDeletion", "Cannot delete load balancer as it is protected")
		return nil
	}

	if err := s.scope.HCloudClient.DeleteLoadBalancer(ctx, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID); err != nil {
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

func (s *Service) findLoadBalancer(ctx context.Context) (*hcloud.LoadBalancer, error) {
	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
	loadBalancers, err := s.scope.HCloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{
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
func apiToStatus(lb *hcloud.LoadBalancer, hasNetwork bool) (infrav1.LoadBalancerStatus, error) {
	ipv4 := lb.PublicNet.IPv4.IP.String()
	ipv6 := lb.PublicNet.IPv6.IP.String()

	var internalIP string
	if hasNetwork && len(lb.PrivateNet) > 0 {
		internalIP = lb.PrivateNet[0].IP.String()
	}

	targets := make([]infrav1.LoadBalancerTarget, 0, len(lb.Targets))
	for _, target := range lb.Targets {
		switch target.Type {
		case hcloud.LoadBalancerTargetTypeServer:
			targets = append(targets, infrav1.LoadBalancerTarget{
				Type:     infrav1.LoadBalancerTargetTypeServer,
				ServerID: target.Server.Server.ID,
			},
			)
		case hcloud.LoadBalancerTargetTypeIP:
			targets = append(targets, infrav1.LoadBalancerTarget{
				Type: infrav1.LoadBalancerTargetTypeIP,
				IP:   target.IP.IP,
			},
			)
		default:
			return infrav1.LoadBalancerStatus{}, fmt.Errorf("unknown load balancer target type %s", target.Type)
		}
	}

	return infrav1.LoadBalancerStatus{
		ID:         lb.ID,
		IPv4:       ipv4,
		IPv6:       ipv6,
		InternalIP: internalIP,
		Target:     targets,
		Protected:  lb.Protection.Delete,
	}, nil
}
