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

package loadbalancer

import (
	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var _ = Describe("Loadbalancer", func() {
	Context("hcloud cluster has network attached", func() {
		var sts *infrav1.LoadBalancerStatus
		BeforeEach(func() {
			sts = statusFromHCloudLB(lb, true, logr.Discard())
		})

		It("should have two targets", func() {
			Expect(sts.Target).To(Equal(targets))
		})
		It("should have the right ip addresses", func() {
			Expect(sts.IPv4).To(Equal(ipv4))
			Expect(sts.IPv6).To(Equal(ipv6))
		})
		It("should have the right internal IP", func() {
			Expect(sts.InternalIP).To(Equal(internalIP))
		})
		It("should be unprotected", func() {
			Expect(sts.Protected).To(Equal(protected))
		})
	})
	Context("hcloud cluster has no network attached", func() {
		var sts *infrav1.LoadBalancerStatus
		BeforeEach(func() {
			sts = statusFromHCloudLB(lb, false, logr.Discard())
		})

		It("should have two targets", func() {
			Expect(sts.Target).To(Equal(targets))
		})
		It("should have the right ip addresses", func() {
			Expect(sts.IPv4).To(Equal(ipv4))
			Expect(sts.IPv6).To(Equal(ipv6))
		})
		It("should have no internal IP", func() {
			Expect(sts.InternalIP).To(Equal(""))
		})
		It("should be unprotected", func() {
			Expect(sts.Protected).To(Equal(protected))
		})
	})
})

var _ = Describe("createOptsFromSpec", func() {
	var hetznerCluster *infrav1.HetznerCluster
	var wantCreateOpts hcloud.LoadBalancerCreateOpts
	BeforeEach(func() {
		lbName := "lb-name"
		lbType := "lb11"
		lbRegion := "fsn1"
		controlPlaneEndpointPort := 22
		lbPort := 6443
		networkID := 42

		hetznerCluster = &infrav1.HetznerCluster{
			Spec: infrav1.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
					Name:      &lbName,
					Algorithm: infrav1.LoadBalancerAlgorithmTypeLeastConnections,
					Type:      lbType,
					Region:    infrav1.Region(lbRegion),
					Port:      lbPort,
				},
				ControlPlaneEndpoint: &clusterv1.APIEndpoint{Port: int32(controlPlaneEndpointPort)},
			},
			Status: infrav1.HetznerClusterStatus{
				Network: &infrav1.NetworkStatus{ID: networkID},
			},
		}
		hetznerCluster.Name = "hetzner-cluster"

		publicInterface := true
		proxyprotocol := false

		wantCreateOpts = hcloud.LoadBalancerCreateOpts{
			LoadBalancerType: &hcloud.LoadBalancerType{Name: lbType},
			Name:             lbName,
			Algorithm:        &hcloud.LoadBalancerAlgorithm{Type: hcloud.LoadBalancerAlgorithmTypeLeastConnections},
			Location:         &hcloud.Location{Name: lbRegion},
			Network:          &hcloud.Network{ID: networkID},
			Labels:           map[string]string{hetznerCluster.ClusterTagKey(): string(infrav1.ResourceLifecycleOwned)},
			PublicInterface:  &publicInterface,
			Services: []hcloud.LoadBalancerCreateOptsService{
				{
					Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
					ListenPort:      &controlPlaneEndpointPort,
					DestinationPort: &lbPort,
					Proxyprotocol:   &proxyprotocol,
				},
			},
		}
	})

	It("creates specs for cluster without network", func() {
		hetznerCluster.Status.Network = nil
		wantCreateOpts.Network = nil

		Expect(createOptsFromSpec(hetznerCluster)).To(Equal(wantCreateOpts))
	})

	It("creates specs for cluster with network", func() {
		Expect(createOptsFromSpec(hetznerCluster)).To(Equal(wantCreateOpts))
	})

	It("creates specs for cluster without load balancer name set", func() {
		hetznerCluster.Spec.ControlPlaneLoadBalancer.Name = nil

		createOpts := createOptsFromSpec(hetznerCluster)

		// should generate correct name
		Expect(createOpts.Name).To(HavePrefix("hetzner-cluster-kube-apiserver-"))

		// should be the same for all other specs
		createOpts.Name = ""
		wantCreateOpts.Name = ""
		Expect(createOpts).To(Equal(wantCreateOpts))
	})
})
