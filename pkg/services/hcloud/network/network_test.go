/*
Copyright 2023 The Kubernetes Authors.

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

package network

import (
	"context"
	"net"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	fakeclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

func TestNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Suite")
}

var _ = Describe("Test createOpts", func() {
	var hetznerCluster infrav1.HetznerCluster
	var service Service
	BeforeEach(func() {
		hetznerCluster.Spec.HCloudNetwork = infrav1.HCloudNetworkSpec{
			Enabled:         true,
			CIDRBlock:       ptr.To(infrav1.DefaultCIDRBlock),
			SubnetCIDRBlock: ptr.To(infrav1.DefaultSubnetCIDRBlock),
			NetworkZone:     ptr.To[infrav1.HCloudNetworkZone](infrav1.DefaultNetworkZone),
		}
		hetznerCluster.Name = "hetzner-cluster"

		service = Service{&scope.ClusterScope{HetznerCluster: &hetznerCluster}}
	})
	It("Outputs the correct NetworkCreateOpts", func() {
		_, network, err := net.ParseCIDR("10.0.0.0/16")
		Expect(err).To(BeNil())

		_, subnet, err := net.ParseCIDR("10.0.0.0/24")
		Expect(err).To(BeNil())

		expectOpts := hcloud.NetworkCreateOpts{
			Name:    "hetzner-cluster",
			IPRange: network,
			Labels:  map[string]string{"caph-cluster-hetzner-cluster": "owned"},
			Subnets: []hcloud.NetworkSubnet{
				{
					IPRange:     subnet,
					NetworkZone: hcloud.NetworkZoneEUCentral,
					Type:        hcloud.NetworkSubnetTypeCloud,
				},
			},
		}

		opts, err := service.createOpts()
		Expect(err).To(BeNil())
		Expect(opts).To(Equal(expectOpts))
	})

	It("gives an error with wrong CIDRBlock", func() {
		hetznerCluster.Spec.HCloudNetwork.CIDRBlock = ptr.To("invalid-cidr-block")
		_, err := service.createOpts()
		Expect(err).ToNot(BeNil())
	})

	It("gives an error with wrong SubnetCIDRBlock", func() {
		hetznerCluster.Spec.HCloudNetwork.SubnetCIDRBlock = ptr.To("invalid-cidr-block")
		_, err := service.createOpts()
		Expect(err).ToNot(BeNil())
	})

	It("gives an error with nil CIDRBlock", func() {
		hetznerCluster.Spec.HCloudNetwork.CIDRBlock = nil
		_, err := service.createOpts()
		Expect(err).ToNot(BeNil())
	})

	It("gives an error with nil SubnetCIDRBlock", func() {
		hetznerCluster.Spec.HCloudNetwork.SubnetCIDRBlock = nil
		_, err := service.createOpts()
		Expect(err).ToNot(BeNil())
	})

	It("gives an error with nil NetworkZone", func() {
		hetznerCluster.Spec.HCloudNetwork.NetworkZone = nil
		_, err := service.createOpts()
		Expect(err).ToNot(BeNil())
	})
})

var _ = Describe("Test findNetwork", func() {
	var hetznerCluster infrav1.HetznerCluster
	var service Service
	var network *hcloud.Network
	client := fakeclient.NewHCloudClientFactory().NewClient("")

	BeforeEach(func() {
		hetznerCluster.Spec.HCloudNetwork = infrav1.HCloudNetworkSpec{
			Enabled:         true,
			CIDRBlock:       ptr.To(infrav1.DefaultCIDRBlock),
			SubnetCIDRBlock: ptr.To(infrav1.DefaultSubnetCIDRBlock),
			NetworkZone:     ptr.To[infrav1.HCloudNetworkZone](infrav1.DefaultNetworkZone),
		}
		hetznerCluster.Name = "hetzner-cluster"

		service = Service{&scope.ClusterScope{HetznerCluster: &hetznerCluster, HCloudClient: client}}
	})
	AfterEach(func() {
		err := client.DeleteNetwork(context.Background(), network)
		Expect(err).To(Succeed())
	})
	It("Gets the Network if ID is set", func() {
		hetznerCluster.Spec.HCloudNetwork.ID = ptr.To(int64(1))

		var err error
		network, err = client.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{Name: "networkName"})
		Expect(err).To(Succeed())
		res, err := service.findNetwork(context.Background())
		Expect(err).To(BeNil())
		Expect(res).To(Equal(network))
	})
	It("Finds the labeled Network if ID is not set", func() {
		var err error
		network, err = client.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name: "networkName",
			Labels: map[string]string{
				hetznerCluster.ClusterTagKey(): string(infrav1.ResourceLifecycleOwned),
			},
		})
		Expect(err).To(Succeed())
		res, err := service.findNetwork(context.Background())
		Expect(err).To(BeNil())
		Expect(res).To(Equal(network))
	})
	It("gives an error when there is more than one Network", func() {
		var err error
		network, err = client.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name: "networkName",
			Labels: map[string]string{
				hetznerCluster.ClusterTagKey(): string(infrav1.ResourceLifecycleOwned),
			},
		})
		Expect(err).To(Succeed())
		network2, err := client.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name: "networkName2",
			Labels: map[string]string{
				hetznerCluster.ClusterTagKey(): string(infrav1.ResourceLifecycleOwned),
			},
		})
		Expect(err).To(Succeed())
		res, err := service.findNetwork(context.Background())
		Expect(res).To(BeNil())
		Expect(err).ToNot(BeNil())

		err = client.DeleteNetwork(context.Background(), network2)
		Expect(err).To(Succeed())
	})
	It("gives an error when there is more than one Subnet", func() {
		var err error
		network, err = client.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name: "networkName",
			Labels: map[string]string{
				hetznerCluster.ClusterTagKey(): string(infrav1.ResourceLifecycleOwned),
			},
			Subnets: make([]hcloud.NetworkSubnet, 2),
		})
		Expect(err).To(Succeed())
		res, err := service.findNetwork(context.Background())
		Expect(res).To(BeNil())
		Expect(err).ToNot(BeNil())
	})
})
