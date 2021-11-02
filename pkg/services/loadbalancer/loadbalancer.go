package loadbalancer

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

type Service struct {
	scope *scope.ClusterScope
}

func NewService(scope *scope.ClusterScope) *Service {
	return &Service{
		scope: scope,
	}
}

func (s *Service) Reconcile(ctx context.Context) (err error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconcile load balancer")

	var lbStatus infrav1.HCloudLoadBalancerStatus

	// find load balancer
	lb, err := findLoadBalancer(s.scope)
	if err != nil {
		return errors.Wrap(err, "failed to find load balancer")
	}

	if lb != nil {
		lbStatus, err = s.apiToStatus(lb)
		if err != nil {
			return errors.Wrap(err, "failed to obtain load balancer status")
		}
		s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = lbStatus
	} else if lb, err = s.createLoadBalancer(ctx, s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer); err != nil {
		return errors.Wrap(err, "failed to create load balancer")
	}

	// reconcile network attachement
	if err := s.reconcileNetworkAttachement(ctx, lb); err != nil {
		return errors.Wrap(err, "failed to reconcile network attachement")
	}

	// update current status
	lbStatus, err = s.apiToStatus(lb)
	if err != nil {
		return errors.Wrap(err, "failed to refresh load balancer status")
	}
	s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer = lbStatus

	return nil
}

func (s *Service) reconcileNetworkAttachement(ctx context.Context, lb *hcloud.LoadBalancer) error {
	// Nothing to do if already attached to network
	if len(lb.PrivateNet) > 0 {
		return nil
	}

	// attach load balancer to network
	if s.scope.HetznerCluster.Status.Network == nil {
		return errors.New("no network set on the cluster")
	}

	networkID := s.scope.HetznerCluster.Status.Network.ID

	opts := hcloud.LoadBalancerAttachToNetworkOpts{
		Network: &hcloud.Network{
			ID: networkID,
		},
	}

	if _, _, err := s.scope.HCloudClient().AttachLoadBalancerToNetwork(ctx, lb, opts); err != nil {
		record.Warnf(
			s.scope.HetznerCluster,
			"FailedAttachLoadBalancer",
			"Failed to attach load balancer to network: %s",
			err)
		return errors.Wrap(err, "failed to attach load balancer to network")
	}

	var err error
	lb, err = findLoadBalancer(s.scope)
	if err != nil {
		return errors.Wrap(err, "failed to find load balancer")
	}

	return nil
}

func (s *Service) createLoadBalancer(ctx context.Context, spec infrav1.HCloudLoadBalancerSpec) (*hcloud.LoadBalancer, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Create a new loadbalancer", "algorithm type", spec.Algorithm)

	// gather algorithm type
	var algType hcloud.LoadBalancerAlgorithmType
	if spec.Algorithm == infrav1.HCloudLoadBalancerAlgorithmTypeRoundRobin {
		algType = hcloud.LoadBalancerAlgorithmTypeRoundRobin
	} else if spec.Algorithm == infrav1.HCloudLoadBalancerAlgorithmTypeLeastConnections {
		algType = hcloud.LoadBalancerAlgorithmTypeLeastConnections
	} else {
		return nil, fmt.Errorf("error invalid load balancer algorithm type: %s", spec.Algorithm)
	}

	loadBalancerAlgorithm := &hcloud.LoadBalancerAlgorithm{Type: algType}

	name := names.SimpleNameGenerator.GenerateName(s.scope.HetznerCluster.Name + "-kube-apiserver-")
	if s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name != nil {
		name = *s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Name
	}

	// Get the Hetzner cloud object of load balancer type
	loadBalancerType, _, err := s.scope.HCloudClient().GetLoadBalancerTypeByName(ctx, spec.Type)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find load balancer type")
	}

	location := &hcloud.Location{Name: s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Region}

	var network *hcloud.Network

	if s.scope.HetznerCluster.Status.Network != nil {
		networkID := s.scope.HetznerCluster.Status.Network.ID
		network, _, err := s.scope.HCloudClient().GetNetwork(ctx, networkID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list networks")
		}
		if network == nil {
			return nil, fmt.Errorf("could not find network with ID %v", networkID)
		}
	}

	var mybool bool
	// The first service in the list is the one of kubeAPI
	kubeAPISpec := s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Services[0]

	createServiceOpts := hcloud.LoadBalancerCreateOptsService{
		Protocol:        hcloud.LoadBalancerServiceProtocol(kubeAPISpec.Protocol),
		ListenPort:      &kubeAPISpec.ListenPort,
		DestinationPort: &kubeAPISpec.DestinationPort,
		Proxyprotocol:   &mybool,
	}

	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)

	labels := map[string]string{clusterTagKey: string(infrav1.ResourceLifecycleOwned)}

	opts := hcloud.LoadBalancerCreateOpts{
		LoadBalancerType: loadBalancerType,
		Name:             name,
		Algorithm:        loadBalancerAlgorithm,
		Location:         location,
		Network:          network,
		Labels:           labels,
		Services:         []hcloud.LoadBalancerCreateOptsService{createServiceOpts},
	}

	res, _, err := s.scope.HCloudClient().CreateLoadBalancer(ctx, opts)
	if err != nil {
		record.Warnf(
			s.scope.HetznerCluster,
			"FailedCreateLoadBalancer",
			"Failed to create load balancer: %s",
			err)
		return nil, fmt.Errorf("error creating load balancer: %s", err)
	}

	// If there is more than one service in the specs, add them here one after another
	// Adding all at the same time on creation led to an error that the source port is already in use
	if len(s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Services) > 1 {
		for _, spec := range s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Services[1:] {
			serviceOpts := hcloud.LoadBalancerAddServiceOpts{
				Protocol:        hcloud.LoadBalancerServiceProtocol(spec.Protocol),
				ListenPort:      &spec.ListenPort,
				DestinationPort: &spec.DestinationPort,
				Proxyprotocol:   &mybool,
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

func (s *Service) Delete(ctx context.Context) (err error) {
	if _, err := s.scope.HCloudClient().DeleteLoadBalancer(ctx, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID); err != nil {
		record.Eventf(s.scope.HetznerCluster, "FailedLoadBalancerDelete", "Failed to delete load balancer: %s", err)
		return errors.Wrap(err, "failed to delete load balancer")
	}

	record.Eventf(s.scope.HetznerCluster, "DeleteLoadBalancer", "Deleted load balancer")

	return nil
}

func findLoadBalancer(scope *scope.ClusterScope) (*hcloud.LoadBalancer, error) {
	clusterTagKey := infrav1.ClusterTagKey(scope.HetznerCluster.Name)
	labels := map[string]string{clusterTagKey: string(infrav1.ResourceLifecycleOwned)}
	opts := hcloud.LoadBalancerListOpts{}
	opts.LabelSelector = utils.LabelsToLabelSelector(labels)

	loadBalancers, err := scope.HCloudClient().ListLoadBalancers(scope.Ctx, opts)
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
func (s *Service) apiToStatus(lb *hcloud.LoadBalancer) (status infrav1.HCloudLoadBalancerStatus, err error) {
	ipv4 := lb.PublicNet.IPv4.IP.String()
	ipv6 := lb.PublicNet.IPv6.IP.String()

	var internalIP string
	if s.scope.HetznerCluster.Status.Network != nil && len(lb.PrivateNet) > 0 {
		internalIP = lb.PrivateNet[0].IP.String()
	}

	var algType infrav1.HCloudLoadBalancerAlgorithmType

	switch lb.Algorithm.Type {
	case hcloud.LoadBalancerAlgorithmTypeRoundRobin:
		algType = infrav1.HCloudLoadBalancerAlgorithmTypeRoundRobin
	case hcloud.LoadBalancerAlgorithmTypeLeastConnections:
		algType = infrav1.HCloudLoadBalancerAlgorithmTypeLeastConnections
	default:
		return status, fmt.Errorf("unknown load balancer algorithm type: %s", lb.Algorithm.Type)
	}

	var targetIDs []int

	for _, server := range lb.Targets {
		targetIDs = append(targetIDs, server.Server.Server.ID)
	}

	attachedToNetwork := len(lb.PrivateNet) > 0

	return infrav1.HCloudLoadBalancerStatus{
		ID:                lb.ID,
		Name:              lb.Name,
		Type:              lb.LoadBalancerType.Name,
		IPv4:              ipv4,
		IPv6:              ipv6,
		InternalIP:        internalIP,
		Labels:            lb.Labels,
		Algorithm:         algType,
		Target:            targetIDs,
		AttachedToNetwork: attachedToNetwork,
	}, nil
}
