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

// Package loadbalancer implements the lifecycle of HCloud load balancers.
package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/conditions"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// Service is a struct with the cluster scope to reconcile load balancers.
type Service struct {
	scope *scope.ClusterScope
}

// NewService creates a new service object.
func NewService(scope *scope.ClusterScope) *Service {
	return &Service{scope: scope}
}

// ErrNoLoadBalancerAvailable indicates that no available load balancer could be fond.
var ErrNoLoadBalancerAvailable = fmt.Errorf("no available load balancer")

// ErrControlPlaneEndpointNotSet indicates that hetznercluster.spec.controlPlaneEndpoint is not set.
var ErrControlPlaneEndpointNotSet = errors.New("hetznercluster.spec.controlPlaneEndpoint is not set")

// kubeAPIPorts returns the listen and destination port for the kube-apiserver service on the load
// balancer. It falls back to sane defaults (ControlPlaneLoadBalancer.Port,
// ControlPlaneEndpoint.Port, 6443) instead of failing.
func kubeAPIPorts(hc *infrav1.HetznerCluster) (listenPort int, destinationPort int) {
	// Determine destination port (the port used on the control plane nodes).
	destinationPort = hc.Spec.ControlPlaneLoadBalancer.Port
	if destinationPort == 0 && hc.Spec.ControlPlaneEndpoint != nil {
		destinationPort = int(hc.Spec.ControlPlaneEndpoint.Port)
	}
	if destinationPort == 0 {
		destinationPort = infrav1.DefaultAPIServerPort
	}

	// Listen port is what clients hit on the load balancer.
	listenPort = destinationPort
	if hc.Spec.ControlPlaneEndpoint != nil && hc.Spec.ControlPlaneEndpoint.Port != 0 {
		listenPort = int(hc.Spec.ControlPlaneEndpoint.Port)
	}

	return listenPort, destinationPort
}

// Reconcile implements the life cycle of HCloud load balancers.
func (s *Service) Reconcile(ctx context.Context) (reconcile.Result, error) {
	// delete the deprecated condition from existing cluster objects
	conditions.Delete(s.scope.HetznerCluster, infrav1.DeprecatedLoadBalancerAttachedToNetworkCondition)

	if !s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled {
		return reconcile.Result{}, nil
	}

	log := s.scope.Logger.WithValues("reconciler", "load balancer")

	// find load balancer
	lb, err := s.findLoadBalancer(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to find load balancer: %w", err)
	}

	if lb == nil {
		if s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name != nil {
			// fixed name is set - we expect a load balancer with this name to exist
			lb, err = s.ownExistingLoadBalancer(ctx)
			if err != nil {
				// if load balancer is not found even though we expect it to exist, wait and reconcile until user creates it
				if errors.Is(err, ErrNoLoadBalancerAvailable) {
					return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
				}
				return reconcile.Result{}, fmt.Errorf("failed to own existing load balancer (name=%s): %w", *s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name, err)
			}
		} else {
			lb, err = s.createLoadBalancer(ctx)
			if err != nil {
				if errors.Is(err, ErrControlPlaneEndpointNotSet) {
					// When an external ControlPlane Provider gets used (Kamaji), it might
					// need some time until the endpoint is available.
					err = fmt.Errorf("requeue, waiting for control-plane endpoint to be set: %w",
						err)
					conditions.MarkFalse(
						s.scope.HetznerCluster,
						infrav1.LoadBalancerReadyCondition,
						"MissingControlPlaneEndpoint",
						clusterv1.ConditionSeverityWarning,
						"%s",
						err.Error(),
					)
					s.scope.Logger.Info(err.Error())
					return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
				}
				return reconcile.Result{}, fmt.Errorf("failed to create load balancer: %w", err)
			}
		}
	}

	s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = statusFromHCloudLB(lb, s.scope.HetznerCluster.Status.Network != nil, log)

	// check whether load balancer name, algorithm or type has been changed
	if err := s.reconcileLBProperties(ctx, lb); err != nil {
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.LoadBalancerUpdateFailedReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)
		return reconcile.Result{}, fmt.Errorf("failed to reconcile load balancer properties: %w", err)
	}

	if err := s.reconcileNetworkAttachement(ctx, lb); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile network attachment: %w", err)
	}

	if err := s.reconcileServices(ctx, lb); err != nil {
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.LoadBalancerServiceSyncFailedReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)
		return reconcile.Result{}, fmt.Errorf("failed to reconcile services: %w", err)
	}

	conditions.MarkTrue(s.scope.HetznerCluster, infrav1.LoadBalancerReadyCondition)
	return reconcile.Result{}, nil
}

func (s *Service) reconcileNetworkAttachement(ctx context.Context, lb *hcloud.LoadBalancer) error {
	// nothing to do if already attached to network
	if len(lb.PrivateNet) > 0 {
		return nil
	}

	// nothing to do if no network is specified
	if !s.scope.HetznerCluster.Spec.HCloudNetwork.Enabled {
		return nil
	}

	// attach load balancer to network
	if s.scope.HetznerCluster.Status.Network == nil {
		err := fmt.Errorf("no network found in object status")
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.NetworkAttachFailedReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)

		// no need to return error, as once the network is added it will be added to the status which triggers
		// another reconcile loop
		return nil
	}

	opts := hcloud.LoadBalancerAttachToNetworkOpts{
		Network: &hcloud.Network{ID: s.scope.HetznerCluster.Status.Network.ID},
	}

	if err := s.scope.HCloudClient.AttachLoadBalancerToNetwork(ctx, lb, opts); err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "AttachLoadBalancerToNetwork")

		// In case lb is already attached don't raise an error
		if hcloud.IsError(err, hcloud.ErrorCodeLoadBalancerAlreadyAttached) {
			return nil
		}

		err = fmt.Errorf("failed to attach load balancer to network: %w", err)

		record.Warnf(s.scope.HetznerCluster, "FailedAttachLoadBalancer", err.Error())
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.NetworkAttachFailedReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return err
	}

	return nil
}

func (s *Service) reconcileLBProperties(ctx context.Context, lb *hcloud.LoadBalancer) error {
	var multierr error
	lbSpec := s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer

	// check if type has been updated
	if lbSpec.Type != lb.LoadBalancerType.Name {
		opts := hcloud.LoadBalancerChangeTypeOpts{LoadBalancerType: &hcloud.LoadBalancerType{Name: lbSpec.Type}}
		if err := s.scope.HCloudClient.ChangeLoadBalancerType(ctx, lb, opts); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "ChangeLoadBalancerType")
			multierr = errors.Join(multierr, fmt.Errorf("failed to change load balancer type: %w", err))
		} else {
			record.Eventf(s.scope.HetznerCluster, "ChangeLoadBalancerType", "Changed load balancer type")
		}
	}

	// check if algorithm has been updated
	if string(lbSpec.Algorithm) != string(lb.Algorithm.Type) {
		opts := hcloud.LoadBalancerChangeAlgorithmOpts{Type: hcloud.LoadBalancerAlgorithmType(lbSpec.Algorithm)}
		if err := s.scope.HCloudClient.ChangeLoadBalancerAlgorithm(ctx, lb, opts); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "ChangeLoadBalancerAlgorithm")
			multierr = errors.Join(multierr, fmt.Errorf("failed to change load balancer algorithm: %w", err))
		} else {
			record.Eventf(s.scope.HetznerCluster, "ChangeLoadBalancerAlgorithm", "Changed load balancer algorithm")
		}
	}

	// check if name has been updated
	if lbSpec.Name != nil && *lbSpec.Name != lb.Name {
		opts := hcloud.LoadBalancerUpdateOpts{Name: *lbSpec.Name}
		if _, err := s.scope.HCloudClient.UpdateLoadBalancer(ctx, lb, opts); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "UpdateLoadBalancer")
			multierr = errors.Join(multierr, fmt.Errorf("failed to update load balancer name: %w", err))
		} else {
			record.Eventf(s.scope.HetznerCluster, "ChangeLoadBalancerName", "Changed load balancer name")
		}
	}

	return multierr
}

func (s *Service) reconcileServices(ctx context.Context, lb *hcloud.LoadBalancer) error {
	extraServicesSpec := s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices

	// build slices and maps to make diffs
	haveServiceListenPorts := make([]int, 0, len(lb.Services))
	wantServiceListenPorts := make([]int, 0, len(extraServicesSpec)+1)
	wantServiceListenPortsMap := make(map[int]infrav1.LoadBalancerServiceSpec, len(extraServicesSpec)+1)

	// filter kubeAPI service out
	for _, service := range lb.Services {
		haveServiceListenPorts = append(haveServiceListenPorts, service.ListenPort)
	}

	for _, serviceInSpec := range extraServicesSpec {
		wantServiceListenPorts = append(wantServiceListenPorts, serviceInSpec.ListenPort)
		wantServiceListenPortsMap[serviceInSpec.ListenPort] = serviceInSpec
	}

	// add kubeAPI service if exists
	kubeAPIServicePort, destinationPort := kubeAPIPorts(s.scope.HetznerCluster)
	wantServiceListenPorts = append(wantServiceListenPorts, kubeAPIServicePort)
	wantServiceListenPortsMap[kubeAPIServicePort] = infrav1.LoadBalancerServiceSpec{
		Protocol:        "tcp",
		ListenPort:      kubeAPIServicePort,
		DestinationPort: destinationPort,
	}

	toCreate, toDelete := utils.DifferenceOfIntSlices(wantServiceListenPorts, haveServiceListenPorts)

	// delete services which are registered for lb but are not in specs
	var multierr error

	for _, listenPort := range toDelete {
		if _, ok := wantServiceListenPortsMap[listenPort]; !ok {
			if err := s.scope.HCloudClient.DeleteServiceFromLoadBalancer(ctx, lb, listenPort); err != nil {
				// return immediately on rate limit
				hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeleteServiceFromLoadBalancer")
				multierr = errors.Join(multierr, fmt.Errorf("failed to delete service from load balancer: %w", err))
				if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
					return multierr
				}
			}
		}
	}

	// create services which are in specs and not yet in API
	for i, listenPort := range toCreate {
		proxyProtocol := false
		destinationPort := wantServiceListenPortsMap[listenPort].DestinationPort
		serviceOpts := hcloud.LoadBalancerAddServiceOpts{
			Protocol:        hcloud.LoadBalancerServiceProtocol(wantServiceListenPortsMap[listenPort].Protocol),
			ListenPort:      &toCreate[i],
			DestinationPort: &destinationPort,
			Proxyprotocol:   &proxyProtocol,
		}
		if err := s.scope.HCloudClient.AddServiceToLoadBalancer(ctx, lb, serviceOpts); err != nil {
			// return immediately on rate limit
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "AddServiceToLoadBalancer")
			multierr = errors.Join(multierr, fmt.Errorf("failed to add service to load balancer: %w", err))
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				return multierr
			}
		}
	}
	return multierr
}

func (s *Service) createLoadBalancer(ctx context.Context) (*hcloud.LoadBalancer, error) {
	opts, err := createOptsFromSpec(s.scope.HetznerCluster)
	if err != nil {
		return nil, err
	}
	lb, err := s.scope.HCloudClient.CreateLoadBalancer(ctx, opts)
	if err != nil {
		err = fmt.Errorf("failed to create load balancer: %w", err)
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "CreateLoadBalancer")
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.LoadBalancerCreateFailedReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		record.Warnf(s.scope.HetznerCluster, "FailedCreateLoadBalancer", err.Error())

		return nil, err
	}

	record.Eventf(s.scope.HetznerCluster, "CreateLoadBalancer", "Created load balancer")
	return lb, nil
}

func createOptsFromSpec(hc *infrav1.HetznerCluster) (hcloud.LoadBalancerCreateOpts, error) {
	// gather algorithm type
	algorithmType := hc.Spec.ControlPlaneLoadBalancer.Algorithm.HCloudAlgorithmType()

	listenPort, destinationPort := kubeAPIPorts(hc)

	// Set name
	name := utils.GenerateName(nil, fmt.Sprintf("%s-kube-apiserver-", hc.Name))

	proxyprotocol := false

	var network *hcloud.Network
	if hc.Status.Network != nil {
		network = &hcloud.Network{ID: hc.Status.Network.ID}
	}

	publicInterface := true
	return hcloud.LoadBalancerCreateOpts{
		LoadBalancerType: &hcloud.LoadBalancerType{Name: hc.Spec.ControlPlaneLoadBalancer.Type},
		Name:             name,
		Algorithm:        &hcloud.LoadBalancerAlgorithm{Type: algorithmType},
		Location:         &hcloud.Location{Name: string(hc.Spec.ControlPlaneLoadBalancer.Region)},
		Network:          network,
		Labels:           map[string]string{hc.ClusterTagKey(): string(infrav1.ResourceLifecycleOwned)},
		PublicInterface:  &publicInterface,
		Services: []hcloud.LoadBalancerCreateOptsService{
			{
				Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
				ListenPort:      &listenPort,
				DestinationPort: &destinationPort,
				Proxyprotocol:   &proxyprotocol,
			},
		},
	}, nil
}

// Delete implements the deletion of HCloud load balancers.
func (s *Service) Delete(ctx context.Context) (err error) {
	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		// nothing to do
		return nil
	}

	// do not delete a protected load balancer or one that has not been created by this controller
	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Protected || s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name != nil {
		lb, err := s.findLoadBalancer(ctx)
		if err != nil {
			return fmt.Errorf("failed to find load balancer: %w", err)
		}

		// nothing to do if load balancer is not found
		if lb == nil {
			return nil
		}

		// remove owned label and update
		delete(lb.Labels, s.scope.HetznerCluster.ClusterTagKey())

		if _, err := s.scope.HCloudClient.UpdateLoadBalancer(ctx, lb, hcloud.LoadBalancerUpdateOpts{Labels: lb.Labels}); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "UpdateLoadBalancer")
			err = fmt.Errorf("failed to update load balancer to remove the cluster label: %w", err)
			record.Warnf(s.scope.HetznerCluster, "FailedUpdateLoadBalancer", err.Error())
			conditions.MarkFalse(
				s.scope.HetznerCluster,
				infrav1.LoadBalancerReadyCondition,
				infrav1.LoadBalancerUpdateFailedReason,
				clusterv1.ConditionSeverityWarning,
				"%s",
				err.Error(),
			)
			return err
		}

		// Delete lb information from cluster status
		s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = nil

		record.Eventf(s.scope.HetznerCluster, "LoadBalancerOwnedLabelRemoved", "removed owned label of load balancer")
		return nil
	}

	if err := s.scope.HCloudClient.DeleteLoadBalancer(ctx, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID); err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeleteLoadBalancer")
		if hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			return nil
		}
		err = fmt.Errorf("failed to delete load balancer: %w", err)
		record.Warnf(s.scope.HetznerCluster, "FailedLoadBalancerDelete", err.Error())
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.LoadBalancerDeleteFailedReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)
		return err
	}

	// Delete lb information from cluster status
	s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = nil

	record.Eventf(s.scope.HetznerCluster, "DeleteLoadBalancer", "Deleted load balancer")
	return nil
}

func (s *Service) findLoadBalancer(ctx context.Context) (*hcloud.LoadBalancer, error) {
	clusterTagKey := s.scope.HetznerCluster.ClusterTagKey()
	opts := hcloud.LoadBalancerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: utils.LabelsToLabelSelector(map[string]string{
				clusterTagKey: string(infrav1.ResourceLifecycleOwned),
			}),
		},
	}
	loadBalancers, err := s.scope.HCloudClient.ListLoadBalancers(ctx, opts)
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "ListLoadBalancers")
		return nil, fmt.Errorf("failed to list load balancers: %w", err)
	}

	if len(loadBalancers) > 1 {
		return nil, fmt.Errorf("found %v loadbalancers in HCloud", len(loadBalancers))
	} else if len(loadBalancers) == 0 {
		return nil, nil
	}

	return loadBalancers[0], nil
}

func (s *Service) ownExistingLoadBalancer(ctx context.Context) (*hcloud.LoadBalancer, error) {
	name := *s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name
	loadBalancers, err := s.scope.HCloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{Name: name})
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "ListLoadBalancers")
		return nil, fmt.Errorf("failed to list load balancers: %w", err)
	}

	if len(loadBalancers) > 1 {
		return nil, fmt.Errorf("found %v load balancers in HCloud with name %q", len(loadBalancers), name)
	}

	if len(loadBalancers) == 0 {
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.LoadBalancerFailedToOwnReason,
			clusterv1.ConditionSeverityError,
			"%s",
			fmt.Sprintf("load balancer %q not found", name),
		)
		return nil, ErrNoLoadBalancerAvailable
	}

	lb := loadBalancers[0]

	for label := range lb.Labels {
		if strings.HasPrefix(label, infrav1.NameHetznerProviderOwned) {
			conditions.MarkFalse(
				s.scope.HetznerCluster,
				infrav1.LoadBalancerReadyCondition,
				infrav1.LoadBalancerFailedToOwnReason,
				clusterv1.ConditionSeverityError,
				"%s",
				fmt.Sprintf("load balancer %q already owned with label %q", name, label),
			)
			return nil, ErrNoLoadBalancerAvailable
		}
	}

	newLabels := make(map[string]string)
	for key, val := range lb.Labels {
		newLabels[key] = val
	}

	newLabels[s.scope.HetznerCluster.ClusterTagKey()] = string(infrav1.ResourceLifecycleOwned)

	lb, err = s.scope.HCloudClient.UpdateLoadBalancer(ctx, lb, hcloud.LoadBalancerUpdateOpts{Labels: newLabels})
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "UpdateLoadBalancer")
		err = fmt.Errorf("failed to update load balancer: %w", err)
		record.Warnf(s.scope.HetznerCluster, "FailedUpdateLoadBalancer", err.Error())
		conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.LoadBalancerFailedToOwnReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return nil, err
	}

	return lb, nil
}

// statusFromHCloudLB gets the information of the Hetzner load balancer and returns it in the status object.
func statusFromHCloudLB(lb *hcloud.LoadBalancer, hasNetwork bool, log logr.Logger) *infrav1.LoadBalancerStatus {
	var internalIP string
	if hasNetwork && len(lb.PrivateNet) > 0 {
		internalIP = lb.PrivateNet[0].IP.String()
	}

	targetObjects := make([]infrav1.LoadBalancerTarget, 0, len(lb.Targets))
	for _, target := range lb.Targets {
		switch target.Type {
		case hcloud.LoadBalancerTargetTypeServer:
			targetObjects = append(targetObjects, infrav1.LoadBalancerTarget{
				Type:     infrav1.LoadBalancerTargetTypeServer,
				ServerID: target.Server.Server.ID,
			},
			)
		case hcloud.LoadBalancerTargetTypeIP:
			targetObjects = append(targetObjects, infrav1.LoadBalancerTarget{
				Type: infrav1.LoadBalancerTargetTypeIP,
				IP:   target.IP.IP,
			},
			)
		default:
			log.Info("Unknown load balancer target type - will be ignored", "target type", target.Type)
		}
	}

	return &infrav1.LoadBalancerStatus{
		ID:         lb.ID,
		IPv4:       lb.PublicNet.IPv4.IP.String(),
		IPv6:       lb.PublicNet.IPv6.IP.String(),
		InternalIP: internalIP,
		Target:     targetObjects,
		Protected:  lb.Protection.Delete,
	}
}
