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

// Package fake implements fakes for important interfaces like the HCloud API.
package fake

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"

	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// DefaultCPUCores defines the default CPU cores for HCloud machines' capacities.
const DefaultCPUCores = 1

// DefaultMemoryInGB defines the default memory in GB for HCloud machines' capacities.
const DefaultMemoryInGB = float32(4)

// DefaultArchitecture defines the default CPU architecture for HCloud server types.
const DefaultArchitecture = hcloud.ArchitectureX86

type cacheHCloudClient struct {
	serverCache             serverCache
	placementGroupCache     placementGroupCache
	loadBalancerCache       loadBalancerCache
	networkCache            networkCache
	counterMutex            sync.Mutex
	serverIDCounter         int64
	placementGroupIDCounter int64
	loadBalancerIDCounter   int64
	networkIDCounter        int64
}

// NewClient gives reference to the fake client using cache for HCloud API.
func (f *cacheHCloudClientFactory) NewClient(string) hcloudclient.Client {
	return cacheHCloudClientInstance
}

// Close implements Close method of hcloud client interface.
func (c *cacheHCloudClient) Close() {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()

	cacheHCloudClientInstance.serverCache = serverCache{}
	cacheHCloudClientInstance.networkCache = networkCache{}
	cacheHCloudClientInstance.loadBalancerCache = loadBalancerCache{}
	cacheHCloudClientInstance.placementGroupCache = placementGroupCache{}

	cacheHCloudClientInstance.serverCache = serverCache{
		idMap:   make(map[int64]*hcloud.Server),
		nameMap: make(map[string]struct{}),
	}
	cacheHCloudClientInstance.placementGroupCache = placementGroupCache{
		idMap:   make(map[int64]*hcloud.PlacementGroup),
		nameMap: make(map[string]struct{}),
	}
	cacheHCloudClientInstance.loadBalancerCache = loadBalancerCache{
		idMap:   make(map[int64]*hcloud.LoadBalancer),
		nameMap: make(map[string]struct{}),
	}
	cacheHCloudClientInstance.networkCache = networkCache{
		idMap:   make(map[int64]*hcloud.Network),
		nameMap: make(map[string]struct{}),
	}

	cacheHCloudClientInstance.serverIDCounter = 0
	cacheHCloudClientInstance.placementGroupIDCounter = 0
	cacheHCloudClientInstance.loadBalancerIDCounter = 0
	cacheHCloudClientInstance.networkIDCounter = 0
}

type cacheHCloudClientFactory struct{}

var cacheHCloudClientInstance = &cacheHCloudClient{
	serverCache: serverCache{
		idMap:   make(map[int64]*hcloud.Server),
		nameMap: make(map[string]struct{}),
	},
	placementGroupCache: placementGroupCache{
		idMap:   make(map[int64]*hcloud.PlacementGroup),
		nameMap: make(map[string]struct{}),
	},
	loadBalancerCache: loadBalancerCache{
		idMap:   make(map[int64]*hcloud.LoadBalancer),
		nameMap: make(map[string]struct{}),
	},
	networkCache: networkCache{
		idMap:   make(map[int64]*hcloud.Network),
		nameMap: make(map[string]struct{}),
	},
}

// NewHCloudClientFactory creates new fake HCloud client factories using cache.
func NewHCloudClientFactory() hcloudclient.Factory {
	return &cacheHCloudClientFactory{}
}

var _ = hcloudclient.Factory(&cacheHCloudClientFactory{})

type serverCache struct {
	idMap   map[int64]*hcloud.Server
	nameMap map[string]struct{}
}

type loadBalancerCache struct {
	idMap   map[int64]*hcloud.LoadBalancer
	nameMap map[string]struct{}
}

type networkCache struct {
	idMap   map[int64]*hcloud.Network
	nameMap map[string]struct{}
}

type placementGroupCache struct {
	idMap   map[int64]*hcloud.PlacementGroup
	nameMap map[string]struct{}
}

var defaultSSHKey = hcloud.SSHKey{
	ID:          1,
	Name:        "testsshkey",
	Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
}

var defaultImage = hcloud.Image{
	ID:   42,
	Name: "myimage",
}

func (c *cacheHCloudClient) CreateLoadBalancer(_ context.Context, opts hcloud.LoadBalancerCreateOpts) (*hcloud.LoadBalancer, error) {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()

	// cannot have two load balancers with the same name
	if _, found := c.loadBalancerCache.nameMap[opts.Name]; found {
		return nil, fmt.Errorf("failed to create lb: already exists")
	}

	c.loadBalancerIDCounter++
	lb := &hcloud.LoadBalancer{
		ID:               c.loadBalancerIDCounter,
		Name:             opts.Name,
		Labels:           opts.Labels,
		Algorithm:        *opts.Algorithm,
		LoadBalancerType: opts.LoadBalancerType,
		Location:         opts.Location,
		PublicNet: hcloud.LoadBalancerPublicNet{
			IPv4: hcloud.LoadBalancerPublicNetIPv4{
				IP: net.IP("1.2.3.4"),
			},
			IPv6: hcloud.LoadBalancerPublicNetIPv6{
				IP: net.IP("2001:db8::1"),
			},
		},
	}
	if opts.Network != nil {
		lb.PrivateNet = append(lb.PrivateNet, hcloud.LoadBalancerPrivateNet{
			IP: net.IP("10.0.0.2"),
		})
	}

	// Add load balancer to cache
	c.loadBalancerCache.idMap[lb.ID] = lb
	c.loadBalancerCache.nameMap[lb.Name] = struct{}{}
	return lb, nil
}

func (c *cacheHCloudClient) DeleteLoadBalancer(_ context.Context, id int64) error {
	if _, found := c.loadBalancerCache.idMap[id]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	lb := c.loadBalancerCache.idMap[id]
	delete(c.loadBalancerCache.nameMap, lb.Name)
	delete(c.loadBalancerCache.idMap, id)
	return nil
}

func (c *cacheHCloudClient) ListLoadBalancers(_ context.Context, opts hcloud.LoadBalancerListOpts) ([]*hcloud.LoadBalancer, error) {
	lbs := make([]*hcloud.LoadBalancer, 0, len(c.loadBalancerCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to convert label selector to labels: %w", err)
	}

	for key, lb := range c.loadBalancerCache.idMap {
		// if name is set and is not correct, continue
		if opts.Name != "" && lb.Name != opts.Name {
			continue
		}

		// check for labels
		allLabelsFound := true
		for key, label := range labels {
			if val, found := lb.Labels[key]; !found || val != label {
				allLabelsFound = false
				break
			}
		}

		if allLabelsFound {
			lbs = append(lbs, c.loadBalancerCache.idMap[key])
		}
	}
	return lbs, nil
}

func (c *cacheHCloudClient) AttachLoadBalancerToNetwork(_ context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerAttachToNetworkOpts) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Check if network exists
	network, found := c.networkCache.idMap[opts.Network.ID]
	if !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].PrivateNet {
		if s.IP.Equal(network.IPRange.IP) {
			return fmt.Errorf("already added")
		}
	}

	// Add it
	c.loadBalancerCache.idMap[lb.ID].PrivateNet = append(
		c.loadBalancerCache.idMap[lb.ID].PrivateNet,
		hcloud.LoadBalancerPrivateNet{IP: network.IPRange.IP},
	)
	return nil
}

func (c *cacheHCloudClient) ChangeLoadBalancerType(_ context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerChangeTypeOpts) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Update it
	c.loadBalancerCache.idMap[lb.ID].LoadBalancerType = opts.LoadBalancerType
	return nil
}

func (c *cacheHCloudClient) ChangeLoadBalancerAlgorithm(_ context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerChangeAlgorithmOpts) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Update it
	c.loadBalancerCache.idMap[lb.ID].Algorithm.Type = opts.Type
	return nil
}

func (c *cacheHCloudClient) UpdateLoadBalancer(_ context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerUpdateOpts) (*hcloud.LoadBalancer, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Update it
	if opts.Name != "" {
		c.loadBalancerCache.idMap[lb.ID].Name = opts.Name
	}

	// Update it
	if opts.Labels != nil {
		c.loadBalancerCache.idMap[lb.ID].Labels = opts.Labels
	}

	return c.loadBalancerCache.idMap[lb.ID], nil
}

func (c *cacheHCloudClient) AddTargetServerToLoadBalancer(_ context.Context, opts hcloud.LoadBalancerAddServerTargetOpts, lb *hcloud.LoadBalancer) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeServer && s.Server.Server.ID == opts.Server.ID {
			return hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAdded, Message: "already added"}
		}
	}

	// Add it
	c.loadBalancerCache.idMap[lb.ID].Targets = append(
		c.loadBalancerCache.idMap[lb.ID].Targets,
		hcloud.LoadBalancerTarget{
			Type:   hcloud.LoadBalancerTargetTypeServer,
			Server: &hcloud.LoadBalancerTargetServer{Server: opts.Server},
		},
	)
	return nil
}

func (c *cacheHCloudClient) DeleteTargetServerOfLoadBalancer(_ context.Context, lb *hcloud.LoadBalancer, server *hcloud.Server) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// delete it if it exists
	for i, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeServer && s.Server.Server.ID == server.ID {
			// Truncate the slice
			c.loadBalancerCache.idMap[lb.ID].Targets[i] = c.loadBalancerCache.idMap[lb.ID].Targets[len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			c.loadBalancerCache.idMap[lb.ID].Targets = c.loadBalancerCache.idMap[lb.ID].Targets[:len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			return nil
		}
	}
	return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
}

func (c *cacheHCloudClient) AddIPTargetToLoadBalancer(_ context.Context, opts hcloud.LoadBalancerAddIPTargetOpts, lb *hcloud.LoadBalancer) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeIP && s.IP.IP == opts.IP.String() {
			return hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAdded, Message: "already added"}
		}
	}

	// Add it
	c.loadBalancerCache.idMap[lb.ID].Targets = append(
		c.loadBalancerCache.idMap[lb.ID].Targets,
		hcloud.LoadBalancerTarget{
			Type: hcloud.LoadBalancerTargetTypeIP,
			IP:   &hcloud.LoadBalancerTargetIP{IP: opts.IP.String()},
		},
	)
	return nil
}

func (c *cacheHCloudClient) DeleteIPTargetOfLoadBalancer(_ context.Context, lb *hcloud.LoadBalancer, ip net.IP) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// delete it if it exists
	for i, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeIP && s.IP.IP == ip.String() {
			// Truncate the slice
			c.loadBalancerCache.idMap[lb.ID].Targets[i] = c.loadBalancerCache.idMap[lb.ID].Targets[len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			c.loadBalancerCache.idMap[lb.ID].Targets = c.loadBalancerCache.idMap[lb.ID].Targets[:len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			return nil
		}
	}
	return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
}

func (c *cacheHCloudClient) AddServiceToLoadBalancer(_ context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerAddServiceOpts) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	if *opts.ListenPort == 0 {
		return fmt.Errorf("cannot add service with listenPort 0")
	}
	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].Services {
		if s.ListenPort == *opts.ListenPort {
			return fmt.Errorf("already added")
		}
	}

	// Add it
	c.loadBalancerCache.idMap[lb.ID].Services = append(
		c.loadBalancerCache.idMap[lb.ID].Services, hcloud.LoadBalancerService{ListenPort: *opts.ListenPort, DestinationPort: *opts.DestinationPort})
	return nil
}

func (c *cacheHCloudClient) DeleteServiceFromLoadBalancer(_ context.Context, lb *hcloud.LoadBalancer, listenPort int) error {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// delete if it exists
	for i, s := range c.loadBalancerCache.idMap[lb.ID].Services {
		if s.ListenPort == listenPort {
			c.loadBalancerCache.idMap[lb.ID].Services[i] = c.loadBalancerCache.idMap[lb.ID].Services[len(c.loadBalancerCache.idMap[lb.ID].Services)-1]
			c.loadBalancerCache.idMap[lb.ID].Services = c.loadBalancerCache.idMap[lb.ID].Services[:len(c.loadBalancerCache.idMap[lb.ID].Services)-1]
			return nil
		}
	}

	return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
}

func (c *cacheHCloudClient) ListImages(_ context.Context, opts hcloud.ImageListOpts) ([]*hcloud.Image, error) {
	if opts.Name != "" {
		return nil, nil
	}
	return []*hcloud.Image{&defaultImage}, nil
}

func (c *cacheHCloudClient) CreateServer(_ context.Context, opts hcloud.ServerCreateOpts) (*hcloud.Server, error) {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()

	if _, found := c.serverCache.nameMap[opts.Name]; found {
		return nil, fmt.Errorf("already exists")
	}

	c.serverIDCounter++
	server := &hcloud.Server{
		ID:             c.serverIDCounter,
		Name:           opts.Name,
		Labels:         opts.Labels,
		Image:          opts.Image,
		ServerType:     opts.ServerType,
		PlacementGroup: opts.PlacementGroup,
		Status:         hcloud.ServerStatusRunning,
	}

	for _, network := range opts.Networks {
		server.PrivateNet = append(server.PrivateNet, hcloud.ServerPrivateNet{IP: c.networkCache.idMap[network.ID].IPRange.IP})
	}

	// Add server to cache
	c.serverCache.idMap[server.ID] = server
	c.serverCache.nameMap[server.Name] = struct{}{}
	return server, nil
}

func (c *cacheHCloudClient) AttachServerToNetwork(_ context.Context, server *hcloud.Server, opts hcloud.ServerAttachToNetworkOpts) error {
	// Check if network exists
	if _, found := c.networkCache.idMap[opts.Network.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Check if server exists
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.networkCache.idMap[opts.Network.ID].Servers {
		if s.ID == server.ID {
			return hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAttached, Message: "already attached"}
		}
	}

	// Add it
	c.networkCache.idMap[opts.Network.ID].Servers = append(c.networkCache.idMap[opts.Network.ID].Servers, server)
	return nil
}

func (c *cacheHCloudClient) ListServers(_ context.Context, opts hcloud.ServerListOpts) ([]*hcloud.Server, error) {
	servers := make([]*hcloud.Server, 0, len(c.serverCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to convert label selector to labels: %w", err)
	}

	for _, s := range c.serverCache.idMap {
		allLabelsFound := true
		for key, label := range labels {
			if val, found := s.Labels[key]; !found || val != label {
				allLabelsFound = false
				break
			}
		}
		if allLabelsFound {
			servers = append(servers, s)
		}
	}
	return servers, nil
}

func (c *cacheHCloudClient) GetServer(_ context.Context, id int64) (*hcloud.Server, error) {
	return c.serverCache.idMap[id], nil
}

func (c *cacheHCloudClient) ShutdownServer(_ context.Context, server *hcloud.Server) error {
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	c.serverCache.idMap[server.ID].Status = hcloud.ServerStatusOff
	return nil
}

func (c *cacheHCloudClient) RebootServer(_ context.Context, _ *hcloud.Server) error {
	return nil
}

func (c *cacheHCloudClient) PowerOnServer(_ context.Context, server *hcloud.Server) error {
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	c.serverCache.idMap[server.ID].Status = hcloud.ServerStatusRunning
	return nil
}

func (c *cacheHCloudClient) DeleteServer(_ context.Context, server *hcloud.Server) error {
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	n := c.serverCache.idMap[server.ID]
	delete(c.serverCache.nameMap, n.Name)
	delete(c.serverCache.idMap, server.ID)
	return nil
}

func (c *cacheHCloudClient) ListServerTypes(_ context.Context) ([]*hcloud.ServerType, error) {
	return []*hcloud.ServerType{
		{
			ID:           1,
			Name:         "cpx11",
			Cores:        DefaultCPUCores,
			Memory:       DefaultMemoryInGB,
			Architecture: DefaultArchitecture,
		},
		{
			ID:           2,
			Name:         "cpx21",
			Cores:        DefaultCPUCores,
			Memory:       DefaultMemoryInGB,
			Architecture: DefaultArchitecture,
		},
		{
			ID:           3,
			Name:         "cpx31",
			Cores:        DefaultCPUCores,
			Memory:       DefaultMemoryInGB,
			Architecture: DefaultArchitecture,
		},
	}, nil
}

func (c *cacheHCloudClient) GetServerType(_ context.Context, name string) (*hcloud.ServerType, error) {
	serverType := &hcloud.ServerType{
		Cores:        DefaultCPUCores,
		Memory:       DefaultMemoryInGB,
		Architecture: DefaultArchitecture,
	}
	switch name {
	case "cpx11":
		serverType.ID = 1
		serverType.Name = "cpx11"
	case "cpx21":
		serverType.ID = 2
		serverType.Name = "cpx21"
	case "cpx31":
		serverType.ID = 3
		serverType.Name = "cpx31"
	default:
		return nil, nil
	}

	return serverType, nil
}

func (c *cacheHCloudClient) CreateNetwork(_ context.Context, opts hcloud.NetworkCreateOpts) (*hcloud.Network, error) {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()

	if _, found := c.networkCache.nameMap[opts.Name]; found {
		return nil, fmt.Errorf("already exists")
	}

	c.networkIDCounter++
	network := &hcloud.Network{
		ID:      c.networkIDCounter,
		Name:    opts.Name,
		Labels:  opts.Labels,
		IPRange: opts.IPRange,
		Subnets: opts.Subnets,
	}

	// Add network to cache
	c.networkCache.idMap[network.ID] = network
	c.networkCache.nameMap[network.Name] = struct{}{}
	return network, nil
}

func (c *cacheHCloudClient) ListNetworks(_ context.Context, opts hcloud.NetworkListOpts) ([]*hcloud.Network, error) {
	networks := make([]*hcloud.Network, 0, len(c.networkCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to convert label selector to labels: %w", err)
	}

	for _, network := range c.networkCache.idMap {
		allLabelsFound := true
		for key, label := range labels {
			if val, found := network.Labels[key]; !found || val != label {
				allLabelsFound = false
				break
			}
		}
		if allLabelsFound {
			networks = append(networks, network)
		}
	}

	return networks, nil
}

func (c *cacheHCloudClient) DeleteNetwork(_ context.Context, network *hcloud.Network) error {
	if _, found := c.networkCache.idMap[network.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	n := c.networkCache.idMap[network.ID]
	delete(c.networkCache.nameMap, n.Name)
	delete(c.networkCache.idMap, network.ID)
	return nil
}

func (c *cacheHCloudClient) ListSSHKeys(_ context.Context, _ hcloud.SSHKeyListOpts) ([]*hcloud.SSHKey, error) {
	return []*hcloud.SSHKey{&defaultSSHKey}, nil
}

func (c *cacheHCloudClient) CreatePlacementGroup(_ context.Context, opts hcloud.PlacementGroupCreateOpts) (*hcloud.PlacementGroup, error) {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()

	if _, found := c.placementGroupCache.nameMap[opts.Name]; found {
		return nil, fmt.Errorf("already exists")
	}

	c.placementGroupIDCounter++
	placementGroup := &hcloud.PlacementGroup{
		ID:     c.placementGroupIDCounter,
		Name:   opts.Name,
		Labels: opts.Labels,
		Type:   opts.Type,
	}

	// Add placementGroup to cache
	c.placementGroupCache.idMap[placementGroup.ID] = placementGroup
	c.placementGroupCache.nameMap[placementGroup.Name] = struct{}{}
	return placementGroup, nil
}

func (c *cacheHCloudClient) DeletePlacementGroup(_ context.Context, id int64) error {
	if _, found := c.placementGroupCache.idMap[id]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	n := c.placementGroupCache.idMap[id]

	delete(c.placementGroupCache.nameMap, n.Name)
	delete(c.placementGroupCache.idMap, id)
	return nil
}

func (c *cacheHCloudClient) ListPlacementGroups(_ context.Context, opts hcloud.PlacementGroupListOpts) ([]*hcloud.PlacementGroup, error) {
	placementGroups := make([]*hcloud.PlacementGroup, 0, len(c.placementGroupCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to convert label selector to labels: %w", err)
	}

	for _, pg := range c.placementGroupCache.idMap {
		allLabelsFound := true
		for key, label := range labels {
			if val, found := pg.Labels[key]; !found || val != label {
				allLabelsFound = false
				break
			}
		}
		if allLabelsFound {
			placementGroups = append(placementGroups, pg)
		}
	}

	return placementGroups, nil
}

func (c *cacheHCloudClient) AddServerToPlacementGroup(_ context.Context, server *hcloud.Server, pg *hcloud.PlacementGroup) error {
	// Check if placement group exists
	if _, found := c.placementGroupCache.idMap[pg.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Check if server exists
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	if isIntInList(c.placementGroupCache.idMap[pg.ID].Servers, server.ID) {
		return hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAdded, Message: "already added"}
	}

	// Add it
	c.placementGroupCache.idMap[pg.ID].Servers = append(c.placementGroupCache.idMap[pg.ID].Servers, server.ID)
	return nil
}

func isIntInList(list []int64, str int64) bool {
	for _, s := range list {
		if s == str {
			return true
		}
	}
	return false
}
