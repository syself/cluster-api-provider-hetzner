package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	errorutil "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

type Service struct {
	scope *scope.MachineScope
}

func NewService(scope *scope.MachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {
	// detect failure domain
	failureDomain, err := s.scope.GetFailureDomain()
	if err != nil {
		return nil, err
	}
	s.scope.HCloudMachine.Status.Region = infrav1.HCloudRegion(failureDomain)

	// gather image ID

	imageID, err := s.scope.EnsureImage(ctx, s.scope.HCloudMachine.Spec.Image.Name)
	if err != nil {
		record.Warnf(s.scope.HCloudMachine,
			"FailedEnsuringHCloudImage",
			"Failed to ensure image for HCloud server %s: %s",
			s.scope.Name(),
			err,
		)
		return nil, err
	}
	// We have to wait for the image and bootstrap data to be ready
	if imageID == nil {
		return &ctrl.Result{RequeueAfter: 20 * time.Second}, nil
	}

	if !s.scope.IsBootstrapDataReady(s.scope.Ctx) {
		return &ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	instance, err := s.findServer(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get server")
	}

	// If no server is found we have to create one
	if instance == nil {
		instance, err = s.createServer(s.scope.Ctx, failureDomain, imageID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create server")
		}
		record.Eventf(
			s.scope.HCloudMachine,
			"SuccessfulCreate",
			"Created new server with id %d",
			instance.ID,
		)
	}

	if err := setStatusFromAPI(&s.scope.HCloudMachine.Status, instance); err != nil {
		return nil, errors.New("error setting status")
	}

	switch instance.Status {
	case hcloud.ServerStatusOff:
		// Check if server is in ServerStatusOff and turn it on. This is to avoid a bug of Hetzner where
		// sometimes machines are created and not turned on
		if _, _, err := s.scope.HCloudClient().PowerOnServer(ctx, instance); err != nil {
			return nil, err
		}
	case hcloud.ServerStatusRunning: // Do nothing
	default:
		s.scope.HCloudMachine.Status.Ready = false
		s.scope.V(1).Info("server not in running state", "server", instance.Name, "status", instance.Status)
		return &reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	}

	// Check whether server is attached to the network
	if err := s.reconcileNetworkAttachment(ctx, instance); err != nil {
		return nil, errors.Wrap(err, "failed to reconcile network attachement")
	}

	// // Check whether server is attached to the placement group
	// if err := s.reconcilePlacementGroupAttachment(ctx, instance); err != nil {
	// 	return nil, errors.Wrap(err, "failed to reconcile placement group attachement")
	// }

	providerID := fmt.Sprintf("hcloud://%d", instance.ID)

	if !s.scope.IsControlPlane() {
		s.scope.HCloudMachine.Spec.ProviderID = &providerID
		s.scope.HCloudMachine.Status.Ready = true
		return nil, nil
	}

	// all control planes have to be attached to the load balancer
	if err := s.reconcileLoadBalancerAttachment(ctx, instance); err != nil {
		return nil, errors.Wrap(err, "failed to reconcile load balancer attachement")
	}

	// TODO: Refactor of function:
	// Checks if control-plane is ready. Loops through available control-planes,
	// rewrites kubeconfig address to address of control-plane.
	// Requests readyz endpoint of control-plane kube-apiserver
	var errors []error
	for _, address := range s.scope.HCloudMachine.Status.Addresses {
		if address.Type != corev1.NodeExternalIP && address.Type != corev1.NodeExternalDNS {
			continue
		}

		clientConfig, err := s.scope.ClientConfigWithAPIEndpoint(clusterv1.APIEndpoint{
			Host: address.Address,
			Port: s.scope.ControlPlaneAPIEndpointPort(),
		})
		if err != nil {
			return nil, err
		}

		if err := scope.IsControlPlaneReady(ctx, clientConfig); err != nil {
			errors = append(errors, err)
			break
		}

		s.scope.HCloudMachine.Spec.ProviderID = &providerID
		s.scope.HCloudMachine.Status.Ready = true
		return nil, nil
	}

	if err := errorutil.NewAggregate(errors); err != nil {
		record.Warnf(
			s.scope.HCloudMachine,
			"APIServerNotReady",
			"Health check for API server failed: %s",
			err,
		)
		log.Error(err, "aggregate error")
	}
	return nil, fmt.Errorf("not usable address found")
}

func (s *Service) reconcileNetworkAttachment(ctx context.Context, server *hcloud.Server) error {
	// If no network exists, then do nothing
	if s.scope.HetznerCluster.Status.Network == nil {
		return nil
	}

	// Already attached to network - nothing to do
	for _, id := range s.scope.HetznerCluster.Status.Network.AttachedServer {
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
		if strings.Contains(err.Error(), "already attached") {
			return nil
		}
		return errors.Wrap(err, "failed to attach server to network")
	}

	return nil
}

func (s *Service) createServer(ctx context.Context, failureDomain string, imageID *infrav1.HCloudImageID) (*hcloud.Server, error) {
	s.scope.HCloudMachine.Status.ImageID = imageID

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

	var myTrue = true
	var myFalse = false

	name := s.scope.Name()
	opts := hcloud.ServerCreateOpts{
		Name:   name,
		Labels: s.createLabels(),
		Image: &hcloud.Image{
			ID: int(*s.scope.HCloudMachine.Status.ImageID),
		},
		Location: &hcloud.Location{
			Name: failureDomain,
		},
		ServerType: &hcloud.ServerType{
			Name: string(s.scope.HCloudMachine.Spec.Type),
		},
		Automount:        &myFalse,
		StartAfterCreate: &myTrue,
		UserData:         string(userData),
	}

	// set placement group if necessary
	if s.scope.HCloudMachine.Spec.PlacementGroupName != nil {
		var foundPlacementGroupInStatus bool
		for _, pgSts := range s.scope.HetznerCluster.Status.PlacementGroup {
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

	// set up SSH keys
	sshKeySpecs := s.scope.HCloudMachine.Spec.SSHKey
	if len(sshKeySpecs) == 0 {
		sshKeySpecs = s.scope.HetznerCluster.Spec.SSHKey
	}
	sshKeys, _, err := s.scope.HCloudClient().ListSSHKeys(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return nil, errors.Wrap(err, "failed listing ssh heys from hcloud")
	}
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
			opts.SSHKeys = append(opts.SSHKeys, sshKey)
		}
	}

	// set up network if available
	if net := s.scope.HetznerCluster.Status.Network; net != nil {
		opts.Networks = []*hcloud.Network{{
			ID: net.ID,
		}}
	}

	// Create the server
	res, _, err := s.scope.HCloudClient().CreateServer(s.scope.Ctx, opts)
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

func (s *Service) Delete(ctx context.Context) (_ *ctrl.Result, err error) {
	// find current server
	server, err := s.findServer(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to refresh server status")
	}
	var result *ctrl.Result
	// If no server has been found then nothing can be deleted
	if server == nil {
		s.scope.V(2).Info("Unable to locate HCloud instance by ID or tags")
		record.Warnf(s.scope.HCloudMachine, "NoInstanceFound", "Unable to find matching HCloud instance for %s", s.scope.Name())
		return result, nil
	}

	err = s.deleteServerOfLoadBalancer(ctx, server)
	if err != nil {
		return &reconcile.Result{}, errors.Errorf("Error while deleting attached server of loadbalancer: %s", err)
	}

	// First shut the server down, then delete it
	switch status := server.Status; status {
	case hcloud.ServerStatusRunning:
		if _, _, err := s.scope.HCloudClient().ShutdownServer(ctx, server); err != nil {
			return &reconcile.Result{}, errors.Wrap(err, "failed to shutdown server")
		}
		return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	case hcloud.ServerStatusOff:
		if _, err := s.scope.HCloudClient().DeleteServer(ctx, server); err != nil {
			record.Warnf(s.scope.HCloudMachine, "FailedDeleteHCloudServer", "Failed to delete HCloud server %s", s.scope.Name())
			return &reconcile.Result{}, errors.Wrap(err, "failed to delete server")
		}

	default:
		//actionWait
		return &ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	record.Eventf(
		s.scope.HCloudMachine,
		"HCloudServerDeleted",
		"HCloud server %s deleted",
		s.scope.Name(),
	)
	return result, nil
}

func setStatusFromAPI(status *infrav1.HCloudMachineStatus, server *hcloud.Server) error {
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
		ip[15] += 1
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

	return nil
}

func (s *Service) reconcileLoadBalancerAttachment(ctx context.Context, server *hcloud.Server) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("lb targets", "target", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target)

	// If already attached do nothing
	for _, id := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
		if id == server.ID {
			return nil
		}
	}
	log.Info("Reconciling load balancer attachement", "targets", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target)
	// We differentiate between private and public net
	var hasPrivateIP bool
	if len(server.PrivateNet) > 0 {
		hasPrivateIP = true
	}

	// If load balancer has not been attached to a network, then it cannot add a server with private IP
	if hasPrivateIP && !s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.AttachedToNetwork {
		return nil
	}

	loadBalancerAddServerTargetOpts := hcloud.LoadBalancerAddServerTargetOpts{Server: server, UsePrivateIP: &hasPrivateIP}

	if _, _, err := s.scope.HCloudClient().AddTargetServerToLoadBalancer(
		ctx,
		loadBalancerAddServerTargetOpts,
		&hcloud.LoadBalancer{
			ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
		}); err != nil {
		// Check if load balancer status is old and server is in fact already attached
		if strings.Contains(err.Error(), "target_already_defined") {
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
	lb, _, err := s.scope.HCloudClient().GetLoadBalancer(ctx, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
	if err != nil {
		return err
	}

	// if the server is not attached to the load balancer then we return without doing anything
	var stillAttached bool
	for _, target := range lb.Targets {
		if target.Server.Server.ID == server.ID {
			stillAttached = true
		}
	}

	if !stillAttached {
		return nil
	}

	_, _, err = s.scope.HCloudClient().DeleteTargetServerOfLoadBalancer(ctx, lb, server)
	if err != nil {
		s.scope.V(1).Info("Could not delete server as target of load balancer", "Server", server.ID, "Load Balancer", lb.ID)
		return err
	} else {
		record.Eventf(
			s.scope.HetznerCluster,
			"DeletedTargetOfLoadBalancer",
			"Deleted new server with id %d of the loadbalancer %v",
			server.ID, lb.ID)
	}
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
			"Found %v instances of name %s",
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
