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
	"context"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	fakehcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
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

	It("creates the kube-apiserver service with the configured health check", func() {
		hetznerCluster.Spec.ControlPlaneLoadBalancer.HealthCheck = &infrav2.LoadBalancerHealthCheckSpec{
			Protocol: "http",
			Path:     ptr.To("/readyz"),
		}

		createOpts := createOptsFromSpec(hetznerCluster)

		hc := createOpts.Services[0].HealthCheck
		Expect(hc).NotTo(BeNil())
		Expect(string(hc.Protocol)).To(Equal("http"))
		Expect(hc.Port).NotTo(BeNil())
		Expect(*hc.Port).To(Equal(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port))
		Expect(hc.HTTP).NotTo(BeNil())
		Expect(hc.HTTP.Path).NotTo(BeNil())
		Expect(*hc.HTTP.Path).To(Equal("/readyz"))
	})
})

var _ = Describe("reconcileServices health check migration", func() {
	const (
		namespace   = "default"
		clusterName = "test-cluster"
	)

	cpMachine := func(name string, annotated bool) *infrav1.HCloudMachine {
		annotations := map[string]string{}
		if annotated {
			annotations[infrav2.HTTPHealthCheckForControlPlaneLoadBalancerAnnotation] = "true"
		}
		return &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel:         clusterName,
					clusterv1.MachineControlPlaneLabel: "",
				},
				Annotations: annotations,
			},
		}
	}

	// newServiceWithExistingCheck sets up a fake load balancer whose kube-API service already has
	// existingCheck, against a spec that wants the http /readyz check on port 8443.
	newServiceWithExistingCheck := func(existingCheck *hcloud.LoadBalancerAddServiceOptsHealthCheck, machines ...client.Object) (*Service, *hcloud.LoadBalancer) {
		hcloudClient := fakehcloudclient.NewHCloudClientFactory().NewClient("")
		createdLB, err := hcloudClient.CreateLoadBalancer(context.Background(), hcloud.LoadBalancerCreateOpts{
			Name:      "test-lb",
			Algorithm: &hcloud.LoadBalancerAlgorithm{Type: hcloud.LoadBalancerAlgorithmTypeRoundRobin},
		})
		Expect(err).NotTo(HaveOccurred())

		listenPort := 6443
		destinationPort := 6443
		proxyprotocolOff := false
		Expect(hcloudClient.AddServiceToLoadBalancer(context.Background(), createdLB, hcloud.LoadBalancerAddServiceOpts{
			Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
			ListenPort:      &listenPort,
			DestinationPort: &destinationPort,
			Proxyprotocol:   &proxyprotocolOff,
			HealthCheck:     existingCheck,
		})).To(Succeed())

		scheme := runtime.NewScheme()
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(infrav1.AddToScheme(scheme)).To(Succeed())
		Expect(infrav2.AddToScheme(scheme)).To(Succeed())

		checkPort := 8443
		hetznerCluster := &infrav2.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName},
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneEndpoint: infrav2.APIEndpoint{Port: 6443},
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: true,
					Port:    6443,
					HealthCheck: &infrav2.LoadBalancerHealthCheckSpec{
						Protocol: "http",
						Path:     ptr.To("/readyz"),
						Port:     &checkPort,
					},
				},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{},
			},
		}

		clusterScope := &scope.ClusterScope{
			HetznerCluster: hetznerCluster,
			HCloudClient:   hcloudClient,
			Client:         fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(machines...).Build(),
			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
			},
		}
		return NewService(clusterScope), createdLB
	}

	// newServiceWantingHTTPCheck starts from the default tcp check, mimicking an existing cluster
	// that wants to migrate.
	newServiceWantingHTTPCheck := func(machines ...client.Object) (*Service, *hcloud.LoadBalancer) {
		tcpPort := 6443
		return newServiceWithExistingCheck(
			&hcloud.LoadBalancerAddServiceOptsHealthCheck{Protocol: hcloud.LoadBalancerServiceProtocolTCP, Port: &tcpPort},
			machines...,
		)
	}

	It("switches the health check to http once all control-plane machines carry the annotation", func() {
		svc, lb := newServiceWantingHTTPCheck(cpMachine("cp-1", true), cpMachine("cp-2", true))

		res, err := svc.reconcileServices(context.Background(), lb)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())

		Expect(lb.Services).To(HaveLen(1))
		Expect(string(lb.Services[0].HealthCheck.Protocol)).To(Equal("http"))
		Expect(lb.Services[0].HealthCheck.HTTP).NotTo(BeNil())
		Expect(lb.Services[0].HealthCheck.HTTP.Path).To(Equal("/readyz"))
	})

	It("requeues and keeps the tcp health check while a control-plane machine misses the annotation", func() {
		svc, lb := newServiceWantingHTTPCheck(cpMachine("cp-1", true), cpMachine("cp-2", false))

		res, err := svc.reconcileServices(context.Background(), lb)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).NotTo(BeZero())

		Expect(lb.Services).To(HaveLen(1))
		Expect(string(lb.Services[0].HealthCheck.Protocol)).To(Equal("tcp"))
		Expect(lb.Services[0].HealthCheck.HTTP).To(BeNil())
	})

	It("changes the path on an already http check without waiting for the annotation", func() {
		checkPort := 8443
		oldPath := "/healthz"
		tlsOff := false
		// No machines at all, so the gate would block if this path went through it.
		svc, lb := newServiceWithExistingCheck(&hcloud.LoadBalancerAddServiceOptsHealthCheck{
			Protocol: hcloud.LoadBalancerServiceProtocolHTTP,
			Port:     &checkPort,
			HTTP:     &hcloud.LoadBalancerAddServiceOptsHealthCheckHTTP{Path: &oldPath, TLS: &tlsOff},
		})

		res, err := svc.reconcileServices(context.Background(), lb)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())

		Expect(lb.Services).To(HaveLen(1))
		Expect(lb.Services[0].HealthCheck.HTTP).NotTo(BeNil())
		Expect(lb.Services[0].HealthCheck.HTTP.Path).To(Equal("/readyz"))
	})
})
