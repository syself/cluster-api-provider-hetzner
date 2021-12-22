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

// Package server implements functions to manage the lifecycle of HCloud servers
package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const maxShutDownTime = 2 * time.Minute

// Service defines struct with machine scope to reconcile HCloud machines.
type Service struct {
	scope *scope.MachineScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.MachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HCloud machines.
func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {

	// If no token information has been given, the server cannot be successfully reconciled
	if s.scope.HetznerCluster.Spec.HetznerSecret.Key.HCloudToken == "" {
		record.Eventf(s.scope.HCloudMachine, corev1.EventTypeWarning, "NoTokenFound", "No HCloudToken found")
		return nil, fmt.Errorf("no token for HCloud provided - cannot reconcile hcloud server")
	}

	// detect failure domain
	failureDomain, err := s.scope.GetFailureDomain()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get failure domain")
	}
	s.scope.HCloudMachine.Status.Region = infrav1.Region(failureDomain)

	// Waiting for bootstrap data to be ready
	if !s.scope.IsBootstrapDataReady(ctx) {
		return &ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Try to find an existing server
	server, err := s.findServer(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get server")
	}
	// If no server is found we have to create one
	if server == nil {
		server, err = s.createServer(ctx, failureDomain)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create server")
		}
		record.Eventf(
			s.scope.HCloudMachine,
			"SuccessfulCreate",
			"Created new server with id %d",
			server.ID,
		)
	}

	s.scope.HCloudMachine.Status = setStatusFromAPI(server)

	switch server.Status {
	case hcloud.ServerStatusOff:
		// Check if server is in ServerStatusOff and turn it on. This is to avoid a bug of Hetzner where
		// sometimes machines are created and not turned on
		// Don't do this when server is just created
		if _, _, err := s.scope.HCloudClient().PowerOnServer(ctx, server); err != nil {
			return nil, errors.Wrap(err, "failed to power on server")
		}

	case hcloud.ServerStatusRunning: // Do nothing
	default:
		s.scope.HCloudMachine.Status.Ready = false
		s.scope.V(1).Info("server not in running state", "server", server.Name, "status", server.Status)
		return &reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	}

	// Check whether server is attached to the network
	if err := s.reconcileNetworkAttachment(ctx, server); err != nil {
		return nil, errors.Wrap(err, "failed to reconcile network attachement")
	}

	providerID := fmt.Sprintf("hcloud://%d", server.ID)

	if !s.scope.IsControlPlane() {
		s.scope.HCloudMachine.Spec.ProviderID = &providerID
		s.scope.HCloudMachine.Status.Ready = true
		conditions.MarkTrue(s.scope.HCloudMachine, infrav1.InstanceReadyCondition)
		return nil, nil
	}

	// all control planes have to be attached to the load balancer
	if err := s.reconcileLoadBalancerAttachment(ctx, server); err != nil {
		return nil, errors.Wrap(err, "failed to reconcile load balancer attachement")
	}

	if err := s.isControlPlaneReady(ctx); err != nil {
		record.Warnf(
			s.scope.HCloudMachine,
			"APIServerNotReady",
			"Health check for API server failed: %s",
			err,
		)
		return nil, errors.Wrap(err, "control plane not ready - no usable address found")
	}

	s.scope.HCloudMachine.Spec.ProviderID = &providerID
	s.scope.HCloudMachine.Status.Ready = true
	conditions.MarkTrue(s.scope.HCloudMachine, infrav1.InstanceReadyCondition)

	return nil, nil
}

// Checks if control-plane is ready. Loops through available control-planes,
// rewrites kubeconfig address to address of control-plane.
// Requests readyz endpoint of control-plane kube-apiserver.
func (s *Service) isControlPlaneReady(ctx context.Context) error {
	var multierr []error
	for _, address := range s.scope.HCloudMachine.Status.Addresses {
		if address.Type != corev1.NodeExternalIP && address.Type != corev1.NodeExternalDNS {
			continue
		}

		clientConfig, err := s.scope.ClientConfigWithAPIEndpoint(ctx, clusterv1.APIEndpoint{
			Host: address.Address,
			Port: s.scope.ControlPlaneAPIEndpointPort(),
		})
		if err != nil {
			multierr = append(multierr, errors.Wrap(err, "failed to get client config with API endpoint"))
			break
		}

		if err := scope.IsControlPlaneReady(ctx, clientConfig); err != nil {
			multierr = append(multierr, err)
			break
		}
		return nil
	}
	return kerrors.NewAggregate(multierr)
}

func (s *Service) reconcileNetworkAttachment(ctx context.Context, server *hcloud.Server) error {
	// If no network exists, then do nothing
	if s.scope.HetznerCluster.Status.Network == nil {
		return nil
	}

	// Already attached to network - nothing to do
	for _, id := range s.scope.HetznerCluster.Status.Network.AttachedServers {
		if id == server.ID {
			return nil
		}
	}

	// Attach server to network
	if _, _, err := s.scope.HCloudClient().AttachServerToNetwork(ctx, server, hcloud.ServerAttachToNetworkOpts{
		Network: &hcloud.Network{
			ID: s.scope.HetznerCluster.Status.Network.ID,
		},
	}); err != nil {
		// Check if network status is old and server is in fact already attached
		if hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAttached) {
			return nil
		}
		return errors.Wrap(err, "failed to attach server to network")
	}

	return nil
}

func (s *Service) createServer(ctx context.Context, failureDomain string) (*hcloud.Server, error) {
	// get userData
	userData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		record.Warnf(
			s.scope.HCloudMachine,
			"FailedGetBootstrapData",
			err.Error(),
		)
		return nil, fmt.Errorf("failed to get raw bootstrap data: %s", err)
	}

	image, err := s.getServerImage(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get server image")
	}

	name := s.scope.Name()
	automount := false
	startAfterCreate := true
	opts := hcloud.ServerCreateOpts{
		Name:   name,
		Labels: s.createLabels(),
		Image:  image,
		Location: &hcloud.Location{
			Name: failureDomain,
		},
		ServerType: &hcloud.ServerType{
			Name: string(s.scope.HCloudMachine.Spec.Type),
		},
		Automount:        &automount,
		StartAfterCreate: &startAfterCreate,
		UserData:         string(userData),
	}

	// set placement group if necessary
	if s.scope.HCloudMachine.Spec.PlacementGroupName != nil {
		var foundPlacementGroupInStatus bool
		for _, pgSts := range s.scope.HetznerCluster.Status.HCloudPlacementGroup {
			if *s.scope.HCloudMachine.Spec.PlacementGroupName == pgSts.Name {
				foundPlacementGroupInStatus = true
				opts.PlacementGroup = &hcloud.PlacementGroup{
					ID:   pgSts.ID,
					Name: pgSts.Name,
					Type: hcloud.PlacementGroupType(pgSts.Type),
				}
			}
		}
		if !foundPlacementGroupInStatus {
			log.Info("No placement group found on server creation", "placementGroupName", *s.scope.HCloudMachine.Spec.PlacementGroupName)
			return nil, errors.New("failed to find server's placement group")
		}
	}

	sshKeys, err := s.getSSHKeys(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ssh keys")
	}
	opts.SSHKeys = sshKeys

	// set up network if available
	if net := s.scope.HetznerCluster.Status.Network; net != nil {
		opts.Networks = []*hcloud.Network{{
			ID: net.ID,
		}}
	}

	// Create the server
	res, _, err := s.scope.HCloudClient().CreateServer(ctx, opts)
	if err != nil {
		record.Warnf(s.scope.HCloudMachine,
			"FailedCreateHCloudServer",
			"Failed to create HCloud server %s: %s",
			s.scope.Name(),
			err,
		)
		return nil, fmt.Errorf("error while creating HCloud server %s: %s", s.scope.HCloudMachine.Name, err)
	}

	return res.Server, nil
}

func (s *Service) getServerImage(ctx context.Context) (*hcloud.Image, error) {
	key := fmt.Sprintf("%s%s", infrav1.NameHetznerProviderPrefix, "image-name")

	// query for an existing image by label this is needed because snapshots doesn't have any name only descriptions and labels.
	listOpts := hcloud.ImageListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: fmt.Sprintf("%s==%s", key, s.scope.HCloudMachine.Spec.ImageName),
		},
	}

	imagesByLabel, err := s.scope.HCloudClient().ListImages(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	// query for an existing image by name.
	listOpts = hcloud.ImageListOpts{
		Name: s.scope.HCloudMachine.Spec.ImageName,
	}
	imagesByName, err := s.scope.HCloudClient().ListImages(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	images := append(imagesByLabel, imagesByName...)

	if len(images) > 1 {
		record.Warnf(s.scope.HCloudMachine,
			"ImageNameAmbiguous",
			"%v images have name %s",
			len(images),
			s.scope.HCloudMachine.Spec.ImageName,
		)
		return nil, fmt.Errorf("image name is ambiguous. %v images have name %s",
			len(images), s.scope.HCloudMachine.Spec.ImageName)
	}
	if len(images) == 0 {
		record.Warnf(s.scope.HCloudMachine,
			"ImageNotFound",
			"No image found with name %s",
			s.scope.HCloudMachine.Spec.ImageName,
		)
		return nil, fmt.Errorf("no image found with name %s", s.scope.HCloudMachine.Spec.ImageName)
	}

	return images[0], nil
}

func (s *Service) getSSHKeys(ctx context.Context) ([]*hcloud.SSHKey, error) {
	sshKeySpecs := s.scope.HCloudMachine.Spec.SSHKeys
	if len(sshKeySpecs) == 0 {
		sshKeySpecs = s.scope.HetznerCluster.Spec.SSHKeys
	}
	sshKeys, _, err := s.scope.HCloudClient().ListSSHKeys(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return nil, errors.Wrap(err, "failed listing ssh heys from hcloud")
	}
	resp := make([]*hcloud.SSHKey, 0, len(sshKeys))
	for _, sshKey := range sshKeys {
		var match bool
		for _, sshKeySpec := range sshKeySpecs {
			if sshKeySpec.Name != nil && *sshKeySpec.Name == sshKey.Name {
				match = true
			}
			if sshKeySpec.ID != nil && *sshKeySpec.ID == sshKey.ID {
				match = true
			}
		}
		if match {
			resp = append(resp, sshKey)
		}
	}
	return resp, nil
}

// Delete implements delete method of server.
func (s *Service) Delete(ctx context.Context) (_ *ctrl.Result, err error) {
	// find current server
	server, err := s.findServer(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Server")
	}

	// If no server has been found then nothing can be deleted
	if server == nil {
		s.scope.V(2).Info("Unable to locate HCloud server by ID or tags")
		record.Warnf(s.scope.HCloudMachine, "NoInstanceFound", "Unable to find matching HCloud server for %s", s.scope.Name())
		return nil, nil
	}

	if s.scope.IsControlPlane() {
		if err := s.deleteServerOfLoadBalancer(ctx, server); err != nil {
			return &reconcile.Result{}, errors.Errorf("Error while deleting attached server of loadbalancer: %s", err)
		}
	}

	// First shut the server down, then delete it
	switch status := server.Status; status {
	case hcloud.ServerStatusRunning:
		// Check if the server has been tried to shut down already and if so,
		// if time of last condition change + maxWaitTime is already in the past.
		// With one of these two conditions true, delete server immediately. Otherwise, shut it down and requeue.
		if conditions.IsTrue(s.scope.HCloudMachine, infrav1.InstanceReadyCondition) ||
			conditions.IsFalse(s.scope.HCloudMachine, infrav1.InstanceReadyCondition) &&
				conditions.GetReason(s.scope.HCloudMachine, infrav1.InstanceReadyCondition) == infrav1.InstanceTerminatedReason &&
				time.Now().Before(conditions.GetLastTransitionTime(s.scope.HCloudMachine, infrav1.InstanceReadyCondition).Time.Add(maxShutDownTime)) {
			if _, _, err := s.scope.HCloudClient().ShutdownServer(ctx, server); err != nil {
				return &reconcile.Result{}, errors.Wrap(err, "failed to shutdown server")
			}
			conditions.MarkFalse(s.scope.HCloudMachine,
				infrav1.InstanceReadyCondition,
				infrav1.InstanceTerminatedReason,
				clusterv1.ConditionSeverityInfo,
				"Instance has been shut down")
			return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if _, err := s.scope.HCloudClient().DeleteServer(ctx, server); err != nil {
			record.Warnf(s.scope.HCloudMachine, "FailedDeleteHCloudServer", "Failed to delete HCloud server %s", s.scope.Name())
			return &reconcile.Result{}, errors.Wrap(err, "failed to delete server")
		}

	case hcloud.ServerStatusOff:
		if _, err := s.scope.HCloudClient().DeleteServer(ctx, server); err != nil {
			record.Warnf(s.scope.HCloudMachine, "FailedDeleteHCloudServer", "Failed to delete HCloud server %s", s.scope.Name())
			return &reconcile.Result{}, errors.Wrap(err, "failed to delete server")
		}

	default:
		return &ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	record.Eventf(
		s.scope.HCloudMachine,
		"HCloudServerDeleted",
		"HCloud server %s deleted",
		s.scope.Name(),
	)
	return nil, nil
}

func setStatusFromAPI(server *hcloud.Server) infrav1.HCloudMachineStatus {
	var status infrav1.HCloudMachineStatus
	s := server.Status
	status.InstanceState = &s
	status.Addresses = []corev1.NodeAddress{}

	if ip := server.PublicNet.IPv4.IP.String(); ip != "" {
		status.Addresses = append(
			status.Addresses,
			corev1.NodeAddress{
				Type:    corev1.NodeExternalIP,
				Address: ip,
			},
		)
	}

	if ip := server.PublicNet.IPv6.IP; ip.IsGlobalUnicast() {
		ip[15]++
		status.Addresses = append(
			status.Addresses,
			corev1.NodeAddress{
				Type:    corev1.NodeExternalIP,
				Address: ip.String(),
			},
		)
	}

	for _, net := range server.PrivateNet {
		status.Addresses = append(
			status.Addresses,
			corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: net.IP.String(),
			},
		)
	}

	return status
}

func (s *Service) reconcileLoadBalancerAttachment(ctx context.Context, server *hcloud.Server) error {
	log := ctrl.LoggerFrom(ctx)

	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		return nil
	}

	// If already attached do nothing
	for _, id := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
		if id == server.ID {
			return nil
		}
	}

	log.V(1).Info("Reconciling load balancer attachement", "targets", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target)

	// We differentiate between private and public net
	var hasPrivateIP bool
	if len(server.PrivateNet) > 0 {
		hasPrivateIP = true
	}
	// If load balancer has not been attached to a network, then it cannot add a server with private IP
	if hasPrivateIP && conditions.IsFalse(s.scope.HetznerCluster, infrav1.LoadBalancerAttachedToNetworkCondition) {
		return nil
	}

	loadBalancerAddServerTargetOpts := hcloud.LoadBalancerAddServerTargetOpts{
		Server:       server,
		UsePrivateIP: &hasPrivateIP,
	}

	if _, _, err := s.scope.HCloudClient().AddTargetServerToLoadBalancer(
		ctx,
		loadBalancerAddServerTargetOpts,
		&hcloud.LoadBalancer{
			ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
		}); err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeTargetAlreadyDefined) {
			return nil
		}
		s.scope.V(1).Info("Could not add server as target to load balancer",
			"Server", server.ID, "Load Balancer", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
		return err
	}

	record.Eventf(
		s.scope.HetznerCluster,
		"AddedAsTargetToLoadBalancer",
		"Added new server with id %d to the loadbalancer %v",
		server.ID, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)

	return nil
}

func (s *Service) deleteServerOfLoadBalancer(ctx context.Context, server *hcloud.Server) error {
	if _, _, err := s.scope.HCloudClient().DeleteTargetServerOfLoadBalancer(
		ctx,
		&hcloud.LoadBalancer{
			ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
		},
		server); err != nil {
		if strings.Contains(err.Error(), "load_balancer_target_not_found") {
			return nil
		}
		s.scope.V(1).Info("Could not delete server as target of load balancer",
			"Server", server.ID, "Load Balancer", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
		return err
	}
	record.Eventf(
		s.scope.HetznerCluster,
		"DeletedTargetOfLoadBalancer",
		"Deleted new server with id %d of the loadbalancer %v",
		server.ID, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)

	return nil
}

// We write the server name in the labels, so that all labels are or should be unique.
func (s *Service) findServer(ctx context.Context) (*hcloud.Server, error) {
	opts := hcloud.ServerListOpts{}
	opts.LabelSelector = utils.LabelsToLabelSelector(s.createLabels())
	servers, err := s.scope.HCloudClient().ListServers(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(servers) > 1 {
		record.Warnf(s.scope.HCloudMachine,
			"MultipleInstances",
			"Found %v servers of name %s",
			len(servers),
			s.scope.Name())
		return nil, fmt.Errorf("found %v servers with name %s", len(servers), s.scope.Name())
	} else if len(servers) == 0 {
		return nil, nil
	}

	return servers[0], nil
}

func (s *Service) createLabels() map[string]string {
	m := map[string]string{
		infrav1.ClusterTagKey(s.scope.HetznerCluster.Name): string(infrav1.ResourceLifecycleOwned),
		infrav1.MachineNameTagKey:                          s.scope.Name(),
	}

	var machineType string
	if s.scope.IsControlPlane() {
		machineType = "control_plane"
	} else {
		machineType = "worker"
	}
	m["machine_type"] = machineType
	return m
}
