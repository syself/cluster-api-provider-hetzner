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

// Package server implements functions to manage the lifecycle of HCloud servers.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

const (
	serverOffTimeout = 10 * time.Minute

	// requeueAfterCreateServer: TODO get a good value. It should work for most cases, so
	// that the next reconcile gets a created server.
	requeueAfterCreateServer = 10 * time.Second
)

var (
	errWrongLabel              = fmt.Errorf("label is wrong")
	errMissingLabel            = fmt.Errorf("label is missing")
	errServerCreateNotPossible = fmt.Errorf("server create not possible - need action")
)

// Service defines struct with machine scope to reconcile HCloudMachines.
type Service struct {
	scope *scope.MachineScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.MachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HCloudMachines.
func (s *Service) Reconcile(ctx context.Context) (res reconcile.Result, err error) {
	// delete the deprecated condition from existing machine objects
	conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedInstanceReadyCondition)
	conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedInstanceBootstrapReadyCondition)
	conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedRateLimitExceededCondition)

	// detect failure domain
	failureDomain, err := s.scope.GetFailureDomain()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get failure domain: %w", err)
	}

	// set region in status of machine
	s.scope.SetRegion(failureDomain)

	// waiting for bootstrap data to be ready
	if !s.scope.IsBootstrapDataReady() {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.BootstrapReadyCondition,
			infrav1.BootstrapNotReadyReason,
			clusterv1.ConditionSeverityInfo,
			"bootstrap not ready yet",
		)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	conditions.MarkTrue(s.scope.HCloudMachine, infrav1.BootstrapReadyCondition)

	var server *hcloud.Server
	if s.scope.HCloudMachine.Spec.ProviderID != nil {
		server, err = s.findServer(ctx)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("handleBootToRealOS: %w", err)
		}

		if server == nil {
			// This should not happen in production, but happens in tests.
			return reconcile.Result{}, fmt.Errorf("no server found: Spec.ProviderID=%s Status.BootState=%s", *s.scope.HCloudMachine.Spec.ProviderID,
				s.scope.HCloudMachine.Status.BootState)
		}
	}

	switch s.scope.HCloudMachine.Status.BootState {
	case infrav1.HCloudBootStateUnset:
		return s.handleBootStateUnset(ctx)
	case infrav1.HCloudBootStateBootToRealOS:
		return s.handleBootToRealOS(ctx, server)
	default:
		return reconcile.Result{}, fmt.Errorf("unknown BootState: %s", s.scope.HCloudMachine.Status.BootState)
	}
}

func (s *Service) handleBootStateUnset(ctx context.Context) (reconcile.Result, error) {
	m := s.scope.HCloudMachine
	if m.Spec.ProviderID != nil && *m.Spec.ProviderID != "" {
		// This machine seems to be an existing machine which was created before introducing
		// Status.BootState.

		if !m.Status.Ready {
			m.SetBootState(infrav1.HCloudBootStateBootToRealOS)
			return reconcile.Result{RequeueAfter: requeueAfterCreateServer}, nil
		}
		m.SetBootState(infrav1.HCloudBootStateOperatingSystemRunning)
		return reconcile.Result{}, nil
	}

	server, err := s.createServerFromImageName(ctx)
	if err != nil {
		if errors.Is(err, errServerCreateNotPossible) {
			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to create server: %w", err)
	}
	s.scope.SetProviderID(server.ID)
	return reconcile.Result{RequeueAfter: requeueAfterCreateServer}, nil
}

func (s *Service) handleBootToRealOS(ctx context.Context, server *hcloud.Server) (res reconcile.Result, reterr error) {
	// update HCloudMachineStatus
	s.scope.HCloudMachine.Status.Addresses = statusAddresses(server)

	// Copy value
	s.scope.HCloudMachine.Status.InstanceState = ptr.To(server.Status)

	// validate labels
	if err := validateLabels(server, s.createLabels()); err != nil {
		err := fmt.Errorf("could not validate labels of HCloud server: %w", err)
		s.scope.SetError(err.Error(), capierrors.CreateMachineError)
		return res, nil
	}

	// analyze status of server
	switch server.Status {
	case hcloud.ServerStatusOff:
		return s.handleServerStatusOff(ctx, server)
	case hcloud.ServerStatusStarting:
		// Requeue here so that server does not switch back and forth between off and starting.
		// If we don't return here, the condition ServerAvailable would get marked as true in this
		// case. However, if the server is stuck and does not power on, we should not mark the
		// condition ServerAvailable as true to be able to remediate the server after a timeout.
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.ServerStartingReason,
			clusterv1.ConditionSeverityInfo,
			"server is starting",
		)
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	case hcloud.ServerStatusRunning: // do nothing
	default:
		// some temporary status
		ctrl.LoggerFrom(ctx).Info("Unknown hcloud server status", "status", server.Status)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// check whether server is attached to the network
	if err := s.reconcileNetworkAttachment(ctx, server); err != nil {
		reterr := fmt.Errorf("failed to reconcile network attachment: %w", err)
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.NetworkAttachFailedReason,
			clusterv1.ConditionSeverityError,
			"%s",
			reterr.Error(),
		)
		return res, reterr
	}

	// nothing to do any more for worker nodes
	if !s.scope.IsControlPlane() {
		conditions.MarkTrue(s.scope.HCloudMachine, infrav1.ServerAvailableCondition)
		s.scope.SetReady(true)
		return res, nil
	}

	// all control planes have to be attached to the load balancer if it exists
	res, err := s.reconcileLoadBalancerAttachment(ctx, server)
	if err != nil {
		reterr := fmt.Errorf("failed to reconcile load balancer attachment: %w", err)
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.LoadBalancerAttachFailedReason,
			clusterv1.ConditionSeverityError,
			"%s",
			reterr.Error(),
		)
		return res, reterr
	}

	s.scope.SetReady(true)
	s.scope.HCloudMachine.SetBootState(infrav1.HCloudBootStateOperatingSystemRunning)
	conditions.MarkTrue(s.scope.HCloudMachine, infrav1.ServerAvailableCondition)
	return reconcile.Result{}, nil
}

// implements setting rate limit on hcloudmachine.
func handleRateLimit(hm *infrav1.HCloudMachine, err error, functionName string, errMsg string) error {
	// returns error if not a rate limit exceeded error
	if !hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	// does not return error if machine is running and does not have a deletion timestamp
	if hm.Status.Ready && hm.DeletionTimestamp.IsZero() {
		return nil
	}

	// check for a rate limit exceeded error if the machine is not running or if machine has a deletion timestamp
	hcloudutil.HandleRateLimitExceeded(hm, err, functionName)
	return fmt.Errorf("%s: %w", errMsg, err)
}

// Delete implements delete method of server.
func (s *Service) Delete(ctx context.Context) (res reconcile.Result, err error) {
	server, err := s.findServer(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to find server: %w", err)
	}

	// if no server has been found, then nothing can be deleted
	if server == nil {
		msg := fmt.Sprintf("Unable to delete HCloud server. Could not find matching server for %s", s.scope.Name())
		s.scope.V(1).Info(msg)
		record.Warnf(s.scope.HCloudMachine, "NoInstanceFound", msg)
		return res, nil
	}

	// control planes have to be deleted as targets of server
	if s.scope.IsControlPlane() && s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled {
		for _, target := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
			if target.Type == infrav1.LoadBalancerTargetTypeServer && target.ServerID == server.ID {
				if err := s.deleteServerOfLoadBalancer(ctx, server); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to delete attached server of loadbalancer: %w", err)
				}
				break
			}
		}
	}

	// first shut the server down, then delete it
	switch server.Status {
	case hcloud.ServerStatusOff:
		return s.handleDeleteServerStatusOff(ctx, server)
	default:
		return s.handleDeleteServerStatusRunning(ctx, server)
	}
}

func (s *Service) reconcileNetworkAttachment(ctx context.Context, server *hcloud.Server) error {
	// if no network exists, then do nothing
	if s.scope.HetznerCluster.Status.Network == nil {
		return nil
	}

	// if it is already attached to network, then do nothing
	for _, id := range s.scope.HetznerCluster.Status.Network.AttachedServers {
		if id == server.ID {
			return nil
		}
	}

	// attach server to network
	if err := s.scope.HCloudClient.AttachServerToNetwork(ctx, server, hcloud.ServerAttachToNetworkOpts{
		Network: &hcloud.Network{
			ID: s.scope.HetznerCluster.Status.Network.ID,
		},
	}); err != nil {
		// check if network status is old and server is in fact already attached
		if hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAttached) {
			return nil
		}
		return handleRateLimit(s.scope.HCloudMachine, err, "AttachServerToNetwork", "failed to attach server to network")
	}

	return nil
}

func (s *Service) reconcileLoadBalancerAttachment(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		return reconcile.Result{}, nil
	}

	// remove server from load balancer if it's being deleted
	if conditions.Has(s.scope.Machine, clusterv1.PreDrainDeleteHookSucceededCondition) {
		if err := s.deleteServerOfLoadBalancer(ctx, server); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to delete server %s with ID %d from loadbalancer: %w", server.Name, server.ID, err)
		}
		return reconcile.Result{}, nil
	}

	// if already attached do nothing
	for _, target := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
		if target.Type == infrav1.LoadBalancerTargetTypeServer && target.ServerID == server.ID {
			return reconcile.Result{}, nil
		}
	}

	// we differentiate between private and public net
	var hasPrivateIP bool
	if len(server.PrivateNet) > 0 {
		hasPrivateIP = true
	}

	// if load balancer has not been attached to a network, then it cannot add a server with private IP
	if hasPrivateIP && conditions.IsFalse(s.scope.HetznerCluster, infrav1.LoadBalancerReadyCondition) {
		return reconcile.Result{}, nil
	}

	// attach only if server has private IP or public IPv4, otherwise Hetzner cannot handle it
	if server.PublicNet.IPv4.IP == nil && !hasPrivateIP {
		return reconcile.Result{}, nil
	}

	apiServerPodHealthy := s.scope.Cluster.Spec.ControlPlaneRef == nil ||
		s.scope.Cluster.Spec.ControlPlaneRef.Kind != "KubeadmControlPlane" ||
		conditions.IsTrue(s.scope.Machine, controlplanev1.MachineAPIServerPodHealthyCondition)

	// we attach only nodes with kube-apiserver pod healthy to avoid downtime, skipped for the first node
	if len(s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target) > 0 && !apiServerPodHealthy {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	opts := hcloud.LoadBalancerAddServerTargetOpts{
		Server:       server,
		UsePrivateIP: &hasPrivateIP,
	}
	loadBalancer := &hcloud.LoadBalancer{
		ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
	}

	if err := s.scope.HCloudClient.AddTargetServerToLoadBalancer(ctx, opts, loadBalancer); err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeTargetAlreadyDefined) {
			return reconcile.Result{}, nil
		}
		errMsg := fmt.Sprintf("failed to add server %s with ID %d as target to load balancer", server.Name, server.ID)
		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "AddTargetServerToLoadBalancer", errMsg)
	}

	record.Eventf(
		s.scope.HetznerCluster,
		"AddedAsTargetToLoadBalancer",
		"Added new server %s with ID %d to the loadbalancer with ID %d",
		server.Name, server.ID, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)

	return reconcile.Result{}, nil
}

func (s *Service) createServerFromImageName(ctx context.Context) (*hcloud.Server, error) {
	userData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		record.Warnf(
			s.scope.HCloudMachine,
			"FailedGetBootstrapData",
			err.Error(),
		)
		return nil, fmt.Errorf("failed to get raw bootstrap data: %s", err)
	}
	image, err := getServerImage(ctx, s.scope.HCloudClient, s.scope.HCloudMachine, s.scope.HCloudMachine.Spec.Type, s.scope.HCloudMachine.Spec.ImageName)
	if err != nil {
		return nil, fmt.Errorf("createServerFromImageName: failed to get server image: %w", err)
	}
	server, err := s.createServer(ctx, userData, image)
	if err != nil {
		return nil, err
	}
	s.scope.HCloudMachine.SetBootState(infrav1.HCloudBootStateBootToRealOS)
	return server, nil
}

func (s *Service) createServer(ctx context.Context, userData []byte, image *hcloud.Image) (*hcloud.Server, error) {
	automount := false
	startAfterCreate := true
	opts := hcloud.ServerCreateOpts{
		Name:   s.scope.Name(),
		Labels: s.createLabels(),
		Image:  image,
		Location: &hcloud.Location{
			Name: string(s.scope.HCloudMachine.Status.Region),
		},
		ServerType: &hcloud.ServerType{
			Name: string(s.scope.HCloudMachine.Spec.Type),
		},
		Automount:        &automount,
		StartAfterCreate: &startAfterCreate,
		UserData:         string(userData),
		PublicNet: &hcloud.ServerCreatePublicNet{
			EnableIPv4: s.scope.HCloudMachine.Spec.PublicNetwork.EnableIPv4,
			EnableIPv6: s.scope.HCloudMachine.Spec.PublicNetwork.EnableIPv6,
		},
	}

	// set placement group if necessary
	if s.scope.HCloudMachine.Spec.PlacementGroupName != nil {
		var foundPlacementGroupInStatus bool
		for _, pgSts := range s.scope.HetznerCluster.Status.HCloudPlacementGroups {
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
			conditions.MarkFalse(s.scope.HCloudMachine,
				infrav1.ServerCreateSucceededCondition,
				infrav1.InstanceHasNonExistingPlacementGroupReason,
				clusterv1.ConditionSeverityError,
				"Placement group %q does not exist in cluster",
				*s.scope.HCloudMachine.Spec.PlacementGroupName,
			)
			return nil, errServerCreateNotPossible
		}
	}

	caphSSHKeys, hcloudSSHKeys, err := s.getSSHKeys(ctx)
	if err != nil {
		return nil, err
	}
	opts.SSHKeys = hcloudSSHKeys

	// set up network if available
	if net := s.scope.HetznerCluster.Status.Network; net != nil {
		opts.Networks = []*hcloud.Network{{
			ID: net.ID,
		}}
	}

	// if no private network exists, there must be an IPv4 for the load balancer
	if !s.scope.HetznerCluster.Spec.HCloudNetwork.Enabled {
		opts.PublicNet.EnableIPv4 = true
	}

	// Create the server
	server, err := s.scope.HCloudClient.CreateServer(ctx, opts)
	if err != nil {
		if hcloudutil.HandleRateLimitExceeded(s.scope.HCloudMachine, err, "CreateServer") {
			// RateLimit was reached. Condition and Event got already created.
			return nil, fmt.Errorf("failed to create HCloud server %s: %w", s.scope.HCloudMachine.Name, err)
		}
		// No condition was set yet. Set a general condition to false.
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ServerCreateFailedReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)
		record.Warnf(s.scope.HCloudMachine,
			"FailedCreateHCloudServer",
			"Failed to create HCloud server %s: %s",
			s.scope.Name(),
			err,
		)
		errMsg := fmt.Sprintf("failed to create HCloud server %s", s.scope.HCloudMachine.Name)
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "CreateServer", errMsg)
	}

	// set ssh keys to status
	s.scope.HCloudMachine.Status.SSHKeys = caphSSHKeys

	conditions.MarkTrue(s.scope.HCloudMachine, infrav1.ServerCreateSucceededCondition)
	record.Eventf(s.scope.HCloudMachine, "SuccessfulCreate", "Created new server %s with ID %d", server.Name, server.ID)
	return server, nil
}

// getSSHKeys returns the sshKey slice for the status and the sshKey slice for the createServer API
// call.
func (s *Service) getSSHKeys(ctx context.Context) (
	caphSSHKeys []infrav1.SSHKey,
	hcloudSSHKeys []*hcloud.SSHKey,
	reterr error,
) {
	caphSSHKeys = s.scope.HCloudMachine.Spec.SSHKeys

	// if no ssh keys are specified on the machine, take the ones from the cluster
	if len(caphSSHKeys) == 0 {
		caphSSHKeys = s.scope.HetznerCluster.Spec.SSHKeys.HCloud
	}

	// always add ssh key from secret if one is found

	// this code is redundant with a similar one on cluster level but is necessary if ClusterClass is used
	// as in ClusterClass we cannot store anything in HetznerCluster object
	sshKeyName := s.scope.HetznerSecret().Data[s.scope.HetznerCluster.Spec.HetznerSecret.Key.SSHKey]
	if len(sshKeyName) > 0 {
		// Check if the SSH key name already exists
		keyExists := false
		for _, key := range caphSSHKeys {
			if string(sshKeyName) == key.Name {
				keyExists = true
				break
			}
		}

		// If the SSH key name doesn't exist, append it
		if !keyExists {
			caphSSHKeys = append(caphSSHKeys, infrav1.SSHKey{Name: string(sshKeyName)})
		}
	}

	// get all ssh keys that are stored in HCloud API
	sshKeysAPI, err := s.scope.HCloudClient.ListSSHKeys(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return nil, nil, handleRateLimit(s.scope.HCloudMachine, err, "ListSSHKeys", "failed listing ssh keys from hcloud")
	}

	// find matching keys and store them
	hcloudSSHKeys, err = convertCaphSSHKeysToHcloudSSHKeys(sshKeysAPI, caphSSHKeys)
	if err != nil {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.SSHKeyNotFoundReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return nil, nil, errServerCreateNotPossible
	}
	return caphSSHKeys, hcloudSSHKeys, nil
}

func getServerImage(ctx context.Context, hcloudClient hcloudclient.Client,
	hcloudMachine *infrav1.HCloudMachine, machineType infrav1.HCloudMachineType, imageName string,
) (*hcloud.Image, error) {
	key := fmt.Sprintf("%s%s", infrav1.NameHetznerProviderPrefix, "image-name")

	// Get server type so we can filter for images with correct architecture
	serverType, err := hcloudClient.GetServerType(ctx, string(machineType))
	if err != nil {
		return nil, handleRateLimit(hcloudMachine, err, "GetServerType", "failed to get server type in HCloud")
	}
	if serverType == nil {
		conditions.MarkFalse(
			hcloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ServerTypeNotFoundReason,
			clusterv1.ConditionSeverityError,
			"failed to get server type - nil type",
		)
		return nil, errServerCreateNotPossible
	}

	// query for an existing image by label
	// this is needed because snapshots don't have a name, only descriptions and labels
	listOpts := hcloud.ImageListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: fmt.Sprintf("%s==%s", key, imageName),
		},
		Architecture: []hcloud.Architecture{serverType.Architecture},
	}

	images, err := hcloudClient.ListImages(ctx, listOpts)
	if err != nil {
		return nil, handleRateLimit(hcloudMachine, err, "ListImages", "failed to list images by label in HCloud")
	}

	// query for an existing image by name.
	listOpts = hcloud.ImageListOpts{
		Name:         imageName,
		Architecture: []hcloud.Architecture{serverType.Architecture},
	}
	imagesByName, err := hcloudClient.ListImages(ctx, listOpts)
	if err != nil {
		return nil, handleRateLimit(hcloudMachine, err, "ListImages", "failed to list images by name in HCloud")
	}

	images = append(images, imagesByName...)

	if len(images) > 1 {
		err := fmt.Errorf("image is ambiguous - %d images have name %s",
			len(images), imageName)
		record.Warnf(hcloudMachine, "ImageNameAmbiguous", err.Error())
		conditions.MarkFalse(hcloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ImageAmbiguousReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return nil, errServerCreateNotPossible
	}
	if len(images) == 0 {
		err := fmt.Errorf("no image found with name %s", imageName)
		record.Warnf(hcloudMachine, "ImageNotFound", err.Error())
		conditions.MarkFalse(hcloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ImageNotFoundReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return nil, errServerCreateNotPossible
	}

	return images[0], nil
}

func (s *Service) handleServerStatusOff(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	// Check if server is in ServerStatusOff and turn it on. This is to avoid a bug of Hetzner where
	// sometimes machines are created and not turned on

	serverAvailableCondition := conditions.Get(s.scope.HCloudMachine, infrav1.ServerAvailableCondition)
	if serverAvailableCondition != nil &&
		serverAvailableCondition.Status == corev1.ConditionFalse &&
		serverAvailableCondition.Reason == infrav1.ServerOffReason {
		s.scope.Info("Trigger power on again")
		if time.Now().Before(serverAvailableCondition.LastTransitionTime.Time.Add(serverOffTimeout)) {
			// Not yet timed out, try again to power on
			if err := s.scope.HCloudClient.PowerOnServer(ctx, server); err != nil {
				if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
					// if server is locked, we just retry again
					return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
				}
				return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "PowerOnServer", "failed to power on server")
			}
		} else {
			// Timed out. Set failure reason
			s.scope.SetError("reached timeout of waiting for machines that are switched off", capierrors.CreateMachineError)
			return res, nil
		}
	} else {
		// No condition set yet. Try to power server on.
		if err := s.scope.HCloudClient.PowerOnServer(ctx, server); err != nil {
			if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
				// if server is locked, we just retry again
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
			return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "PowerOnServer", "failed to power on server")
		}
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.ServerOffReason,
			clusterv1.ConditionSeverityInfo,
			"server is switched off",
		)
	}

	// Try again in 30 sec.
	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

func (s *Service) handleDeleteServerStatusRunning(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	// Shut down the server if one of the two conditions apply:
	// 1. The server has not yet been tried to shut down and still is marked as "ready".
	// 2. The server has been tried to shut down without an effect and the timeout is not reached yet.

	if s.scope.HasServerAvailableCondition() {
		if err := s.scope.HCloudClient.ShutdownServer(ctx, server); err != nil {
			return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "ShutdownServer", "failed to shutdown server")
		}

		conditions.MarkFalse(s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.ServerTerminatingReason,
			clusterv1.ConditionSeverityInfo,
			"Instance has been shut down",
		)

		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// timeout for shutdown has been reached - delete server
	if err := s.scope.HCloudClient.DeleteServer(ctx, server); err != nil {
		record.Warnf(s.scope.HCloudMachine, "FailedDeleteHCloudServer", "Failed to delete HCloud server %s", s.scope.Name())
		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "DeleteServer", "failed to delete server")
	}

	record.Eventf(s.scope.HCloudMachine, "HCloudServerDeleted", "HCloud server %s deleted", s.scope.Name())
	return res, nil
}

func (s *Service) handleDeleteServerStatusOff(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	// server is off and can be deleted
	if err := s.scope.HCloudClient.DeleteServer(ctx, server); err != nil {
		record.Warnf(s.scope.HCloudMachine, "FailedDeleteHCloudServer", "Failed to delete HCloud server %s", s.scope.Name())
		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "DeleteServer", "failed to delete server")
	}

	record.Eventf(s.scope.HCloudMachine, "HCloudServerDeleted", "HCloud server %s deleted", s.scope.Name())
	return res, nil
}

func (s *Service) deleteServerOfLoadBalancer(ctx context.Context, server *hcloud.Server) error {
	lb := &hcloud.LoadBalancer{ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID}

	if err := s.scope.HCloudClient.DeleteTargetServerOfLoadBalancer(ctx, lb, server); err != nil {
		// Do not return an error in case the target server was not found.
		// In case the target server was not found we will get an error similar to
		// "server with ID xxxxx not found (invalid_input, xxxxxxx)".
		// If the load balancer itself was not found then we will get a "not_found" error.
		// In both cases, don't do anything.
		if hcloud.IsError(err, hcloud.ErrorCodeInvalidInput) || hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			return nil
		}

		errMsg := fmt.Sprintf("failed to delete server %s with ID %d as target of load balancer %s with ID %d", server.Name, server.ID, lb.Name, lb.ID)
		return handleRateLimit(s.scope.HCloudMachine, err, "DeleteTargetServerOfLoadBalancer", errMsg)
	}
	record.Eventf(
		s.scope.HetznerCluster,
		"DeletedTargetOfLoadBalancer",
		"Deleted new server %s with ID %d of the loadbalancer %s with ID %d",
		server.Name, server.ID, lb.Name, lb.ID,
	)

	return nil
}

func (s *Service) findServer(ctx context.Context) (*hcloud.Server, error) {
	var server *hcloud.Server

	// try to find the server based on its id
	serverID, err := s.scope.ServerIDFromProviderID()
	if err == nil {
		server, err = s.scope.HCloudClient.GetServer(ctx, serverID)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get server %d", serverID)
			return nil, handleRateLimit(s.scope.HCloudMachine, err, "GetServer", errMsg)
		}

		// if server has been found, return it
		if server != nil {
			return server, nil
		}
	}

	logger := ctrl.LoggerFrom(ctx)
	logger.Info("DeprecationWarning finding Server by labels is no longer needed. We plan to remove that feature and rename findServer to getServer", "err", err)

	// server has not been found via id - try to find the server based on its labels
	opts := hcloud.ServerListOpts{}

	opts.LabelSelector = utils.LabelsToLabelSelector(s.createLabels())

	servers, err := s.scope.HCloudClient.ListServers(ctx, opts)
	if err != nil {
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "ListServers", "failed to list servers")
	}

	if len(servers) > 1 {
		err := fmt.Errorf("found %d servers with name %s", len(servers), s.scope.Name())
		record.Warnf(s.scope.HCloudMachine, "MultipleInstances", err.Error())
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	return servers[0], nil
}

func validateLabels(server *hcloud.Server, labels map[string]string) error {
	for key, val := range labels {
		wantVal, found := server.Labels[key]
		if !found {
			return fmt.Errorf("did not find label with key %q: %w", key, errMissingLabel)
		}
		if wantVal != val {
			return fmt.Errorf("got %q, want %q: %w", val, wantVal, errWrongLabel)
		}
	}
	return nil
}

func statusAddresses(server *hcloud.Server) []clusterv1.MachineAddress {
	// populate addresses
	addresses := []clusterv1.MachineAddress{}

	if ip := server.PublicNet.IPv4.IP.String(); ip != "" {
		addresses = append(
			addresses,
			clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
				Address: ip,
			},
		)
	}

	if unicastIP := server.PublicNet.IPv6.IP; unicastIP.IsGlobalUnicast() {
		// Create a copy. This is important, otherwise we modify the IP of `server`. This could lead
		// to unexpected behaviour.
		ip := append(net.IP(nil), unicastIP...)

		// Hetzner returns the routed /64 base, increment last byte to obtain first usable address
		// The local value gets changed, not the IP of `server`.
		ip[15]++

		addresses = append(
			addresses,
			clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
				Address: ip.String(),
			},
		)
	}

	for _, net := range server.PrivateNet {
		addresses = append(
			addresses,
			clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
				Address: net.IP.String(),
			},
		)
	}

	return addresses
}

func (s *Service) createLabels() map[string]string {
	var machineType string
	if s.scope.IsControlPlane() {
		machineType = "control_plane"
	} else {
		machineType = "worker"
	}

	return map[string]string{
		infrav1.NameHetznerProviderOwned + s.scope.HetznerCluster.Name: string(infrav1.ResourceLifecycleOwned),
		infrav1.MachineNameTagKey:                                      s.scope.Name(),
		"machine_type":                                                 machineType,
	}
}

// convertCaphSSHKeysToHcloudSSHKeys converts the ssh keys from the spec to hcloud ssh keys.
// sshKeysAPI contains all keys, returned by hcloudClient.ListSSHKeys().
func convertCaphSSHKeysToHcloudSSHKeys(allHcloudSSHKeys []*hcloud.SSHKey, caphSSHKeys []infrav1.SSHKey) ([]*hcloud.SSHKey, error) {
	sshKeysAPIMap := make(map[string]*hcloud.SSHKey, len(allHcloudSSHKeys))
	for i, sshKey := range allHcloudSSHKeys {
		sshKeysAPIMap[sshKey.Name] = allHcloudSSHKeys[i]
	}

	hcloudSSHKeys := make([]*hcloud.SSHKey, len(caphSSHKeys))

	for i, sshKeySpec := range caphSSHKeys {
		sshKey, ok := sshKeysAPIMap[sshKeySpec.Name]
		if !ok {
			return nil, fmt.Errorf("ssh key not found in HCloud. SSH key name: %s", sshKeySpec.Name)
		}
		hcloudSSHKeys[i] = sshKey
	}
	return hcloudSSHKeys, nil
}
