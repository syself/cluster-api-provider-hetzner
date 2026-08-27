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
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
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

// Reconcile implements the life cycle of HCloud load balancers.
func (s *Service) Reconcile(ctx context.Context) (reconcile.Result, error) {
	// delete the deprecated condition from existing cluster objects
	deprecatedv1beta1conditions.Delete(s.scope.HetznerCluster, infrav2.DeprecatedLoadBalancerAttachedToNetworkV1Beta1Condition)

	if !s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled {
		return reconcile.Result{}, nil
	}

	log := s.scope.WithValues("reconciler", "load balancer")

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
				return reconcile.Result{}, fmt.Errorf("failed to create load balancer: %w", err)
			}
		}
	}

	s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = statusFromHCloudLB(lb, s.scope.HetznerCluster.Status.Network != nil, int(s.scope.HetznerCluster.Spec.ControlPlaneEndpoint.Port), log)

	// check whether load balancer name, algorithm or type has been changed
	if err := s.reconcileLBProperties(ctx, lb); err != nil {
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.LoadBalancerUpdateFailedV1Beta1Reason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerUpdateFailedReason,
			Message: err.Error(),
		})

		return reconcile.Result{}, fmt.Errorf("failed to reconcile load balancer properties: %w", err)
	}

	if err := s.reconcileNetworkAttachement(ctx, lb); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile network attachment: %w", err)
	}

	if res, err := s.reconcileServices(ctx, lb); err != nil {
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.LoadBalancerServiceSyncFailedV1Beta1Reason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerSyncingServicesFailedReason,
			Message: err.Error(),
		})

		return reconcile.Result{}, fmt.Errorf("failed to reconcile services: %w", err)
	} else if res != (reconcile.Result{}) {
		return res, nil
	}

	deprecatedv1beta1conditions.MarkTrue(s.scope.HetznerCluster, infrav2.LoadBalancerReadyV1Beta1Condition)

	conditions.Set(s.scope.HetznerCluster, metav1.Condition{
		Type:   infrav2.HetznerClusterLoadBalancerReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: string(infrav2.HetznerClusterLoadBalancerReadyReason),
	})

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
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.NetworkAttachFailedV1Beta1Reason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerAttachingToNetworkFailedReason,
			Message: err.Error(),
		})

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
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.NetworkAttachFailedV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerAttachingToNetworkFailedReason,
			Message: err.Error(),
		})

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

func (s *Service) reconcileServices(ctx context.Context, lb *hcloud.LoadBalancer) (reconcile.Result, error) {
	extraServicesSpec := s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices

	wantServiceListenPorts := make([]int, 0, len(extraServicesSpec)+1)
	wantServiceListenPortsMap := make(map[int]infrav2.LoadBalancerServiceSpec, len(extraServicesSpec)+1)

	existingServicesByPort := make(map[int]hcloud.LoadBalancerService, len(lb.Services))
	for _, service := range lb.Services {
		existingServicesByPort[service.ListenPort] = service
	}

	kubeAPIServicePort := int(s.scope.HetznerCluster.Spec.ControlPlaneEndpoint.Port)

	for _, serviceInSpec := range extraServicesSpec {
		wantServiceListenPorts = append(wantServiceListenPorts, serviceInSpec.ListenPort)
		wantServiceListenPortsMap[serviceInSpec.ListenPort] = serviceInSpec
	}

	// add kubeAPI service if the endpoint port is known
	if kubeAPIServicePort != 0 {
		wantServiceListenPorts = append(wantServiceListenPorts, kubeAPIServicePort)
		wantServiceListenPortsMap[kubeAPIServicePort] = infrav2.LoadBalancerServiceSpec{
			Protocol:        "tcp",
			ListenPort:      kubeAPIServicePort,
			DestinationPort: s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Port,
		}
	}

	toCreate, toDelete := utils.DifferenceOfIntSlices(wantServiceListenPorts, slices.Collect(maps.Keys(existingServicesByPort)))

	// Two cases for the kube-API service:
	//   - present without proxy protocol → an existing cluster enabling it: wait until every
	//     control-plane infrastructure machine is annotated, then switch it on in place below.
	//   - absent → create it below from the spec value. The service is only absent when an
	//     existing load balancer is taken over instead of creating a new one, or if the service
	//     got manually deleted.
	existingKubeAPIService, kubeAPIServiceExists := existingServicesByPort[kubeAPIServicePort]
	proxyProtocolAlreadyActive := kubeAPIServiceExists && existingKubeAPIService.Proxyprotocol

	// proxyProtocolShouldGetEnabled: whether proxy protocol should get enabled now.
	// The control-plane infrastructure machines are only checked when the spec wants proxy protocol
	// but the LB service doesn't have it yet. When the service is absent or already has it, no check
	// is made.
	var proxyProtocolShouldGetEnabled bool
	var requeueForProxyProtocol bool
	if s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol && kubeAPIServiceExists && !proxyProtocolAlreadyActive {
		var err error
		proxyProtocolShouldGetEnabled, err = s.scope.AllControlPlaneInfraMachinesAnnotatedForProxyProtocol(ctx)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !proxyProtocolShouldGetEnabled {
			const msg = "waiting for all control-plane machines to be annotated before enabling proxy protocol"
			s.scope.V(1).Info("proxy protocol: not all control-plane infrastructure machines annotated yet, requeueing")
			requeueForProxyProtocol = true

			deprecatedv1beta1conditions.MarkFalse(
				s.scope.HetznerCluster,
				infrav2.LoadBalancerReadyV1Beta1Condition,
				infrav2.LoadBalancerWaitingToActivateProxyProtocolV1Beta1Reason,
				clusterv1.ConditionSeverityInfo,
				msg,
			)

			conditions.Set(s.scope.HetznerCluster, metav1.Condition{
				Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HetznerClusterLoadBalancerWaitingToActivateProxyProtocolReason,
				Message: msg,
			})
		}
	}

	// delete services that are no longer in the spec
	var multierr error

	for _, listenPort := range toDelete {
		if err := s.scope.HCloudClient.DeleteServiceFromLoadBalancer(ctx, lb, listenPort); err != nil {
			// return immediately on rate limit
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeleteServiceFromLoadBalancer")
			multierr = errors.Join(multierr, fmt.Errorf("failed to delete service from load balancer: %w", err))
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				return reconcile.Result{}, multierr
			}
		}
	}

	// create services that are in the spec but not yet on the LB
	for i, listenPort := range toCreate {
		proxyProtocol := false
		var healthCheck *hcloud.LoadBalancerAddServiceOptsHealthCheck
		destinationPort := wantServiceListenPortsMap[listenPort].DestinationPort
		if listenPort == kubeAPIServicePort {
			// Proxy protocol and health check are only relevant for the kube-API service, which
			// is created here straight from the spec value. The annotation check only guards
			// enabling proxy protocol or migrating the health check on a service that already exists.
			proxyProtocol = s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol
			healthCheck = healthCheckAddOpts(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.HealthCheck, destinationPort)
		}
		serviceOpts := hcloud.LoadBalancerAddServiceOpts{
			Protocol:        hcloud.LoadBalancerServiceProtocol(wantServiceListenPortsMap[listenPort].Protocol),
			ListenPort:      &toCreate[i],
			DestinationPort: &destinationPort,
			Proxyprotocol:   &proxyProtocol,
			HealthCheck:     healthCheck,
		}
		if err := s.scope.HCloudClient.AddServiceToLoadBalancer(ctx, lb, serviceOpts); err != nil {
			// return immediately on rate limit
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "AddServiceToLoadBalancer")
			multierr = errors.Join(multierr, fmt.Errorf("failed to add service to load balancer: %w", err))
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				return reconcile.Result{}, multierr
			}
		} else if listenPort == kubeAPIServicePort {
			// Status.ControlPlaneLoadBalancer was snapshotted from the LB state fetched at the
			// start of Reconcile, before this service was created, so it still shows the old
			// value. Update it now so callers observe the change in this reconcile instead of
			// waiting for the next one (e.g. the next full resync, up to --sync-period later).
			s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled = proxyProtocol
		}
	}

	if requeueForProxyProtocol {
		return reconcile.Result{RequeueAfter: 2 * time.Minute}, multierr
	}

	// If proxy protocol is not active yet but should be, activate it in place. HCloud's
	// update_service flips Proxyprotocol on the live service, so the kube-API service is
	// never absent from the LB.
	if proxyProtocolShouldGetEnabled && !proxyProtocolAlreadyActive {
		proxyProtocol := true
		updateOpts := hcloud.LoadBalancerUpdateServiceOpts{Proxyprotocol: &proxyProtocol}
		if err := s.scope.HCloudClient.UpdateServiceOnLoadBalancer(ctx, lb, kubeAPIServicePort, updateOpts); err != nil {
			// return immediately on rate limit
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "UpdateServiceOnLoadBalancer")
			multierr = errors.Join(multierr, fmt.Errorf("failed to update kube-API service on load balancer to enable proxy protocol: %w", err))
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				return reconcile.Result{}, multierr
			}
		} else {
			s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled = true
		}
	}

	// If the kube-API service already exists and its health check no longer matches the spec,
	// update it in place.
	kubeAPIDestinationPort := s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Port
	if wantHealthCheck := s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.HealthCheck; wantHealthCheck != nil &&
		kubeAPIServiceExists && healthCheckDiffers(existingKubeAPIService.HealthCheck, wantHealthCheck, kubeAPIDestinationPort) {
		// Switching a live service from a tcp check to an http or https check can mark every
		// backend unhealthy at once if the backends don't answer the path yet, which would take
		// the API server offline. So wait until every control-plane infra machine carries the
		// annotation, the same as the proxy-protocol migration: the annotation is set on the new
		// control-plane infrastructure machine template, so the switch happens only after every
		// control plane runs an image that serves the health-check path. A tcp check, or a
		// change that stays within http/https, is applied without the gate.
		if healthCheckMigratesToHTTP(existingKubeAPIService.HealthCheck, wantHealthCheck) {
			allReady, err := s.scope.AllControlPlaneInfraMachinesAnnotatedForHTTPHealthCheck(ctx)
			if err != nil {
				return reconcile.Result{}, errors.Join(multierr, err)
			}
			if !allReady {
				s.scope.V(1).Info("health check: not all control-plane infrastructure machines annotated yet, requeueing")
				return reconcile.Result{RequeueAfter: 10 * time.Second}, multierr
			}
		}

		updateOpts := hcloud.LoadBalancerUpdateServiceOpts{HealthCheck: healthCheckUpdateOpts(wantHealthCheck, kubeAPIDestinationPort)}
		if err := s.scope.HCloudClient.UpdateServiceOnLoadBalancer(ctx, lb, kubeAPIServicePort, updateOpts); err != nil {
			// return immediately on rate limit
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "UpdateServiceOnLoadBalancer")
			multierr = errors.Join(multierr, fmt.Errorf("failed to update kube-API service on load balancer to apply health check: %w", err))
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				return reconcile.Result{}, multierr
			}
		}
	}
	return reconcile.Result{}, multierr
}

func (s *Service) createLoadBalancer(ctx context.Context) (*hcloud.LoadBalancer, error) {
	opts := createOptsFromSpec(s.scope.HetznerCluster)
	lb, err := s.scope.HCloudClient.CreateLoadBalancer(ctx, opts)
	if err != nil {
		err = fmt.Errorf("failed to create load balancer: %w", err)
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "CreateLoadBalancer")
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.LoadBalancerCreateFailedV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerCreationFailedReason,
			Message: err.Error(),
		})

		record.Warnf(s.scope.HetznerCluster, "FailedCreateLoadBalancer", err.Error())

		return nil, err
	}

	record.Eventf(s.scope.HetznerCluster, "CreateLoadBalancer", "Created load balancer")
	return lb, nil
}

func healthCheckCreateOpts(hc *infrav2.LoadBalancerHealthCheckSpec, servicePort int) *hcloud.LoadBalancerCreateOptsServiceHealthCheck {
	f := healthCheckOptsFromSpec(hc, servicePort)
	if f == nil {
		return nil
	}

	opts := &hcloud.LoadBalancerCreateOptsServiceHealthCheck{
		Protocol: f.Protocol,
		Port:     f.Port,
		Interval: f.Interval,
		Timeout:  f.Timeout,
		Retries:  f.Retries,
	}
	if f.TLS != nil {
		opts.HTTP = &hcloud.LoadBalancerCreateOptsServiceHealthCheckHTTP{
			Domain:      f.Domain,
			Path:        f.Path,
			Response:    f.Response,
			StatusCodes: f.StatusCodes,
			TLS:         f.TLS,
		}
	}
	return opts
}

func healthCheckAddOpts(hc *infrav2.LoadBalancerHealthCheckSpec, servicePort int) *hcloud.LoadBalancerAddServiceOptsHealthCheck {
	f := healthCheckOptsFromSpec(hc, servicePort)
	if f == nil {
		return nil
	}

	opts := &hcloud.LoadBalancerAddServiceOptsHealthCheck{
		Protocol: f.Protocol,
		Port:     f.Port,
		Interval: f.Interval,
		Timeout:  f.Timeout,
		Retries:  f.Retries,
	}
	if f.TLS != nil {
		opts.HTTP = &hcloud.LoadBalancerAddServiceOptsHealthCheckHTTP{
			Domain:      f.Domain,
			Path:        f.Path,
			Response:    f.Response,
			StatusCodes: f.StatusCodes,
			TLS:         f.TLS,
		}
	}
	return opts
}

func healthCheckUpdateOpts(hc *infrav2.LoadBalancerHealthCheckSpec, servicePort int) *hcloud.LoadBalancerUpdateServiceOptsHealthCheck {
	f := healthCheckOptsFromSpec(hc, servicePort)
	if f == nil {
		return nil
	}

	opts := &hcloud.LoadBalancerUpdateServiceOptsHealthCheck{
		Protocol: f.Protocol,
		Port:     f.Port,
		Interval: f.Interval,
		Timeout:  f.Timeout,
		Retries:  f.Retries,
	}
	if f.TLS != nil {
		opts.HTTP = &hcloud.LoadBalancerUpdateServiceOptsHealthCheckHTTP{
			Domain:      f.Domain,
			Path:        f.Path,
			Response:    f.Response,
			StatusCodes: f.StatusCodes,
			TLS:         f.TLS,
		}
	}
	return opts
}

// healthCheckOpts holds the fields shared by the three structurally-identical hcloud
// health-check options types used for creating, adding and updating a load balancer service.
// Go doesn't let one type be reused across all three request builders, so this is converted
// into each of them by healthCheckCreateOpts, healthCheckAddOpts and healthCheckUpdateOpts.
// servicePort is the fallback Port for the health check when the spec doesn't set one.
type healthCheckOpts struct {
	Protocol    hcloud.LoadBalancerServiceProtocol
	Port        *int
	Interval    *time.Duration
	Timeout     *time.Duration
	Retries     *int
	Domain      *string
	Path        *string
	Response    *string
	StatusCodes []string
	TLS         *bool
}

func healthCheckOptsFromSpec(hc *infrav2.LoadBalancerHealthCheckSpec, servicePort int) *healthCheckOpts {
	if hc == nil {
		return nil
	}

	protocol := hcloud.LoadBalancerServiceProtocolTCP
	if hc.Protocol != "" {
		protocol = hcloud.LoadBalancerServiceProtocol(hc.Protocol)
	}

	port := servicePort
	if hc.Port != nil {
		port = *hc.Port
	}

	opts := &healthCheckOpts{
		Protocol: protocol,
		Port:     &port,
		Retries:  hc.Retries,
	}
	if hc.IntervalSeconds != nil {
		interval := time.Duration(*hc.IntervalSeconds) * time.Second
		opts.Interval = &interval
	}
	if hc.TimeoutSeconds != nil {
		timeout := time.Duration(*hc.TimeoutSeconds) * time.Second
		opts.Timeout = &timeout
	}

	if protocol == hcloud.LoadBalancerServiceProtocolHTTP || protocol == hcloud.LoadBalancerServiceProtocolHTTPS {
		opts.Domain = hc.Domain
		opts.Path = hc.Path
		opts.Response = hc.Response
		opts.StatusCodes = hc.StatusCodes
		tls := protocol == hcloud.LoadBalancerServiceProtocolHTTPS
		opts.TLS = &tls
	}

	return opts
}

// isHTTPHealthCheckProtocol reports whether protocol is one that sends an HTTP(S) request,
// as opposed to a plain tcp check.
func isHTTPHealthCheckProtocol(protocol hcloud.LoadBalancerServiceProtocol) bool {
	return protocol == hcloud.LoadBalancerServiceProtocolHTTP || protocol == hcloud.LoadBalancerServiceProtocolHTTPS
}

// healthCheckDiffers reports whether the load balancer's observed health check differs from the
// fields set in desired. Fields left unset in desired are not compared, since CAPH only manages
// the sub-fields the user explicitly configured and otherwise leaves Hetzner's behavior alone.
// servicePort is the default Port when desired.Port is unset.
func healthCheckDiffers(observed hcloud.LoadBalancerServiceHealthCheck, desired *infrav2.LoadBalancerHealthCheckSpec, servicePort int) bool {
	if desired == nil {
		return false
	}

	desiredProtocol := hcloud.LoadBalancerServiceProtocolTCP
	if desired.Protocol != "" {
		desiredProtocol = hcloud.LoadBalancerServiceProtocol(desired.Protocol)
	}
	if desiredProtocol != observed.Protocol {
		return true
	}

	desiredPort := servicePort
	if desired.Port != nil {
		desiredPort = *desired.Port
	}
	if desiredPort != observed.Port {
		return true
	}

	if desired.IntervalSeconds != nil && time.Duration(*desired.IntervalSeconds)*time.Second != observed.Interval {
		return true
	}
	if desired.TimeoutSeconds != nil && time.Duration(*desired.TimeoutSeconds)*time.Second != observed.Timeout {
		return true
	}
	if desired.Retries != nil && *desired.Retries != observed.Retries {
		return true
	}

	if !isHTTPHealthCheckProtocol(desiredProtocol) {
		return false
	}

	desiredTLS := desiredProtocol == hcloud.LoadBalancerServiceProtocolHTTPS
	if observed.HTTP == nil || observed.HTTP.TLS != desiredTLS {
		return true
	}
	if desired.Domain != nil && *desired.Domain != observed.HTTP.Domain {
		return true
	}
	if desired.Path != nil && *desired.Path != observed.HTTP.Path {
		return true
	}
	if desired.Response != nil && *desired.Response != observed.HTTP.Response {
		return true
	}
	// Status codes are a set, so compare them sorted. Otherwise the load balancer reporting them
	// back in another order would count as a change and this would call the API every reconcile.
	if len(desired.StatusCodes) > 0 && !slices.Equal(slices.Sorted(slices.Values(observed.HTTP.StatusCodes)), slices.Sorted(slices.Values(desired.StatusCodes))) {
		return true
	}

	return false
}

// healthCheckMigratesToHTTP reports whether applying want to a service that currently has the
// got check switches it from a non-http check (the default tcp) to an http or https check. That
// switch can mark a target that does not yet answer the path as unhealthy, so the caller waits
// for the control-plane rollout via AllControlPlaneInfraMachinesAnnotatedForHTTPHealthCheck before
// applying it. It compares the live check every time, so this is true on every such switch, not
// only the first one. A change that stays within http/https, or a switch back to tcp, returns
// false, as does a nil want.
func healthCheckMigratesToHTTP(got hcloud.LoadBalancerServiceHealthCheck, want *infrav2.LoadBalancerHealthCheckSpec) bool {
	if want == nil {
		return false
	}
	wantProtocol := hcloud.LoadBalancerServiceProtocolTCP
	if want.Protocol != "" {
		wantProtocol = hcloud.LoadBalancerServiceProtocol(want.Protocol)
	}
	return isHTTPHealthCheckProtocol(wantProtocol) && !isHTTPHealthCheckProtocol(got.Protocol)
}

func createOptsFromSpec(hc *infrav2.HetznerCluster) hcloud.LoadBalancerCreateOpts {
	// gather algorithm type
	algorithmType := hc.Spec.ControlPlaneLoadBalancer.Algorithm.HCloudAlgorithmType()

	// Set name
	name := utils.GenerateName(nil, fmt.Sprintf("%s-kube-apiserver-", hc.Name))

	proxyprotocol := hc.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol

	var network *hcloud.Network
	if hc.Status.Network != nil {
		network = &hcloud.Network{ID: hc.Status.Network.ID}
	}

	// The listen port mirrors spec.controlPlaneEndpoint.Port. It can be 0 here on the first reconcile
	// (the control plane endpoint is only filled in from the load balancer IP afterwards in
	// processControlPlaneEndpoint); reconcileLBProperties corrects the listen port on the next pass.
	listenPort := int(hc.Spec.ControlPlaneEndpoint.Port)
	publicInterface := true
	return hcloud.LoadBalancerCreateOpts{
		LoadBalancerType: &hcloud.LoadBalancerType{Name: hc.Spec.ControlPlaneLoadBalancer.Type},
		Name:             name,
		Algorithm:        &hcloud.LoadBalancerAlgorithm{Type: algorithmType},
		Location:         &hcloud.Location{Name: string(hc.Spec.ControlPlaneLoadBalancer.Region)},
		Network:          network,
		Labels:           map[string]string{hc.ClusterTagKey(): string(infrav2.ResourceLifecycleOwned)},
		PublicInterface:  &publicInterface,
		Services: []hcloud.LoadBalancerCreateOptsService{
			{
				Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
				ListenPort:      &listenPort,
				DestinationPort: &hc.Spec.ControlPlaneLoadBalancer.Port,
				Proxyprotocol:   &proxyprotocol,
				HealthCheck:     healthCheckCreateOpts(hc.Spec.ControlPlaneLoadBalancer.HealthCheck, hc.Spec.ControlPlaneLoadBalancer.Port),
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
			deprecatedv1beta1conditions.MarkFalse(
				s.scope.HetznerCluster,
				infrav2.LoadBalancerReadyV1Beta1Condition,
				infrav2.LoadBalancerUpdateFailedV1Beta1Reason,
				clusterv1.ConditionSeverityWarning,
				"%s",
				err.Error(),
			)

			conditions.Set(s.scope.HetznerCluster, metav1.Condition{
				Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HetznerClusterLoadBalancerUpdateFailedReason,
				Message: err.Error(),
			})

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
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.LoadBalancerDeleteFailedV1Beta1Reason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerDeletionFailedReason,
			Message: err.Error(),
		})

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
				clusterTagKey: string(infrav2.ResourceLifecycleOwned),
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
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.LoadBalancerFailedToOwnV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"%s",
			fmt.Sprintf("load balancer %q not found", name),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerOwningFailedReason,
			Message: fmt.Sprintf("load balancer %q not found", name),
		})

		return nil, ErrNoLoadBalancerAvailable
	}

	lb := loadBalancers[0]

	for label := range lb.Labels {
		if strings.HasPrefix(label, infrav2.NameHetznerProviderOwned) {
			deprecatedv1beta1conditions.MarkFalse(
				s.scope.HetznerCluster,
				infrav2.LoadBalancerReadyV1Beta1Condition,
				infrav2.LoadBalancerFailedToOwnV1Beta1Reason,
				clusterv1.ConditionSeverityError,
				"%s",
				fmt.Sprintf("load balancer %q already owned with label %q", name, label),
			)

			conditions.Set(s.scope.HetznerCluster, metav1.Condition{
				Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HetznerClusterLoadBalancerOwningFailedReason,
				Message: fmt.Sprintf("load balancer %q already owned with label %q", name, label),
			})

			return nil, ErrNoLoadBalancerAvailable
		}
	}

	newLabels := make(map[string]string)
	for key, val := range lb.Labels {
		newLabels[key] = val
	}

	newLabels[s.scope.HetznerCluster.ClusterTagKey()] = string(infrav2.ResourceLifecycleOwned)

	lb, err = s.scope.HCloudClient.UpdateLoadBalancer(ctx, lb, hcloud.LoadBalancerUpdateOpts{Labels: newLabels})
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "UpdateLoadBalancer")
		err = fmt.Errorf("failed to update load balancer: %w", err)
		record.Warnf(s.scope.HetznerCluster, "FailedUpdateLoadBalancer", err.Error())
		deprecatedv1beta1conditions.MarkFalse(
			s.scope.HetznerCluster,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.LoadBalancerFailedToOwnV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)

		conditions.Set(s.scope.HetznerCluster, metav1.Condition{
			Type:    infrav2.HetznerClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerClusterLoadBalancerOwningFailedReason,
			Message: err.Error(),
		})

		return nil, err
	}

	return lb, nil
}

// statusFromHCloudLB gets the information of the Hetzner load balancer and returns it in the status object.
func statusFromHCloudLB(lb *hcloud.LoadBalancer, hasNetwork bool, kubeAPIServicePort int, log logr.Logger) *infrav2.LoadBalancerStatus {
	var internalIP string
	if hasNetwork && len(lb.PrivateNet) > 0 {
		internalIP = ipToStatusString(lb.PrivateNet[0].IP)
	}

	targetObjects := make([]infrav2.LoadBalancerTarget, 0, len(lb.Targets))
	for _, target := range lb.Targets {
		switch target.Type {
		case hcloud.LoadBalancerTargetTypeServer:
			targetObjects = append(targetObjects, infrav2.LoadBalancerTarget{
				Type:     infrav2.LoadBalancerTargetTypeServer,
				ServerID: target.Server.Server.ID,
			},
			)
		case hcloud.LoadBalancerTargetTypeIP:
			targetObjects = append(targetObjects, infrav2.LoadBalancerTarget{
				Type: infrav2.LoadBalancerTargetTypeIP,
				IP:   target.IP.IP,
			},
			)
		default:
			log.Info("Unknown load balancer target type - will be ignored", "target type", target.Type)
		}
	}

	var proxyProtocolEnabled bool
	for _, service := range lb.Services {
		if service.ListenPort == kubeAPIServicePort {
			proxyProtocolEnabled = service.Proxyprotocol
			break
		}
	}

	return &infrav2.LoadBalancerStatus{
		ID:                   lb.ID,
		IPv4:                 ipToStatusString(lb.PublicNet.IPv4.IP),
		IPv6:                 ipToStatusString(lb.PublicNet.IPv6.IP),
		InternalIP:           internalIP,
		Target:               targetObjects,
		Protected:            lb.Protection.Delete,
		ProxyProtocolEnabled: proxyProtocolEnabled,
	}
}

// ipToStatusString returns the IP as string, or an empty string if the IP is not set.
// A load balancer without a public interface has no public IPv4 and no public IPv6.
// Calling String() on such an IP returns "<nil>", which is not a valid IP address.
// The same is true for an unset private IP.
func ipToStatusString(ip net.IP) string {
	if len(ip) == 0 || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}
