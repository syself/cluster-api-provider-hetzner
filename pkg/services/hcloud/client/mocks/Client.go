// Code generated by mockery v2.43.0. DO NOT EDIT.

package mocks

import (
	context "context"

	hcloud "github.com/hetznercloud/hcloud-go/v2/hcloud"

	mock "github.com/stretchr/testify/mock"

	net "net"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// AddIPTargetToLoadBalancer provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) AddIPTargetToLoadBalancer(_a0 context.Context, _a1 hcloud.LoadBalancerAddIPTargetOpts, _a2 *hcloud.LoadBalancer) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for AddIPTargetToLoadBalancer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.LoadBalancerAddIPTargetOpts, *hcloud.LoadBalancer) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddServerToPlacementGroup provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) AddServerToPlacementGroup(_a0 context.Context, _a1 *hcloud.Server, _a2 *hcloud.PlacementGroup) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for AddServerToPlacementGroup")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.Server, *hcloud.PlacementGroup) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddServiceToLoadBalancer provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) AddServiceToLoadBalancer(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 hcloud.LoadBalancerAddServiceOpts) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for AddServiceToLoadBalancer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerAddServiceOpts) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddTargetServerToLoadBalancer provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) AddTargetServerToLoadBalancer(_a0 context.Context, _a1 hcloud.LoadBalancerAddServerTargetOpts, _a2 *hcloud.LoadBalancer) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for AddTargetServerToLoadBalancer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.LoadBalancerAddServerTargetOpts, *hcloud.LoadBalancer) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AttachLoadBalancerToNetwork provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) AttachLoadBalancerToNetwork(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 hcloud.LoadBalancerAttachToNetworkOpts) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for AttachLoadBalancerToNetwork")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerAttachToNetworkOpts) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AttachServerToNetwork provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) AttachServerToNetwork(_a0 context.Context, _a1 *hcloud.Server, _a2 hcloud.ServerAttachToNetworkOpts) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for AttachServerToNetwork")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.Server, hcloud.ServerAttachToNetworkOpts) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ChangeLoadBalancerAlgorithm provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) ChangeLoadBalancerAlgorithm(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 hcloud.LoadBalancerChangeAlgorithmOpts) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for ChangeLoadBalancerAlgorithm")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerChangeAlgorithmOpts) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ChangeLoadBalancerType provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) ChangeLoadBalancerType(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 hcloud.LoadBalancerChangeTypeOpts) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for ChangeLoadBalancerType")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerChangeTypeOpts) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close provides a mock function with given fields:
func (_m *Client) Close() {
	_m.Called()
}

// CreateLoadBalancer provides a mock function with given fields: _a0, _a1
func (_m *Client) CreateLoadBalancer(_a0 context.Context, _a1 hcloud.LoadBalancerCreateOpts) (*hcloud.LoadBalancer, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreateLoadBalancer")
	}

	var r0 *hcloud.LoadBalancer
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.LoadBalancerCreateOpts) (*hcloud.LoadBalancer, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.LoadBalancerCreateOpts) *hcloud.LoadBalancer); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hcloud.LoadBalancer)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.LoadBalancerCreateOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateNetwork provides a mock function with given fields: _a0, _a1
func (_m *Client) CreateNetwork(_a0 context.Context, _a1 hcloud.NetworkCreateOpts) (*hcloud.Network, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreateNetwork")
	}

	var r0 *hcloud.Network
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.NetworkCreateOpts) (*hcloud.Network, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.NetworkCreateOpts) *hcloud.Network); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hcloud.Network)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.NetworkCreateOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreatePlacementGroup provides a mock function with given fields: _a0, _a1
func (_m *Client) CreatePlacementGroup(_a0 context.Context, _a1 hcloud.PlacementGroupCreateOpts) (*hcloud.PlacementGroup, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreatePlacementGroup")
	}

	var r0 *hcloud.PlacementGroup
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.PlacementGroupCreateOpts) (*hcloud.PlacementGroup, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.PlacementGroupCreateOpts) *hcloud.PlacementGroup); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hcloud.PlacementGroup)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.PlacementGroupCreateOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateServer provides a mock function with given fields: _a0, _a1
func (_m *Client) CreateServer(_a0 context.Context, _a1 hcloud.ServerCreateOpts) (*hcloud.Server, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreateServer")
	}

	var r0 *hcloud.Server
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.ServerCreateOpts) (*hcloud.Server, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.ServerCreateOpts) *hcloud.Server); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hcloud.Server)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.ServerCreateOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteIPTargetOfLoadBalancer provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) DeleteIPTargetOfLoadBalancer(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 net.IP) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for DeleteIPTargetOfLoadBalancer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, net.IP) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteLoadBalancer provides a mock function with given fields: _a0, _a1
func (_m *Client) DeleteLoadBalancer(_a0 context.Context, _a1 int64) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for DeleteLoadBalancer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteNetwork provides a mock function with given fields: _a0, _a1
func (_m *Client) DeleteNetwork(_a0 context.Context, _a1 *hcloud.Network) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for DeleteNetwork")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.Network) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeletePlacementGroup provides a mock function with given fields: _a0, _a1
func (_m *Client) DeletePlacementGroup(_a0 context.Context, _a1 int64) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for DeletePlacementGroup")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteServer provides a mock function with given fields: _a0, _a1
func (_m *Client) DeleteServer(_a0 context.Context, _a1 *hcloud.Server) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for DeleteServer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.Server) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteServiceFromLoadBalancer provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) DeleteServiceFromLoadBalancer(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 int) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for DeleteServiceFromLoadBalancer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, int) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteTargetServerOfLoadBalancer provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) DeleteTargetServerOfLoadBalancer(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 *hcloud.Server) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for DeleteTargetServerOfLoadBalancer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, *hcloud.Server) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetServer provides a mock function with given fields: _a0, _a1
func (_m *Client) GetServer(_a0 context.Context, _a1 int64) (*hcloud.Server, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetServer")
	}

	var r0 *hcloud.Server
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*hcloud.Server, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *hcloud.Server); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hcloud.Server)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetServerType provides a mock function with given fields: _a0, _a1
func (_m *Client) GetServerType(_a0 context.Context, _a1 string) (*hcloud.ServerType, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetServerType")
	}

	var r0 *hcloud.ServerType
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*hcloud.ServerType, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *hcloud.ServerType); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hcloud.ServerType)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListImages provides a mock function with given fields: _a0, _a1
func (_m *Client) ListImages(_a0 context.Context, _a1 hcloud.ImageListOpts) ([]*hcloud.Image, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListImages")
	}

	var r0 []*hcloud.Image
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.ImageListOpts) ([]*hcloud.Image, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.ImageListOpts) []*hcloud.Image); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hcloud.Image)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.ImageListOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListLoadBalancers provides a mock function with given fields: _a0, _a1
func (_m *Client) ListLoadBalancers(_a0 context.Context, _a1 hcloud.LoadBalancerListOpts) ([]*hcloud.LoadBalancer, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListLoadBalancers")
	}

	var r0 []*hcloud.LoadBalancer
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.LoadBalancerListOpts) ([]*hcloud.LoadBalancer, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.LoadBalancerListOpts) []*hcloud.LoadBalancer); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hcloud.LoadBalancer)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.LoadBalancerListOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListNetworks provides a mock function with given fields: _a0, _a1
func (_m *Client) ListNetworks(_a0 context.Context, _a1 hcloud.NetworkListOpts) ([]*hcloud.Network, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListNetworks")
	}

	var r0 []*hcloud.Network
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.NetworkListOpts) ([]*hcloud.Network, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.NetworkListOpts) []*hcloud.Network); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hcloud.Network)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.NetworkListOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListPlacementGroups provides a mock function with given fields: _a0, _a1
func (_m *Client) ListPlacementGroups(_a0 context.Context, _a1 hcloud.PlacementGroupListOpts) ([]*hcloud.PlacementGroup, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListPlacementGroups")
	}

	var r0 []*hcloud.PlacementGroup
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.PlacementGroupListOpts) ([]*hcloud.PlacementGroup, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.PlacementGroupListOpts) []*hcloud.PlacementGroup); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hcloud.PlacementGroup)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.PlacementGroupListOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListSSHKeys provides a mock function with given fields: _a0, _a1
func (_m *Client) ListSSHKeys(_a0 context.Context, _a1 hcloud.SSHKeyListOpts) ([]*hcloud.SSHKey, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListSSHKeys")
	}

	var r0 []*hcloud.SSHKey
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.SSHKeyListOpts) ([]*hcloud.SSHKey, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.SSHKeyListOpts) []*hcloud.SSHKey); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hcloud.SSHKey)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.SSHKeyListOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListServerTypes provides a mock function with given fields: _a0
func (_m *Client) ListServerTypes(_a0 context.Context) ([]*hcloud.ServerType, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for ListServerTypes")
	}

	var r0 []*hcloud.ServerType
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*hcloud.ServerType, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*hcloud.ServerType); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hcloud.ServerType)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListServers provides a mock function with given fields: _a0, _a1
func (_m *Client) ListServers(_a0 context.Context, _a1 hcloud.ServerListOpts) ([]*hcloud.Server, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListServers")
	}

	var r0 []*hcloud.Server
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.ServerListOpts) ([]*hcloud.Server, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, hcloud.ServerListOpts) []*hcloud.Server); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hcloud.Server)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, hcloud.ServerListOpts) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PowerOnServer provides a mock function with given fields: _a0, _a1
func (_m *Client) PowerOnServer(_a0 context.Context, _a1 *hcloud.Server) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for PowerOnServer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.Server) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RebootServer provides a mock function with given fields: _a0, _a1
func (_m *Client) RebootServer(_a0 context.Context, _a1 *hcloud.Server) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for RebootServer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.Server) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ShutdownServer provides a mock function with given fields: _a0, _a1
func (_m *Client) ShutdownServer(_a0 context.Context, _a1 *hcloud.Server) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ShutdownServer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.Server) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateLoadBalancer provides a mock function with given fields: _a0, _a1, _a2
func (_m *Client) UpdateLoadBalancer(_a0 context.Context, _a1 *hcloud.LoadBalancer, _a2 hcloud.LoadBalancerUpdateOpts) (*hcloud.LoadBalancer, error) {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for UpdateLoadBalancer")
	}

	var r0 *hcloud.LoadBalancer
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerUpdateOpts) (*hcloud.LoadBalancer, error)); ok {
		return rf(_a0, _a1, _a2)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerUpdateOpts) *hcloud.LoadBalancer); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hcloud.LoadBalancer)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *hcloud.LoadBalancer, hcloud.LoadBalancerUpdateOpts) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewClient creates a new instance of Client. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *Client {
	mock := &Client{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
