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

package scope

import (
	"context"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

// HCloudClient collects all methods used by the controller in the hcloud cloud API.
type HCloudClient interface {
	Token() string
	ListLocation(context.Context) ([]*hcloud.Location, error)
	CreateLoadBalancer(context.Context, hcloud.LoadBalancerCreateOpts) (hcloud.LoadBalancerCreateResult, *hcloud.Response, error)
	DeleteLoadBalancer(context.Context, int) (*hcloud.Response, error)
	GetLoadBalancer(context.Context, int) (*hcloud.LoadBalancer, *hcloud.Response, error)
	ListLoadBalancers(context.Context, hcloud.LoadBalancerListOpts) ([]*hcloud.LoadBalancer, error)
	AttachLoadBalancerToNetwork(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerAttachToNetworkOpts) (*hcloud.Action, *hcloud.Response, error)
	GetLoadBalancerTypeByName(context.Context, string) (*hcloud.LoadBalancerType, *hcloud.Response, error)
	AddTargetServerToLoadBalancer(context.Context, hcloud.LoadBalancerAddServerTargetOpts, *hcloud.LoadBalancer) (*hcloud.Action, *hcloud.Response, error)
	DeleteTargetServerOfLoadBalancer(context.Context, *hcloud.LoadBalancer, *hcloud.Server) (*hcloud.Action, *hcloud.Response, error)
	AddServiceToLoadBalancer(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerAddServiceOpts) (*hcloud.Action, *hcloud.Response, error)
	ListImages(context.Context, hcloud.ImageListOpts) ([]*hcloud.Image, error)
	CreateServer(context.Context, hcloud.ServerCreateOpts) (hcloud.ServerCreateResult, *hcloud.Response, error)
	AttachServerToNetwork(context.Context, *hcloud.Server, hcloud.ServerAttachToNetworkOpts) (*hcloud.Action, *hcloud.Response, error)
	ListServers(context.Context, hcloud.ServerListOpts) ([]*hcloud.Server, error)
	GetServerByID(context.Context, int) (*hcloud.Server, *hcloud.Response, error)
	DeleteServer(context.Context, *hcloud.Server) (*hcloud.Response, error)
	PowerOnServer(context.Context, *hcloud.Server) (*hcloud.Action, *hcloud.Response, error)
	ShutdownServer(context.Context, *hcloud.Server) (*hcloud.Action, *hcloud.Response, error)
	CreateNetwork(context.Context, hcloud.NetworkCreateOpts) (*hcloud.Network, *hcloud.Response, error)
	GetNetwork(context.Context, int) (*hcloud.Network, *hcloud.Response, error)
	ListNetworks(context.Context, hcloud.NetworkListOpts) ([]*hcloud.Network, error)
	DeleteNetwork(context.Context, *hcloud.Network) (*hcloud.Response, error)
	ListSSHKeys(ctx context.Context, opts hcloud.SSHKeyListOpts) ([]*hcloud.SSHKey, *hcloud.Response, error)
	CreatePlacementGroup(context.Context, hcloud.PlacementGroupCreateOpts) (hcloud.PlacementGroupCreateResult, *hcloud.Response, error)
	DeletePlacementGroup(context.Context, int) (*hcloud.Response, error)
	GetPlacementGroup(context.Context, int) (*hcloud.PlacementGroup, *hcloud.Response, error)
	ListPlacementGroups(context.Context, hcloud.PlacementGroupListOpts) ([]*hcloud.PlacementGroup, error)
	AddServerToPlacementGroup(context.Context, *hcloud.Server, *hcloud.PlacementGroup) (*hcloud.Action, *hcloud.Response, error)
}

// HCloudClientFactory implements a factory function for HCloudClient repository.
type HCloudClientFactory func(context.Context) (HCloudClient, error)

var _ HCloudClient = &realHCloudClient{}

type realHCloudClient struct {
	client *hcloud.Client
	token  string
}

func (c *realHCloudClient) Token() string {
	return c.token
}

func (c *realHCloudClient) ListLocation(ctx context.Context) ([]*hcloud.Location, error) {
	return c.client.Location.All(ctx)
}

func (c *realHCloudClient) CreateLoadBalancer(ctx context.Context, opts hcloud.LoadBalancerCreateOpts) (hcloud.LoadBalancerCreateResult, *hcloud.Response, error) {
	return c.client.LoadBalancer.Create(ctx, opts)
}

func (c *realHCloudClient) DeleteLoadBalancer(ctx context.Context, id int) (*hcloud.Response, error) {
	return c.client.LoadBalancer.Delete(ctx, &hcloud.LoadBalancer{
		ID: id,
	})
}

func (c *realHCloudClient) GetLoadBalancer(ctx context.Context, id int) (*hcloud.LoadBalancer, *hcloud.Response, error) {
	return c.client.LoadBalancer.GetByID(ctx, id)
}

func (c *realHCloudClient) ListLoadBalancers(ctx context.Context, opts hcloud.LoadBalancerListOpts) ([]*hcloud.LoadBalancer, error) {
	return c.client.LoadBalancer.AllWithOpts(ctx, opts)
}

func (c *realHCloudClient) AttachLoadBalancerToNetwork(ctx context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerAttachToNetworkOpts) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.LoadBalancer.AttachToNetwork(ctx, lb, opts)
}

func (c *realHCloudClient) GetLoadBalancerTypeByName(ctx context.Context, name string) (*hcloud.LoadBalancerType, *hcloud.Response, error) {
	return c.client.LoadBalancerType.GetByName(ctx, name)
}

func (c *realHCloudClient) AddTargetServerToLoadBalancer(ctx context.Context, opts hcloud.LoadBalancerAddServerTargetOpts, lb *hcloud.LoadBalancer) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.LoadBalancer.AddServerTarget(ctx, lb, opts)
}

func (c *realHCloudClient) DeleteTargetServerOfLoadBalancer(ctx context.Context, lb *hcloud.LoadBalancer, server *hcloud.Server) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.LoadBalancer.RemoveServerTarget(ctx, lb, server)
}

func (c *realHCloudClient) AddServiceToLoadBalancer(ctx context.Context, lb *hcloud.LoadBalancer, opts hcloud.LoadBalancerAddServiceOpts) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.LoadBalancer.AddService(ctx, lb, opts)
}

func (c *realHCloudClient) ListImages(ctx context.Context, opts hcloud.ImageListOpts) ([]*hcloud.Image, error) {
	return c.client.Image.AllWithOpts(ctx, opts)
}

func (c *realHCloudClient) CreateServer(ctx context.Context, opts hcloud.ServerCreateOpts) (hcloud.ServerCreateResult, *hcloud.Response, error) {
	return c.client.Server.Create(ctx, opts)
}

func (c *realHCloudClient) AttachServerToNetwork(ctx context.Context, server *hcloud.Server, opts hcloud.ServerAttachToNetworkOpts) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.Server.AttachToNetwork(ctx, server, opts)
}

func (c *realHCloudClient) ListServers(ctx context.Context, opts hcloud.ServerListOpts) ([]*hcloud.Server, error) {
	return c.client.Server.AllWithOpts(ctx, opts)
}

func (c *realHCloudClient) GetServerByID(ctx context.Context, id int) (*hcloud.Server, *hcloud.Response, error) {
	return c.client.Server.GetByID(ctx, id)
}

func (c *realHCloudClient) ShutdownServer(ctx context.Context, server *hcloud.Server) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.Server.Shutdown(ctx, server)
}

func (c *realHCloudClient) PowerOnServer(ctx context.Context, server *hcloud.Server) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.Server.Poweron(ctx, server)
}

func (c *realHCloudClient) DeleteServer(ctx context.Context, server *hcloud.Server) (*hcloud.Response, error) {
	return c.client.Server.Delete(ctx, server)
}

func (c *realHCloudClient) CreateNetwork(ctx context.Context, opts hcloud.NetworkCreateOpts) (*hcloud.Network, *hcloud.Response, error) {
	return c.client.Network.Create(ctx, opts)
}

func (c *realHCloudClient) GetNetwork(ctx context.Context, id int) (*hcloud.Network, *hcloud.Response, error) {
	return c.client.Network.GetByID(ctx, id)
}

func (c *realHCloudClient) ListNetworks(ctx context.Context, opts hcloud.NetworkListOpts) ([]*hcloud.Network, error) {
	return c.client.Network.AllWithOpts(ctx, opts)
}

func (c *realHCloudClient) DeleteNetwork(ctx context.Context, network *hcloud.Network) (*hcloud.Response, error) {
	return c.client.Network.Delete(ctx, network)
}

func (c *realHCloudClient) ListSSHKeys(ctx context.Context, opts hcloud.SSHKeyListOpts) ([]*hcloud.SSHKey, *hcloud.Response, error) {
	return c.client.SSHKey.List(ctx, opts)
}

func (c *realHCloudClient) CreatePlacementGroup(ctx context.Context, opts hcloud.PlacementGroupCreateOpts) (hcloud.PlacementGroupCreateResult, *hcloud.Response, error) {
	return c.client.PlacementGroup.Create(ctx, opts)
}

func (c *realHCloudClient) DeletePlacementGroup(ctx context.Context, id int) (*hcloud.Response, error) {
	return c.client.PlacementGroup.Delete(ctx, &hcloud.PlacementGroup{ID: id})
}

func (c *realHCloudClient) ListPlacementGroups(ctx context.Context, opts hcloud.PlacementGroupListOpts) ([]*hcloud.PlacementGroup, error) {
	return c.client.PlacementGroup.AllWithOpts(ctx, opts)
}

func (c *realHCloudClient) GetPlacementGroup(ctx context.Context, id int) (*hcloud.PlacementGroup, *hcloud.Response, error) {
	return c.client.PlacementGroup.GetByID(ctx, id)
}

func (c *realHCloudClient) AddServerToPlacementGroup(ctx context.Context, server *hcloud.Server, pg *hcloud.PlacementGroup) (*hcloud.Action, *hcloud.Response, error) {
	return c.client.Server.AddToPlacementGroup(ctx, server, pg)
}
