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
	"time"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

var _ = Describe("Loadbalancer", func() {
	Context("hcloud cluster has network attached", func() {
		var sts *infrav2.LoadBalancerStatus
		BeforeEach(func() {
			sts = statusFromHCloudLB(lb, true, 443, logr.Discard())
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
		It("should have proxy protocol disabled", func() {
			Expect(sts.ProxyProtocolEnabled).To(BeFalse())
		})
	})
	Context("hcloud cluster has no network attached", func() {
		var sts *infrav2.LoadBalancerStatus
		BeforeEach(func() {
			sts = statusFromHCloudLB(lb, false, 443, logr.Discard())
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
	Context("proxy protocol detection", func() {
		It("reports enabled when the kube-API service has proxy protocol on", func() {
			lbWithProxyProtocol := &hcloud.LoadBalancer{
				Services: []hcloud.LoadBalancerService{
					{ListenPort: 6443, Proxyprotocol: true},
				},
			}
			sts := statusFromHCloudLB(lbWithProxyProtocol, false, 6443, logr.Discard())
			Expect(sts.ProxyProtocolEnabled).To(BeTrue())
		})
		It("reports disabled when the kube-API port has no matching service", func() {
			sts := statusFromHCloudLB(lb, false, 6443, logr.Discard())
			Expect(sts.ProxyProtocolEnabled).To(BeFalse())
		})
	})
})

var _ = Describe("createOptsFromSpec", func() {
	var hetznerCluster *infrav2.HetznerCluster
	var wantCreateOpts hcloud.LoadBalancerCreateOpts
	BeforeEach(func() {
		lbType := "lb11"
		lbRegion := "fsn1"
		controlPlaneEndpointPort := 22
		lbPort := 6443
		var networkID int64 = 42

		hetznerCluster = &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Name:      nil,
					Algorithm: infrav2.LoadBalancerAlgorithmTypeLeastConnections,
					Type:      lbType,
					Region:    infrav2.Region(lbRegion),
					Port:      lbPort,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{Port: int32(controlPlaneEndpointPort)},
			},
			Status: infrav2.HetznerClusterStatus{
				Network: &infrav2.NetworkStatus{ID: networkID},
			},
		}
		hetznerCluster.Name = "hetzner-cluster"

		publicInterface := true
		proxyprotocol := false

		wantCreateOpts = hcloud.LoadBalancerCreateOpts{
			LoadBalancerType: &hcloud.LoadBalancerType{Name: lbType},
			Name:             "",
			Algorithm:        &hcloud.LoadBalancerAlgorithm{Type: hcloud.LoadBalancerAlgorithmTypeLeastConnections},
			Location:         &hcloud.Location{Name: lbRegion},
			Network:          &hcloud.Network{ID: networkID},
			Labels:           map[string]string{hetznerCluster.ClusterTagKey(): string(infrav2.ResourceLifecycleOwned)},
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

		createOpts := createOptsFromSpec(hetznerCluster)

		// ignore random name
		createOpts.Name = ""

		Expect(createOpts).To(Equal(wantCreateOpts))
	})

	It("creates specs for cluster with network", func() {
		createOpts := createOptsFromSpec(hetznerCluster)

		// ignore random name
		createOpts.Name = ""

		Expect(createOpts).To(Equal(wantCreateOpts))
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

	It("uses a zero listen port until the control plane endpoint is filled in", func() {
		hetznerCluster.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{}

		createOpts := createOptsFromSpec(hetznerCluster)

		Expect(*createOpts.Services[0].ListenPort).To(Equal(0))
	})

	It("omits the health check when the spec doesn't set one", func() {
		createOpts := createOptsFromSpec(hetznerCluster)

		Expect(createOpts.Services[0].HealthCheck).To(BeNil())
	})

	It("carries an http health check into the kube-API service", func() {
		hetznerCluster.Spec.ControlPlaneLoadBalancer.HealthCheck = &infrav2.LoadBalancerHealthCheckSpec{
			Type:     "http",
			Interval: &metav1.Duration{Duration: 5 * time.Second},
			Timeout:  &metav1.Duration{Duration: 2 * time.Second},
			Retries:  ptr.To(5),
			Domain:   ptr.To("example.com"),
			Path:     ptr.To("/readyz"),
		}

		createOpts := createOptsFromSpec(hetznerCluster)

		interval := 5 * time.Second
		timeout := 2 * time.Second
		retries := 5
		domain := "example.com"
		path := "/readyz"
		tls := false
		Expect(createOpts.Services[0].HealthCheck).To(Equal(&hcloud.LoadBalancerCreateOptsServiceHealthCheck{
			Protocol: hcloud.LoadBalancerServiceProtocolHTTP,
			Interval: &interval,
			Timeout:  &timeout,
			Retries:  &retries,
			HTTP: &hcloud.LoadBalancerCreateOptsServiceHealthCheckHTTP{
				Domain: &domain,
				Path:   &path,
				TLS:    &tls,
			},
		}))
	})
})

var _ = Describe("health check option builders", func() {
	spec := func(healthCheckType string) *infrav2.LoadBalancerHealthCheckSpec {
		return &infrav2.LoadBalancerHealthCheckSpec{
			Type:     healthCheckType,
			Interval: &metav1.Duration{Duration: 5 * time.Second},
			Timeout:  &metav1.Duration{Duration: 2 * time.Second},
			Retries:  ptr.To(5),
			Domain:   ptr.To("example.com"),
			Path:     ptr.To("/readyz"),
		}
	}

	Describe("healthCheckCreateOpts", func() {
		It("returns nil when the spec is nil", func() {
			Expect(healthCheckCreateOpts(nil)).To(BeNil())
		})

		It("omits the HTTP sub-options for a tcp health check", func() {
			opts := healthCheckCreateOpts(spec("tcp"))
			Expect(opts.Protocol).To(Equal(hcloud.LoadBalancerServiceProtocolTCP))
			Expect(opts.HTTP).To(BeNil())
		})

		It("sets TLS for an https health check", func() {
			opts := healthCheckCreateOpts(spec("https"))
			Expect(opts.Protocol).To(Equal(hcloud.LoadBalancerServiceProtocolHTTPS))
			Expect(opts.HTTP).NotTo(BeNil())
			Expect(*opts.HTTP.TLS).To(BeTrue())
			Expect(*opts.HTTP.Domain).To(Equal("example.com"))
			Expect(*opts.HTTP.Path).To(Equal("/readyz"))
		})
	})

	Describe("healthCheckAddOpts", func() {
		It("returns nil when the spec is nil", func() {
			Expect(healthCheckAddOpts(nil)).To(BeNil())
		})

		It("mirrors the spec for an http health check", func() {
			opts := healthCheckAddOpts(spec("http"))
			Expect(opts.Protocol).To(Equal(hcloud.LoadBalancerServiceProtocolHTTP))
			Expect(*opts.Retries).To(Equal(5))
			Expect(*opts.HTTP.TLS).To(BeFalse())
		})
	})

	Describe("healthCheckUpdateOpts", func() {
		It("returns nil when the spec is nil", func() {
			Expect(healthCheckUpdateOpts(nil)).To(BeNil())
		})

		It("mirrors the spec for an http health check", func() {
			opts := healthCheckUpdateOpts(spec("http"))
			Expect(opts.Protocol).To(Equal(hcloud.LoadBalancerServiceProtocolHTTP))
			Expect(*opts.Interval).To(Equal(5 * time.Second))
			Expect(*opts.HTTP.Domain).To(Equal("example.com"))
		})
	})

	Describe("healthCheckNeedsUpdate", func() {
		// lb.Services[0] (listen port 443) is an http health check with domain "example.com",
		// path "/", interval 15s, timeout 10s, retries 3, TLS false (see loadbalancer_suite_test.go).
		var got hcloud.LoadBalancerServiceHealthCheck
		BeforeEach(func() {
			got = lb.Services[0].HealthCheck
		})

		It("reports no update needed when the spec matches the live state", func() {
			want := &infrav2.LoadBalancerHealthCheckSpec{
				Type:     "http",
				Interval: &metav1.Duration{Duration: 15 * time.Second},
				Timeout:  &metav1.Duration{Duration: 10 * time.Second},
				Retries:  ptr.To(3),
				Domain:   ptr.To("example.com"),
				Path:     ptr.To("/"),
			}
			Expect(healthCheckNeedsUpdate(want, got)).To(BeFalse())
		})

		It("ignores fields the spec leaves unset", func() {
			want := &infrav2.LoadBalancerHealthCheckSpec{Type: "http"}
			Expect(healthCheckNeedsUpdate(want, got)).To(BeFalse())
		})

		It("reports an update when the protocol differs", func() {
			want := &infrav2.LoadBalancerHealthCheckSpec{Type: "tcp"}
			Expect(healthCheckNeedsUpdate(want, got)).To(BeTrue())
		})

		It("reports an update when the interval differs", func() {
			want := &infrav2.LoadBalancerHealthCheckSpec{
				Type:     "http",
				Interval: &metav1.Duration{Duration: 30 * time.Second},
			}
			Expect(healthCheckNeedsUpdate(want, got)).To(BeTrue())
		})

		It("reports an update when the path differs", func() {
			want := &infrav2.LoadBalancerHealthCheckSpec{
				Type: "http",
				Path: ptr.To("/readyz"),
			}
			Expect(healthCheckNeedsUpdate(want, got)).To(BeTrue())
		})

		It("reports an update when switching from http to https", func() {
			want := &infrav2.LoadBalancerHealthCheckSpec{Type: "https"}
			Expect(healthCheckNeedsUpdate(want, got)).To(BeTrue())
		})

		It("treats an empty type as tcp and reports the mismatch against the http fixture", func() {
			want := &infrav2.LoadBalancerHealthCheckSpec{}
			Expect(healthCheckNeedsUpdate(want, got)).To(BeTrue())
		})
	})
})
