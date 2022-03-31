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

// Package fake implements fakes for important interfaces like the HCloud api.
package fake

import (
	"context"
	"fmt"
	"net"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

type cacheHCloudClient struct {
	serverCache         serverCache
	placementGroupCache placementGroupCache
	loadBalancerCache   loadBalancerCache
	networkCache        networkCache
}

// NewClient gives reference to the fake client using cache for HCloud API.
func (f *cacheHCloudClientFactory) NewClient(hcloudToken string) hcloudclient.Client {
	return cacheHCloudClientInstance
}

// Close implements Close method of hcloud client interface.
func (c *cacheHCloudClient) Close() {
	cacheHCloudClientInstance.serverCache = serverCache{}
	cacheHCloudClientInstance.networkCache = networkCache{}
	cacheHCloudClientInstance.loadBalancerCache = loadBalancerCache{}
	cacheHCloudClientInstance.placementGroupCache = placementGroupCache{}

	cacheHCloudClientInstance.serverCache = serverCache{
		idMap:   make(map[int]*hcloud.Server),
		nameMap: make(map[string]struct{}),
	}
	cacheHCloudClientInstance.placementGroupCache = placementGroupCache{
		idMap:   make(map[int]*hcloud.PlacementGroup),
		nameMap: make(map[string]struct{}),
	}
	cacheHCloudClientInstance.loadBalancerCache = loadBalancerCache{
		idMap:   make(map[int]*hcloud.LoadBalancer),
		nameMap: make(map[string]struct{}),
	}
	cacheHCloudClientInstance.networkCache = networkCache{
		idMap:   make(map[int]*hcloud.Network),
		nameMap: make(map[string]struct{}),
	}
}

type cacheHCloudClientFactory struct{}

var cacheHCloudClientInstance = &cacheHCloudClient{
	serverCache: serverCache{
		idMap:   make(map[int]*hcloud.Server),
		nameMap: make(map[string]struct{}),
	},
	placementGroupCache: placementGroupCache{
		idMap:   make(map[int]*hcloud.PlacementGroup),
		nameMap: make(map[string]struct{}),
	},
	loadBalancerCache: loadBalancerCache{
		idMap:   make(map[int]*hcloud.LoadBalancer),
		nameMap: make(map[string]struct{}),
	},
	networkCache: networkCache{
		idMap:   make(map[int]*hcloud.Network),
		nameMap: make(map[string]struct{}),
	},
}

// NewHCloudClientFactory creates new fake HCloud client factories using cache.
func NewHCloudClientFactory() hcloudclient.Factory {
	return &cacheHCloudClientFactory{}
}

var _ = hcloudclient.Factory(&cacheHCloudClientFactory{})

type serverCache struct {
	idMap   map[int]*hcloud.Server
	nameMap map[string]struct{}
}

type loadBalancerCache struct {
	idMap   map[int]*hcloud.LoadBalancer
	nameMap map[string]struct{}
}

type networkCache struct {
	idMap   map[int]*hcloud.Network
	nameMap map[string]struct{}
}

type placementGroupCache struct {
	idMap   map[int]*hcloud.PlacementGroup
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

func (c *cacheHCloudClient) CreateLoadBalancer(ctx context.Context, opts hcloud.LoadBalancerCreateOpts) (hcloud.LoadBalancerCreateResult, error) {
	// cannot have two load balancers with the same name
	if _, found := c.loadBalancerCache.nameMap[opts.Name]; found {
		return hcloud.LoadBalancerCreateResult{}, fmt.Errorf("failed to create lb: already exists")
	}

	lb := &hcloud.LoadBalancer{
		ID:               len(c.loadBalancerCache.idMap) + 1,
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
	return hcloud.LoadBalancerCreateResult{
		LoadBalancer: lb,
		Action:       &hcloud.Action{},
	}, nil
}

func (c *cacheHCloudClient) DeleteLoadBalancer(ctx context.Context, id int) error {
	if _, found := c.loadBalancerCache.idMap[id]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	lb := c.loadBalancerCache.idMap[id]
	delete(c.loadBalancerCache.nameMap, lb.Name)
	delete(c.loadBalancerCache.idMap, id)
	return nil
}

func (c *cacheHCloudClient) ListLoadBalancers(ctx context.Context, opts hcloud.LoadBalancerListOpts) ([]*hcloud.LoadBalancer, error) {
	lbs := make([]*hcloud.LoadBalancer, 0, len(c.loadBalancerCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert label selector to labels")
	}

	for _, lb := range c.loadBalancerCache.idMap {
		allLabelsFound := true
		for key, label := range labels {
			if val, found := lb.Labels[key]; !found || val != label {
				allLabelsFound = false
				break
			}
		}
		if allLabelsFound {
			lbs = append(lbs, lb)
		}
	}
	return lbs, nil
}

func (c *cacheHCloudClient) AttachLoadBalancerToNetwork(ctx context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerAttachToNetworkOpts) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Check if network exists
	network, found := c.networkCache.idMap[opts.Network.ID]
	if !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].PrivateNet {
		if s.IP.Equal(network.IPRange.IP) {
			return nil, fmt.Errorf("already added")
		}
	}

	// Add it
	c.loadBalancerCache.idMap[lb.ID].PrivateNet = append(
		c.loadBalancerCache.idMap[lb.ID].PrivateNet,
		hcloud.LoadBalancerPrivateNet{IP: network.IPRange.IP},
	)
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) ChangeLoadBalancerType(ctx context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerChangeTypeOpts) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Update it
	c.loadBalancerCache.idMap[lb.ID].LoadBalancerType = opts.LoadBalancerType
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) ChangeLoadBalancerAlgorithm(ctx context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerChangeAlgorithmOpts) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Update it
	c.loadBalancerCache.idMap[lb.ID].Algorithm.Type = opts.Type
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) UpdateLoadBalancer(ctx context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerUpdateOpts) (*hcloud.LoadBalancer, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Update it
	c.loadBalancerCache.idMap[lb.ID].Name = opts.Name
	return c.loadBalancerCache.idMap[lb.ID], nil
}

func (c *cacheHCloudClient) AddTargetServerToLoadBalancer(ctx context.Context, opts hcloud.LoadBalancerAddServerTargetOpts, lb *hcloud.LoadBalancer) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeServer && s.Server.Server.ID == opts.Server.ID {
			return nil, hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAdded, Message: "already added"}
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
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) DeleteTargetServerOfLoadBalancer(ctx context.Context, lb *hcloud.LoadBalancer, server *hcloud.Server) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// delete it if it exists
	for i, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeServer && s.Server.Server.ID == server.ID {
			// Truncate the slice
			c.loadBalancerCache.idMap[lb.ID].Targets[i] = c.loadBalancerCache.idMap[lb.ID].Targets[len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			c.loadBalancerCache.idMap[lb.ID].Targets = c.loadBalancerCache.idMap[lb.ID].Targets[:len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			return &hcloud.Action{}, nil
		}
	}
	return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
}

func (c *cacheHCloudClient) AddIPTargetToLoadBalancer(ctx context.Context, opts hcloud.LoadBalancerAddIPTargetOpts, lb *hcloud.LoadBalancer) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeIP && s.IP.IP == opts.IP.String() {
			return nil, hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAdded, Message: "already added"}
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
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) DeleteIPTargetOfLoadBalancer(ctx context.Context, lb *hcloud.LoadBalancer, ip net.IP) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// delete it if it exists
	for i, s := range c.loadBalancerCache.idMap[lb.ID].Targets {
		if s.Type == hcloud.LoadBalancerTargetTypeIP && s.IP.IP == ip.String() {
			// Truncate the slice
			c.loadBalancerCache.idMap[lb.ID].Targets[i] = c.loadBalancerCache.idMap[lb.ID].Targets[len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			c.loadBalancerCache.idMap[lb.ID].Targets = c.loadBalancerCache.idMap[lb.ID].Targets[:len(c.loadBalancerCache.idMap[lb.ID].Targets)-1]
			return &hcloud.Action{}, nil
		}
	}
	return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
}

func (c *cacheHCloudClient) AddServiceToLoadBalancer(ctx context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerAddServiceOpts) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	if *opts.ListenPort == 0 {
		return nil, fmt.Errorf("cannot add service with listenPort 0")
	}
	// check if already exists
	for _, s := range c.loadBalancerCache.idMap[lb.ID].Services {
		if s.ListenPort == *opts.ListenPort {
			return nil, fmt.Errorf("already added")
		}
	}

	// Add it
	c.loadBalancerCache.idMap[lb.ID].Services = append(
		c.loadBalancerCache.idMap[lb.ID].Services, hcloud.LoadBalancerService{ListenPort: *opts.ListenPort, DestinationPort: *opts.DestinationPort})
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) DeleteServiceFromLoadBalancer(ctx context.Context, lb *hcloud.LoadBalancer, listenPort int) (*hcloud.Action, error) {
	// Check if loadBalancer exists
	if _, found := c.loadBalancerCache.idMap[lb.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// delete if it exists
	for i, s := range c.loadBalancerCache.idMap[lb.ID].Services {
		if s.ListenPort == listenPort {
			c.loadBalancerCache.idMap[lb.ID].Services[i] = c.loadBalancerCache.idMap[lb.ID].Services[len(c.loadBalancerCache.idMap[lb.ID].Services)-1]
			c.loadBalancerCache.idMap[lb.ID].Services = c.loadBalancerCache.idMap[lb.ID].Services[:len(c.loadBalancerCache.idMap[lb.ID].Services)-1]
			return &hcloud.Action{}, nil
		}
	}

	return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
}

func (c *cacheHCloudClient) ListImages(ctx context.Context, opts hcloud.ImageListOpts) ([]*hcloud.Image, error) {
	if opts.Name != "" {
		return nil, nil
	}
	return []*hcloud.Image{&defaultImage}, nil
}

func (c *cacheHCloudClient) CreateServer(ctx context.Context, opts hcloud.ServerCreateOpts) (hcloud.ServerCreateResult, error) {
	if _, found := c.serverCache.nameMap[opts.Name]; found {
		return hcloud.ServerCreateResult{}, fmt.Errorf("already exists")
	}

	server := &hcloud.Server{
		ID:             len(c.serverCache.idMap) + 1,
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
	return hcloud.ServerCreateResult{
		Server: server,
		Action: &hcloud.Action{},
	}, nil
}

func (c *cacheHCloudClient) AttachServerToNetwork(ctx context.Context, server *hcloud.Server, opts hcloud.ServerAttachToNetworkOpts) (*hcloud.Action, error) {
	// Check if network exists
	if _, found := c.networkCache.idMap[opts.Network.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Check if server exists
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	for _, s := range c.networkCache.idMap[opts.Network.ID].Servers {
		if s.ID == server.ID {
			return nil, hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAdded, Message: "already added"}
		}
	}

	// Add it
	c.networkCache.idMap[opts.Network.ID].Servers = append(c.networkCache.idMap[opts.Network.ID].Servers, server)
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) ListServers(ctx context.Context, opts hcloud.ServerListOpts) ([]*hcloud.Server, error) {
	servers := make([]*hcloud.Server, 0, len(c.serverCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert label selector to labels")
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

func (c *cacheHCloudClient) ShutdownServer(ctx context.Context, server *hcloud.Server) (*hcloud.Action, error) {
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	c.serverCache.idMap[server.ID].Status = hcloud.ServerStatusOff
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) PowerOnServer(ctx context.Context, server *hcloud.Server) (*hcloud.Action, error) {
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	c.serverCache.idMap[server.ID].Status = hcloud.ServerStatusRunning
	return &hcloud.Action{}, nil
}

func (c *cacheHCloudClient) DeleteServer(ctx context.Context, server *hcloud.Server) error {
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	n := c.serverCache.idMap[server.ID]
	delete(c.serverCache.nameMap, n.Name)
	delete(c.serverCache.idMap, server.ID)
	return nil
}

func (c *cacheHCloudClient) CreateNetwork(ctx context.Context, opts hcloud.NetworkCreateOpts) (*hcloud.Network, error) {
	if _, found := c.networkCache.nameMap[opts.Name]; found {
		return nil, fmt.Errorf("already exists")
	}

	network := &hcloud.Network{
		ID:      len(c.networkCache.idMap) + 1,
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

func (c *cacheHCloudClient) ListNetworks(ctx context.Context, opts hcloud.NetworkListOpts) ([]*hcloud.Network, error) {
	networks := make([]*hcloud.Network, 0, len(c.networkCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert label selector to labels")
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

func (c *cacheHCloudClient) DeleteNetwork(ctx context.Context, network *hcloud.Network) error {
	if _, found := c.networkCache.idMap[network.ID]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	n := c.networkCache.idMap[network.ID]
	delete(c.networkCache.nameMap, n.Name)
	delete(c.networkCache.idMap, network.ID)
	return nil
}

func (c *cacheHCloudClient) ListSSHKeys(ctx context.Context, opts hcloud.SSHKeyListOpts) ([]*hcloud.SSHKey, error) {
	return []*hcloud.SSHKey{&defaultSSHKey}, nil
}

func (c *cacheHCloudClient) CreatePlacementGroup(ctx context.Context, opts hcloud.PlacementGroupCreateOpts) (hcloud.PlacementGroupCreateResult, error) {
	if _, found := c.placementGroupCache.nameMap[opts.Name]; found {
		return hcloud.PlacementGroupCreateResult{}, fmt.Errorf("already exists")
	}

	placementGroup := &hcloud.PlacementGroup{
		ID:     len(c.placementGroupCache.idMap) + 1,
		Name:   opts.Name,
		Labels: opts.Labels,
		Type:   opts.Type,
	}

	// Add placementGroup to cache
	c.placementGroupCache.idMap[placementGroup.ID] = placementGroup
	c.placementGroupCache.nameMap[placementGroup.Name] = struct{}{}

	return hcloud.PlacementGroupCreateResult{
		PlacementGroup: placementGroup,
		Action:         &hcloud.Action{},
	}, nil
}

func (c *cacheHCloudClient) DeletePlacementGroup(ctx context.Context, id int) error {
	if _, found := c.placementGroupCache.idMap[id]; !found {
		return hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}
	n := c.placementGroupCache.idMap[id]
	delete(c.placementGroupCache.nameMap, n.Name)
	delete(c.placementGroupCache.idMap, id)
	return nil
}

func (c *cacheHCloudClient) ListPlacementGroups(ctx context.Context, opts hcloud.PlacementGroupListOpts) ([]*hcloud.PlacementGroup, error) {
	placementGroups := make([]*hcloud.PlacementGroup, 0, len(c.placementGroupCache.idMap))

	labels, err := utils.LabelSelectorToLabels(opts.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert label selector to labels")
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

func (c *cacheHCloudClient) AddServerToPlacementGroup(ctx context.Context, server *hcloud.Server, pg *hcloud.PlacementGroup) (*hcloud.Action, error) {
	// Check if placement group exists
	if _, found := c.placementGroupCache.idMap[pg.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// Check if server exists
	if _, found := c.serverCache.idMap[server.ID]; !found {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeNotFound, Message: "not found"}
	}

	// check if already exists
	if isIntInList(c.placementGroupCache.idMap[pg.ID].Servers, server.ID) {
		return nil, hcloud.Error{Code: hcloud.ErrorCodeServerAlreadyAdded, Message: "already added"}
	}

	// Add it
	c.placementGroupCache.idMap[pg.ID].Servers = append(c.placementGroupCache.idMap[pg.ID].Servers, server.ID)
	return &hcloud.Action{}, nil
}

func isIntInList(list []int, str int) bool {
	for _, s := range list {
		if s == str {
			return true
		}
	}
	return false
}
