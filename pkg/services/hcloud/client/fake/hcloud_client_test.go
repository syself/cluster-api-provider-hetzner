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

package fake_test

import (
	"context"
	"net"

	"github.com/hetznercloud/hcloud-go/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

const labelSelector = "key1==val1"

var factory = fake.NewHCloudClientFactory()
var ctx = context.Background()

var _ = Describe("Load balancer", func() {
	var opts = hcloud.LoadBalancerCreateOpts{
		Name: "lb-name",
		Labels: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		Algorithm: &hcloud.LoadBalancerAlgorithm{
			Type: hcloud.LoadBalancerAlgorithmTypeRoundRobin,
		},
		LoadBalancerType: &hcloud.LoadBalancerType{
			Name: "lb11",
		},
		Location: &hcloud.Location{
			Name: "fsn1",
		},
		Network: &hcloud.Network{
			ID:      1,
			IPRange: &net.IPNet{IP: net.IP("1.2.3.4")},
		},
	}

	var listOpts hcloud.LoadBalancerListOpts
	listOpts.LabelSelector = labelSelector

	var networkOpts = hcloud.NetworkCreateOpts{
		Name: "network-name",
		Labels: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		IPRange: &net.IPNet{IP: net.IP("1.2.3.5")},
		Subnets: []hcloud.NetworkSubnet{{IPRange: &net.IPNet{IP: net.IP("1.2.3.6")}}},
	}

	var client = factory.NewClient("")

	var lb *hcloud.LoadBalancer
	var network *hcloud.Network

	BeforeEach(func() {
		_, err := client.CreateLoadBalancer(ctx, opts)
		Expect(err).To(Succeed())

		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		lb = resp[0]

		network, err = client.CreateNetwork(ctx, networkOpts)
		Expect(err).To(Succeed())
	})
	AfterEach(func() {
		client.Close()
	})

	It("creates a load balancer and returns it with an ID", func() {
		Expect(lb.ID).ToNot(Equal(0))
	})

	It("gives an error when load balancer is created twice", func() {
		_, err := client.CreateLoadBalancer(ctx, opts)
		Expect(err).ToNot(Succeed())
	})

	It("creates a load balancer and deletes it", func() {
		Expect(client.DeleteLoadBalancer(ctx, lb.ID)).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(0))
	})

	It("gives a NotFound error when a non-existing load balancer is deleted", func() {
		err := client.DeleteLoadBalancer(ctx, 999)
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("changes the load balancer type successfully", func() {
		Expect(lb.LoadBalancerType.Name).To(Equal("lb11"))
		_, err := client.ChangeLoadBalancerType(ctx, lb, hcloud.LoadBalancerChangeTypeOpts{
			LoadBalancerType: &hcloud.LoadBalancerType{
				Name: "lb21",
			},
		})
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(resp[0].LoadBalancerType.Name).To(Equal("lb21"))
	})

	It("gives a NotFound error when a non-existing load balancer's type is updated", func() {
		_, err := client.ChangeLoadBalancerType(ctx, &hcloud.LoadBalancer{ID: 999}, hcloud.LoadBalancerChangeTypeOpts{
			LoadBalancerType: &hcloud.LoadBalancerType{
				Name: "lb21",
			},
		})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("changes the load balancer algorithm successfully", func() {
		Expect(lb.Algorithm.Type).To(Equal(hcloud.LoadBalancerAlgorithmTypeRoundRobin))
		_, err := client.ChangeLoadBalancerAlgorithm(ctx, lb, hcloud.LoadBalancerChangeAlgorithmOpts{
			Type: hcloud.LoadBalancerAlgorithmTypeLeastConnections,
		})
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(resp[0].Algorithm.Type).To(Equal(hcloud.LoadBalancerAlgorithmTypeLeastConnections))
	})

	It("gives a NotFound error when a non-existing load balancer's algorithm is updated", func() {
		_, err := client.ChangeLoadBalancerAlgorithm(ctx, &hcloud.LoadBalancer{ID: 999}, hcloud.LoadBalancerChangeAlgorithmOpts{
			Type: hcloud.LoadBalancerAlgorithmTypeLeastConnections,
		})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("changes the load balancer name successfully", func() {
		Expect(lb.Name).To(Equal("lb-name"))
		lb, err := client.UpdateLoadBalancer(ctx, lb, hcloud.LoadBalancerUpdateOpts{
			Name: "new-lb-name",
		})
		Expect(err).To(Succeed())
		Expect(lb.Name).To(Equal("new-lb-name"))
	})

	It("gives a NotFound error when a non-existing load balancer's name is updated", func() {
		_, err := client.UpdateLoadBalancer(ctx, &hcloud.LoadBalancer{ID: 999}, hcloud.LoadBalancerUpdateOpts{})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("adds a server target to load balancer successfully", func() {
		Expect(len(lb.Targets)).To(Equal(0))
		_, err := client.AddTargetServerToLoadBalancer(ctx, hcloud.LoadBalancerAddServerTargetOpts{
			Server: &hcloud.Server{
				ID: 42,
			},
		}, lb)
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].Targets)).To(Equal(1))
		Expect(resp[0].Targets[0].Server.Server.ID).To(Equal(42))
	})

	It("adds an IP target to load balancer successfully", func() {
		Expect(len(lb.Targets)).To(Equal(0))
		_, err := client.AddIPTargetToLoadBalancer(ctx, hcloud.LoadBalancerAddIPTargetOpts{
			IP: net.ParseIP("1.2.3.4"),
		}, lb)
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].Targets)).To(Equal(1))
		Expect(resp[0].Targets[0].IP.IP).To(Equal("1.2.3.4"))
	})

	It("gives a AlreadyAdded error when a server target is added twice", func() {
		_, err := client.AddTargetServerToLoadBalancer(ctx, hcloud.LoadBalancerAddServerTargetOpts{
			Server: &hcloud.Server{
				ID: 42,
			},
		}, lb)
		Expect(err).To(BeNil())
		_, err = client.AddTargetServerToLoadBalancer(ctx, hcloud.LoadBalancerAddServerTargetOpts{
			Server: &hcloud.Server{
				ID: 42,
			},
		}, lb)
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAdded)).To(BeTrue())
	})

	It("gives a AlreadyAdded error when an IP target is added twice", func() {
		_, err := client.AddIPTargetToLoadBalancer(ctx, hcloud.LoadBalancerAddIPTargetOpts{
			IP: net.ParseIP("1.2.3.4"),
		}, lb)
		Expect(err).To(BeNil())
		_, err = client.AddIPTargetToLoadBalancer(ctx, hcloud.LoadBalancerAddIPTargetOpts{
			IP: net.ParseIP("1.2.3.4"),
		}, lb)
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAdded)).To(BeTrue())
	})

	It("gives a NotFound error when a target is added to a non-existing load balancer", func() {
		_, err := client.AddTargetServerToLoadBalancer(ctx, hcloud.LoadBalancerAddServerTargetOpts{
			Server: &hcloud.Server{
				ID: 42,
			},
		}, &hcloud.LoadBalancer{ID: 999})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("adds and deletes a server target of load balancer successfully", func() {
		server := &hcloud.Server{
			ID: 42,
		}
		_, err := client.AddTargetServerToLoadBalancer(ctx, hcloud.LoadBalancerAddServerTargetOpts{
			Server: server,
		}, lb)
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].Targets)).To(Equal(1))
		_, err = client.DeleteTargetServerOfLoadBalancer(ctx, lb, server)
		Expect(err).To(Succeed())
		resp2, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp2)).To(Equal(1))
		Expect(len(resp2[0].Targets)).To(Equal(0))
	})

	It("adds and deletes an IP target of load balancer successfully", func() {
		_, err := client.AddIPTargetToLoadBalancer(ctx, hcloud.LoadBalancerAddIPTargetOpts{
			IP: net.ParseIP("1.2.3.4"),
		}, lb)
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].Targets)).To(Equal(1))
		_, err = client.DeleteIPTargetOfLoadBalancer(ctx, lb, net.ParseIP("1.2.3.4"))
		Expect(err).To(Succeed())
		resp2, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp2)).To(Equal(1))
		Expect(len(resp2[0].Targets)).To(Equal(0))
	})

	It("gives a NotFound error if a non-existing target is deleted from a load balancer", func() {
		_, err := client.DeleteTargetServerOfLoadBalancer(ctx, lb, &hcloud.Server{ID: 42})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("gives a NotFound error when a target is deleted to a non-existing load balancer", func() {
		_, err := client.DeleteTargetServerOfLoadBalancer(ctx, &hcloud.LoadBalancer{ID: 999}, &hcloud.Server{ID: 42})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("adds the load balancer successfully to a network", func() {
		Expect(len(lb.PrivateNet)).To(Equal(1))
		_, err := client.AttachLoadBalancerToNetwork(ctx, lb, hcloud.LoadBalancerAttachToNetworkOpts{Network: network})
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].PrivateNet)).To(Equal(2))
		Expect(resp[0].PrivateNet[1].IP.Equal(network.IPRange.IP)).To(BeTrue())
	})

	It("gives a NotFound error when a non-existing load balancer is added to a network", func() {
		_, err := client.AttachLoadBalancerToNetwork(ctx, &hcloud.LoadBalancer{ID: 999}, hcloud.LoadBalancerAttachToNetworkOpts{
			Network: network,
		})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("gives a NotFound error when a load balancer is added to a non-existing network", func() {
		_, err := client.AttachLoadBalancerToNetwork(ctx, lb, hcloud.LoadBalancerAttachToNetworkOpts{
			Network: &hcloud.Network{ID: 2},
		})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("gives an error when a load balancer is added twice to a network", func() {
		_, err := client.AttachLoadBalancerToNetwork(ctx, lb, hcloud.LoadBalancerAttachToNetworkOpts{
			Network: network,
		})
		Expect(err).To(Succeed())

		_, err = client.AttachLoadBalancerToNetwork(ctx, lb, hcloud.LoadBalancerAttachToNetworkOpts{
			Network: network,
		})
		Expect(err).ToNot(Succeed())
	})

	It("adds service to load balancer", func() {
		Expect(len(lb.Services)).To(Equal(0))
		listenPort := 443
		destinationPort := 443
		_, err := client.AddServiceToLoadBalancer(ctx, lb, hcloud.LoadBalancerAddServiceOpts{
			ListenPort:      &listenPort,
			DestinationPort: &destinationPort,
		})
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].Services)).To(Equal(1))
		Expect(resp[0].Services[0].DestinationPort).To(Equal(destinationPort))
		Expect(resp[0].Services[0].ListenPort).To(Equal(listenPort))
	})

	It("gives a NotFound error when a service is added to a non-existing load balancer", func() {
		listenPort := 443
		destinationPort := 443
		_, err := client.AddServiceToLoadBalancer(ctx, &hcloud.LoadBalancer{ID: 999}, hcloud.LoadBalancerAddServiceOpts{
			ListenPort:      &listenPort,
			DestinationPort: &destinationPort,
		})
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("gives an error when a service is added twice to a load balancer", func() {
		listenPort := 443
		destinationPort := 443
		_, err := client.AddServiceToLoadBalancer(ctx, lb, hcloud.LoadBalancerAddServiceOpts{
			ListenPort:      &listenPort,
			DestinationPort: &destinationPort,
		})
		Expect(err).To(BeNil())
		_, err = client.AddServiceToLoadBalancer(ctx, lb, hcloud.LoadBalancerAddServiceOpts{
			ListenPort:      &listenPort,
			DestinationPort: &destinationPort,
		})
		Expect(err).ToNot(Succeed())
	})

	It("gives an error when a service with listenPort 0 is added to a load balancer", func() {
		listenPort := 0
		destinationPort := 443
		_, err := client.AddServiceToLoadBalancer(ctx, lb, hcloud.LoadBalancerAddServiceOpts{
			ListenPort:      &listenPort,
			DestinationPort: &destinationPort,
		})
		Expect(err).ToNot(Succeed())
	})

	It("adds and deletes service of and from load balancer", func() {
		Expect(len(lb.Services)).To(Equal(0))
		listenPort := 443
		destinationPort := 443
		_, err := client.AddServiceToLoadBalancer(ctx, lb, hcloud.LoadBalancerAddServiceOpts{
			ListenPort:      &listenPort,
			DestinationPort: &destinationPort,
		})
		Expect(err).To(Succeed())
		resp, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].Services)).To(Equal(1))
		_, err = client.DeleteServiceFromLoadBalancer(ctx, lb, listenPort)
		Expect(err).To(Succeed())
		resp2, err := client.ListLoadBalancers(ctx, listOpts)
		Expect(err).To(BeNil())
		Expect(len(resp2)).To(Equal(1))
		Expect(len(resp2[0].Services)).To(Equal(0))
	})

	It("gives a NotFound error when a service is deleted of a non-existing load balancer", func() {
		_, err := client.DeleteServiceFromLoadBalancer(ctx, &hcloud.LoadBalancer{ID: 999}, 443)
		Expect(err).ToNot(BeNil())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("gives an error when a non-existing service is deleted from a load balancer", func() {
		_, err := client.DeleteServiceFromLoadBalancer(ctx, lb, 999)
		Expect(err).ToNot(Succeed())
	})

	var _ = Describe("Images", func() {
		var listOpts hcloud.ImageListOpts
		var client = factory.NewClient("")
		BeforeEach(func() {
			listOpts.LabelSelector = labelSelector
		})
		It("lists at least one image", func() {
			resp, err := client.ListImages(ctx, listOpts)
			Expect(err).To(Succeed())
			Expect(len(resp)).To(BeNumerically(">", 0))
		})
	})
})

var _ = Describe("Server", func() {
	var listOpts hcloud.ServerListOpts
	listOpts.LabelSelector = labelSelector

	var client = factory.NewClient("")

	var opts = hcloud.ServerCreateOpts{
		Name: "test-server",
		Labels: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		Image: &hcloud.Image{
			ID: 14,
		},
		ServerType: &hcloud.ServerType{
			Name: "cpx11",
		},
		PlacementGroup: &hcloud.PlacementGroup{
			ID: 24,
		},
	}

	var server *hcloud.Server
	var network *hcloud.Network

	BeforeEach(func() {
		resp, err := client.CreateServer(ctx, opts)
		Expect(err).To(Succeed())
		server = resp.Server

		network, err = client.CreateNetwork(ctx, hcloud.NetworkCreateOpts{
			Name: "network-name",
			Labels: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			IPRange: &net.IPNet{IP: net.IP("1.2.3.5")},
			Subnets: []hcloud.NetworkSubnet{{IPRange: &net.IPNet{IP: net.IP("1.2.3.6")}}},
		})
		Expect(err).To(Succeed())
	})
	AfterEach(func() {
		client.Close()
	})

	It("creates a server with an ID", func() {
		Expect(server.ID).ToNot(Equal(0))
	})

	It("gives an error when a server is created twice", func() {
		_, err := client.CreateServer(ctx, opts)
		Expect(err).ToNot(Succeed())
	})

	It("adds a server to a network", func() {
		_, err := client.AttachServerToNetwork(ctx, server, hcloud.ServerAttachToNetworkOpts{
			Network: network,
		})
		Expect(err).To(Succeed())
	})

	It("gives an error when a server is added to a non-existing network", func() {
		_, err := client.AttachServerToNetwork(ctx, server, hcloud.ServerAttachToNetworkOpts{
			Network: &hcloud.Network{ID: 2},
		})
		Expect(err).ToNot(Succeed())
	})

	It("gives an error when a non-existing server is added to a network", func() {
		_, err := client.AttachServerToNetwork(ctx, &hcloud.Server{ID: 2}, hcloud.ServerAttachToNetworkOpts{
			Network: network,
		})
		Expect(err).ToNot(Succeed())
	})

	It("lists servers", func() {
		resp, err := client.ListServers(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(1))
		Expect(resp[0].ID).To(Equal(server.ID))
	})

	It("shuts a server down", func() {
		Expect(server.Status).To(Equal(hcloud.ServerStatusRunning))
		_, err := client.ShutdownServer(ctx, server)
		Expect(err).To(Succeed())
		resp, err := client.ListServers(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(1))
		Expect(resp[0].Status).To(Equal(hcloud.ServerStatusOff))
	})

	It("gives an error when a non-existing server is shut down", func() {
		_, err := client.ShutdownServer(ctx, &hcloud.Server{ID: 2})
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("powers a server on", func() {
		_, err := client.ShutdownServer(ctx, server)
		Expect(err).To(Succeed())
		resp, err := client.ListServers(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(1))
		Expect(resp[0].Status).To(Equal(hcloud.ServerStatusOff))
		_, err = client.PowerOnServer(ctx, server)
		Expect(err).To(Succeed())
		resp2, err := client.ListServers(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp2)).To(Equal(1))
		Expect(resp2[0].Status).To(Equal(hcloud.ServerStatusRunning))
	})

	It("gives an error when a non-existing server is powered on", func() {
		_, err := client.PowerOnServer(ctx, &hcloud.Server{ID: 2})
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("deletes a server", func() {
		Expect(client.DeleteServer(ctx, server)).To(Succeed())
		resp, err := client.ListServers(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(0))
	})

	It("gives an error when a non-existing server is deleted", func() {
		err := client.DeleteServer(ctx, &hcloud.Server{ID: 2})
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("returns the correct server types", func() {
		serverTypes, err := client.ListServerTypes(ctx)
		Expect(err).To(Succeed())
		Expect(serverTypes).To(Equal([]*hcloud.ServerType{
			{
				ID:     1,
				Name:   "cpx11",
				Cores:  fake.DefaultCPUCores,
				Memory: fake.DefaultMemoryInGB,
			},
			{
				ID:     2,
				Name:   "cpx21",
				Cores:  fake.DefaultCPUCores,
				Memory: fake.DefaultMemoryInGB,
			},
			{
				ID:     3,
				Name:   "cpx31",
				Cores:  fake.DefaultCPUCores,
				Memory: fake.DefaultMemoryInGB,
			},
		}))
	})
})

var _ = Describe("Network", func() {
	var listOpts hcloud.NetworkListOpts
	listOpts.LabelSelector = labelSelector

	var client = factory.NewClient("")

	var opts = hcloud.NetworkCreateOpts{
		Name: "test-network",
		Labels: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		IPRange: &net.IPNet{IP: net.IP("1.2.3.5")},
		Subnets: []hcloud.NetworkSubnet{{IPRange: &net.IPNet{IP: net.IP("1.2.3.6")}}},
	}

	var network *hcloud.Network

	BeforeEach(func() {
		var err error
		network, err = client.CreateNetwork(ctx, opts)
		Expect(err).To(Succeed())
	})
	AfterEach(func() {
		client.Close()
	})

	It("creates a network with an ID", func() {
		Expect(network.ID).ToNot(Equal(0))
	})

	It("gives an error when a network is created twice", func() {
		_, err := client.CreateNetwork(ctx, opts)
		Expect(err).ToNot(Succeed())
	})

	It("lists networks", func() {
		resp, err := client.ListNetworks(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(1))
		Expect(resp[0].ID).To(Equal(network.ID))
	})

	It("deletes a network", func() {
		Expect(client.DeleteNetwork(ctx, network)).To(Succeed())
		resp, err := client.ListNetworks(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(0))
	})

	It("gives an error when a non-existing network is deleted", func() {
		err := client.DeleteNetwork(ctx, &hcloud.Network{ID: 2})
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})
})

var _ = Describe("Placement groups", func() {
	var listOpts hcloud.PlacementGroupListOpts
	listOpts.LabelSelector = labelSelector

	var opts = hcloud.PlacementGroupCreateOpts{
		Name: "placement-group-name",
		Labels: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		Type: "stream",
	}

	var client = factory.NewClient("")

	var server *hcloud.Server
	var placementGroup *hcloud.PlacementGroup

	BeforeEach(func() {
		resp, err := client.CreateServer(ctx, hcloud.ServerCreateOpts{
			Name: "test-server",
			Labels: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			Image: &hcloud.Image{
				ID: 14,
			},
			ServerType: &hcloud.ServerType{
				Name: "cpx11",
			},
			PlacementGroup: &hcloud.PlacementGroup{
				ID: 24,
			},
		})
		Expect(err).To(Succeed())
		server = resp.Server

		res, err := client.CreatePlacementGroup(ctx, opts)
		Expect(err).To(Succeed())
		placementGroup = res.PlacementGroup
	})
	AfterEach(func() {
		client.Close()
	})

	It("creates a placementGroup with an ID", func() {
		Expect(placementGroup.ID).ToNot(Equal(0))
	})

	It("gives an error when a placementGroup is created twice", func() {
		_, err := client.CreatePlacementGroup(ctx, opts)
		Expect(err).ToNot(Succeed())
	})

	It("lists placement groups", func() {
		resp, err := client.ListPlacementGroups(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(1))
		Expect(resp[0].ID).To(Equal(placementGroup.ID))
	})

	It("adds a server to a placement group", func() {
		Expect(len(placementGroup.Servers)).To(Equal(0))
		_, err := client.AddServerToPlacementGroup(ctx, server, placementGroup)
		Expect(err).To(Succeed())
		resp, err := client.ListPlacementGroups(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(1))
		Expect(len(resp[0].Servers)).To(Equal(1))
		Expect(resp[0].Servers[0]).To(Equal(server.ID))
	})

	It("gives an error when a server is added to a non-existing placement group", func() {
		_, err := client.AddServerToPlacementGroup(ctx, server, &hcloud.PlacementGroup{ID: 2})
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("gives an error when a non-existing server is added to a placement group", func() {
		_, err := client.AddServerToPlacementGroup(ctx, &hcloud.Server{ID: 2}, placementGroup)
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})

	It("gives an error when a server is added twice to a placement group", func() {
		_, err := client.AddServerToPlacementGroup(ctx, server, placementGroup)
		Expect(err).To(Succeed())
		_, err = client.AddServerToPlacementGroup(ctx, server, placementGroup)
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAdded)).To(BeTrue())
	})

	It("deletes a placementGroup", func() {
		Expect(client.DeletePlacementGroup(ctx, placementGroup.ID)).To(Succeed())
		resp, err := client.ListPlacementGroups(ctx, listOpts)
		Expect(err).To(Succeed())
		Expect(len(resp)).To(Equal(0))
	})

	It("gives an error when a non-existing placementGroup is deleted", func() {
		err := client.DeletePlacementGroup(ctx, 999)
		Expect(err).ToNot(Succeed())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeNotFound)).To(BeTrue())
	})
})
