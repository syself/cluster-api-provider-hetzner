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
	"fmt"
	"net"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

var _ = Describe("Test createOpts", func() {
	It("Outputs the correct NetworkCreateOpts", func() {
		expectOpts := hcloud.NetworkCreateOpts{
			Name:    "hetzner-cluster",
			IPRange: networkCidr,
			Labels:  map[string]string{"caph-cluster-hetzner-cluster": "owned"},
			Subnets: []hcloud.NetworkSubnet{
				{
					IPRange:     subnetCidr,
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
		hetznerCluster.Spec.HCloudNetwork.CIDRBlock = "invalid-cidr-block"
		_, err := service.createOpts()
		Expect(err).ToNot(BeNil())
	})

	It("gives an error with wrong SubnetCIDRBlock", func() {
		hetznerCluster.Spec.HCloudNetwork.SubnetCIDRBlock = "invalid-cidr-block"
		_, err := service.createOpts()
		Expect(err).ToNot(BeNil())
	})
})

var _ = Describe("Test findNetwork", func() {
	_, subnet2Cidr, _ := net.ParseCIDR("10.0.1.0/24")

	BeforeEach(func() {
		Expect(subnet2Cidr).ToNot(BeNil())
		hcloudClient.Reset()
	})

	It("outputs the correct network", func() {
		_, createErr := hcloudClient.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name:    "test-network",
			IPRange: networkCidr,
			Subnets: []hcloud.NetworkSubnet{
				{IPRange: subnetCidr, Type: hcloud.NetworkSubnetTypeCloud},
			},
			Labels: map[string]string{"caph-cluster-hetzner-cluster": "owned"},
		})
		Expect(createErr).To(BeNil())

		expectedNetwork := &hcloud.Network{
			ID:      1,
			Name:    "test-network",
			IPRange: networkCidr,
			Subnets: []hcloud.NetworkSubnet{
				{IPRange: subnetCidr, Type: hcloud.NetworkSubnetTypeCloud},
			},
			Labels: map[string]string{"caph-cluster-hetzner-cluster": "owned"},
		}

		network, err := service.findNetwork(context.Background())
		Expect(err).To(BeNil())
		Expect(network).To(Equal(expectedNetwork))
	})

	It("outputs no network/error if there is no network available", func() {
		network, err := service.findNetwork(context.Background())
		Expect(err).To(BeNil())
		Expect(network).To(BeNil())
	})

	It("outputs the correct network if there are multiple subnets but the first one matches the configured one on the HetznerCluster", func() {
		_, createErr := hcloudClient.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name:    "test-network",
			IPRange: networkCidr,
			Subnets: []hcloud.NetworkSubnet{
				{IPRange: subnetCidr, Type: hcloud.NetworkSubnetTypeCloud},
				{IPRange: subnet2Cidr, Type: hcloud.NetworkSubnetTypeCloud},
			},
			Labels: map[string]string{"caph-cluster-hetzner-cluster": "owned"},
		})
		Expect(createErr).To(BeNil())

		expectedNetwork := &hcloud.Network{
			ID:      1,
			Name:    "test-network",
			IPRange: networkCidr,
			Subnets: []hcloud.NetworkSubnet{
				{IPRange: subnetCidr, Type: hcloud.NetworkSubnetTypeCloud},
				{IPRange: subnet2Cidr, Type: hcloud.NetworkSubnetTypeCloud},
			},
			Labels: map[string]string{"caph-cluster-hetzner-cluster": "owned"},
		}

		network, err := service.findNetwork(context.Background())
		Expect(err).To(BeNil())
		Expect(network).To(Equal(expectedNetwork))
	})

	It("gives an error if there there are multiple subnet and the first one doesn't match the configure one on the HetznerCluster", func() {
		_, createErr := hcloudClient.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name:    "test-network",
			IPRange: networkCidr,
			Subnets: []hcloud.NetworkSubnet{
				{IPRange: subnet2Cidr, Type: hcloud.NetworkSubnetTypeCloud},
				{IPRange: subnetCidr, Type: hcloud.NetworkSubnetTypeCloud},
			},
			Labels: map[string]string{"caph-cluster-hetzner-cluster": "owned"},
		})
		Expect(createErr).To(BeNil())

		network, err := service.findNetwork(context.Background())
		Expect(network).To(BeNil())
		Expect(err).To(Equal(fmt.Errorf("multiple subnets found and first subnet 10.0.1.0/24 doesn't match the configured 10.0.0.0/24")))
	})

	It("gives an error if there are multiple networks with the same label", func() {
		_, createErr1 := hcloudClient.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name:    "test-network",
			IPRange: networkCidr,
			Subnets: []hcloud.NetworkSubnet{
				{IPRange: subnetCidr},
			},
			Labels: map[string]string{"caph-cluster-hetzner-cluster": "owned"},
		})
		Expect(createErr1).To(BeNil())

		_, createErr2 := hcloudClient.CreateNetwork(context.Background(), hcloud.NetworkCreateOpts{
			Name:    "test-network2",
			IPRange: networkCidr,
			Subnets: []hcloud.NetworkSubnet{
				{IPRange: subnetCidr, Type: hcloud.NetworkSubnetTypeCloud},
			},
			Labels: map[string]string{"caph-cluster-hetzner-cluster": "owned"},
		})
		Expect(createErr2).To(BeNil())

		_, err := service.findNetwork(context.Background())
		expectedOpts := hcloud.NetworkListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: utils.LabelsToLabelSelector(map[string]string{"caph-cluster-hetzner-cluster": "owned"}),
			},
		}
		Expect(err).To(Equal(fmt.Errorf("found multiple networks with opts %v - not allowed", expectedOpts)))
	})

})
