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
	"net"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
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
			CIDRBlock:       "10.0.0.0/16",
			SubnetCIDRBlock: "10.0.0.0/24",
			NetworkZone:     "eu-central",
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
					Type:        hcloud.NetworkSubnetTypeServer,
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
