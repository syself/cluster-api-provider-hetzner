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
	"errors"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	fakehcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

var _ = Describe("Loadbalancer", func() {
	Context("hcloud cluster has network attached", func() {
		var sts *infrav1.LoadBalancerStatus
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
		var sts *infrav1.LoadBalancerStatus
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

var _ = Describe("reconcileServices", func() {
	It("sets status.controlPlaneLoadBalancer.proxyProtocolEnabled as soon as the kube-API service is created with proxy protocol, without waiting for the next reconcile", func() {
		hcloudClient := fakehcloudclient.NewHCloudClientFactory().NewClient("")
		createdLB, err := hcloudClient.CreateLoadBalancer(context.Background(), hcloud.LoadBalancerCreateOpts{
			Name:      "test-lb",
			Algorithm: &hcloud.LoadBalancerAlgorithm{Type: hcloud.LoadBalancerAlgorithmTypeRoundRobin},
		})
		Expect(err).NotTo(HaveOccurred())

		hetznerCluster := &infrav1.HetznerCluster{
			Spec: infrav1.HetznerClusterSpec{
				ControlPlaneEndpoint: &clusterv1beta1.APIEndpoint{Port: 6443},
				ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
					Enabled:             true,
					EnableProxyProtocol: true,
					Port:                6443,
				},
			},
			Status: infrav1.HetznerClusterStatus{
				// Simulates the snapshot taken from the LB state at the start of Reconcile,
				// before the kube-API service (and thus proxy protocol) existed on the LB.
				ControlPlaneLoadBalancer: &infrav1.LoadBalancerStatus{},
			},
		}

		svc := &Service{&scope.ClusterScope{HetznerCluster: hetznerCluster, HCloudClient: hcloudClient}}

		_, err = svc.reconcileServices(context.Background(), createdLB)
		Expect(err).NotTo(HaveOccurred())
		Expect(hetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled).To(BeTrue())
	})
})

var _ = Describe("reconcileServices proxy protocol migration", func() {
	const (
		namespace   = "default"
		clusterName = "test-cluster"
	)

	controlPlaneMachine := func(name string, annotated bool) *clusterv1.Machine {
		annotations := map[string]string{}
		if annotated {
			annotations[infrav1.ProxyProtocolForControlPlaneLoadBalancerAnnotation] = "true"
		}
		return &clusterv1.Machine{
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

	// newServiceWithExistingKubeAPIService sets up a fake LB that already has a kube-API
	// service without proxy protocol, mimicking an existing cluster that wants to migrate.
	newServiceWithExistingKubeAPIService := func(machines ...client.Object) (*Service, *hcloud.LoadBalancer) {
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
		})).To(Succeed())

		scheme := runtime.NewScheme()
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(infrav1.AddToScheme(scheme)).To(Succeed())

		hetznerCluster := &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName},
			Spec: infrav1.HetznerClusterSpec{
				ControlPlaneEndpoint: &clusterv1beta1.APIEndpoint{Port: 6443},
				ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
					Enabled:             true,
					EnableProxyProtocol: true,
					Port:                6443,
				},
			},
			Status: infrav1.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav1.LoadBalancerStatus{},
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

	It("switches proxy protocol on in place once all control-plane machines are annotated, without deleting the service", func() {
		svc, lb := newServiceWithExistingKubeAPIService(
			controlPlaneMachine("cp-1", true),
			controlPlaneMachine("cp-2", true),
		)

		res, err := svc.reconcileServices(context.Background(), lb)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())

		Expect(lb.Services).To(HaveLen(1))
		Expect(lb.Services[0].ListenPort).To(Equal(6443))
		Expect(lb.Services[0].Proxyprotocol).To(BeTrue())
		Expect(svc.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled).To(BeTrue())
	})

	It("requeues and leaves proxy protocol off while a control-plane machine is missing the annotation", func() {
		svc, lb := newServiceWithExistingKubeAPIService(
			controlPlaneMachine("cp-1", true),
			controlPlaneMachine("cp-2", false),
		)

		res, err := svc.reconcileServices(context.Background(), lb)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).NotTo(BeZero())

		Expect(lb.Services).To(HaveLen(1))
		Expect(lb.Services[0].Proxyprotocol).To(BeFalse())
		Expect(svc.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled).To(BeFalse())
	})
})

var _ = Describe("createOptsFromSpec", func() {
	var hetznerCluster *infrav1.HetznerCluster
	var wantCreateOpts hcloud.LoadBalancerCreateOpts
	BeforeEach(func() {
		lbType := "lb11"
		lbRegion := "fsn1"
		controlPlaneEndpointPort := 22
		lbPort := 6443
		var networkID int64 = 42

		hetznerCluster = &infrav1.HetznerCluster{
			Spec: infrav1.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
					Name:      nil,
					Algorithm: infrav1.LoadBalancerAlgorithmTypeLeastConnections,
					Type:      lbType,
					Region:    infrav1.Region(lbRegion),
					Port:      lbPort,
				},
				ControlPlaneEndpoint: &clusterv1beta1.APIEndpoint{Port: int32(controlPlaneEndpointPort)},
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
			Name:             "",
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

		createOpts, err := createOptsFromSpec(hetznerCluster)
		Expect(err).To(BeNil())

		// ignore random name
		createOpts.Name = ""

		Expect(createOpts).To(Equal(wantCreateOpts))
	})

	It("creates specs for cluster with network", func() {
		createOpts, err := createOptsFromSpec(hetznerCluster)
		Expect(err).To(BeNil())

		// ignore random name
		createOpts.Name = ""

		Expect(createOpts).To(Equal(wantCreateOpts))
	})

	It("creates specs for cluster without load balancer name set", func() {
		hetznerCluster.Spec.ControlPlaneLoadBalancer.Name = nil

		createOpts, err := createOptsFromSpec(hetznerCluster)
		Expect(err).To(BeNil())

		// should generate correct name
		Expect(createOpts.Name).To(HavePrefix("hetzner-cluster-kube-apiserver-"))

		// should be the same for all other specs
		createOpts.Name = ""
		wantCreateOpts.Name = ""
		Expect(createOpts).To(Equal(wantCreateOpts))
	})

	It("creates the kube-apiserver service with proxy protocol on when EnableProxyProtocol is true", func() {
		hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true
		proxyprotocol := true
		wantCreateOpts.Services[0].Proxyprotocol = &proxyprotocol

		createOpts, err := createOptsFromSpec(hetznerCluster)
		Expect(err).To(BeNil())

		// ignore random name
		createOpts.Name = ""

		Expect(createOpts).To(Equal(wantCreateOpts))
	})

	It("does not enable proxy protocol directly when the cluster already had a load balancer before (e.g. it was lost and is being recreated)", func() {
		hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true
		hetznerCluster.Status.ControlPlaneLoadBalancer = &infrav1.LoadBalancerStatus{IPv4: "1.2.3.4"}

		createOpts, err := createOptsFromSpec(hetznerCluster)
		Expect(err).To(BeNil())

		// ignore random name
		createOpts.Name = ""

		Expect(createOpts).To(Equal(wantCreateOpts))
	})

	It("returns ErrControlPlaneEndpointNotSet", func() {
		hetznerCluster.Spec.ControlPlaneEndpoint = nil

		_, err := createOptsFromSpec(hetznerCluster)
		Expect(errors.Is(err, ErrControlPlaneEndpointNotSet)).To(BeTrue())
	})
})
