/*
Copyright 2025 The Kubernetes Authors.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	fakeclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

var (
	_, networkCidr, _ = net.ParseCIDR("10.0.0.0/16")
	_, subnetCidr, _  = net.ParseCIDR("10.0.0.0/24")
	hetznerCluster    infrav1.HetznerCluster
	service           Service
	hcloudClient      hcloudclient.Client
)

func TestNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Suite")
}

var _ = BeforeSuite(func() {
	hetznerCluster.Name = "hetzner-cluster"
	hetznerCluster.Spec.HCloudNetwork = infrav1.HCloudNetworkSpec{
		Enabled:         true,
		CIDRBlock:       networkCidr.String(),
		SubnetCIDRBlock: subnetCidr.String(),
		NetworkZone:     "eu-central",
	}

	hcloudClient = fakeclient.NewHCloudClientFactory().NewClient("")
	service = Service{&scope.ClusterScope{HetznerCluster: &hetznerCluster, HCloudClient: hcloudClient}}
})
