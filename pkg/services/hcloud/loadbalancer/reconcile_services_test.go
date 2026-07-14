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

package loadbalancer

import (
	"context"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
)

const (
	testKubeAPIListenPort = 443
	testLBDestPort        = 6443
)

func newTestService(t *testing.T, hetznerCluster *infrav2.HetznerCluster, mockClient *mocks.Client) *Service {
	t.Helper()
	return &Service{scope: &scope.ClusterScope{
		HetznerCluster: hetznerCluster,
		HCloudClient:   mockClient,
	}}
}

func newTestHetznerCluster() *infrav2.HetznerCluster {
	return &infrav2.HetznerCluster{
		Spec: infrav2.HetznerClusterSpec{
			ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
				Port: testLBDestPort,
			},
			ControlPlaneEndpoint: infrav2.APIEndpoint{Port: testKubeAPIListenPort},
		},
	}
}

func TestReconcileServices_KubeAPIPortZero_NoChanges(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneEndpoint.Port = 0

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)

	_, err := svc.reconcileServices(context.Background(), &hcloud.LoadBalancer{})
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestReconcileServices_NewCluster_AddsKubeAPIServiceWithoutProxyProtocol(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{}

	var capturedOpts hcloud.LoadBalancerAddServiceOpts
	mockClient.On("AddServiceToLoadBalancer", mock.Anything, hcloudLB, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedOpts = args.Get(2).(hcloud.LoadBalancerAddServiceOpts)
		}).
		Return(nil)

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.Equal(t, testKubeAPIListenPort, *capturedOpts.ListenPort)
	require.Equal(t, testLBDestPort, *capturedOpts.DestinationPort)
	require.False(t, *capturedOpts.Proxyprotocol)
	require.Equal(t, hcloud.LoadBalancerServiceProtocol("tcp"), capturedOpts.Protocol)
	mockClient.AssertExpectations(t)
}

func TestReconcileServices_NewCluster_EnableProxyProtocol_AddsServiceWithProxyProtocol(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{}

	var capturedOpts hcloud.LoadBalancerAddServiceOpts
	mockClient.On("AddServiceToLoadBalancer", mock.Anything, hcloudLB, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedOpts = args.Get(2).(hcloud.LoadBalancerAddServiceOpts)
		}).
		Return(nil)

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.True(t, *capturedOpts.Proxyprotocol)
	mockClient.AssertExpectations(t)
}

func TestReconcileServices_KubeAPIServiceAlreadyExists_NoChanges(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, Proxyprotocol: false},
		},
	}

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestReconcileServices_ExtraServiceMissing_AddsIt(t *testing.T) {
	const extraListenPort = 8080
	const extraDestPort = 8081

	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices = []infrav2.LoadBalancerServiceSpec{
		{Protocol: "tcp", ListenPort: extraListenPort, DestinationPort: extraDestPort},
	}

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort}, // kube-API already on LB
		},
	}

	var capturedOpts hcloud.LoadBalancerAddServiceOpts
	mockClient.On("AddServiceToLoadBalancer", mock.Anything, hcloudLB, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedOpts = args.Get(2).(hcloud.LoadBalancerAddServiceOpts)
		}).
		Return(nil)

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.Equal(t, extraListenPort, *capturedOpts.ListenPort)
	require.Equal(t, extraDestPort, *capturedOpts.DestinationPort)
	mockClient.AssertExpectations(t)
}

func TestReconcileServices_StaleServiceOnLB_DeletesIt(t *testing.T) {
	const stalePort = 9090

	hetznerCluster := newTestHetznerCluster()
	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort},
			{ListenPort: stalePort},
		},
	}

	mockClient.On("DeleteServiceFromLoadBalancer", mock.Anything, hcloudLB, stalePort).
		Return(nil)

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestReconcileServices_ProxyProtocolAlreadyActive_NoChanges(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, Proxyprotocol: true},
		},
	}

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_ProxyProtocolMigration_NodesNotReady verifies that the kube-API
// service is NOT recreated when proxy protocol is requested but the control-plane nodes
// have not yet signalled readiness (annotation absent / workload cluster unreachable).
func TestReconcileServices_ProxyProtocolMigration_NodesNotReady(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, Proxyprotocol: false},
		},
	}

	// Provide a k8s client with no secrets so AllControlPlaneNodesReadyForProxyProtocol
	// returns false (kubeconfig secret not found → nodes considered not ready).
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeK8sClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	svc.scope.Client = fakeK8sClient
	svc.scope.APIReader = fakeK8sClient
	svc.scope.Cluster = &clusterv1.Cluster{}

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_ProxyProtocolMigration_NodesNotReady_StillReconcilesExtraServices
// verifies that when proxy protocol migration is waiting (CP nodes not ready), the function
// still reconciles extraServices instead of returning early.
func TestReconcileServices_ProxyProtocolMigration_NodesNotReady_StillReconcilesExtraServices(t *testing.T) {
	const extraListenPort = 8080
	const extraDestPort = 8081

	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true
	hetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices = []infrav2.LoadBalancerServiceSpec{
		{Protocol: "tcp", ListenPort: extraListenPort, DestinationPort: extraDestPort},
	}

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, Proxyprotocol: false}, // kube-API exists without proxy protocol
			// extraService is missing from the LB — should be added even while waiting for proxy protocol
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeK8sClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	svc.scope.Client = fakeK8sClient
	svc.scope.APIReader = fakeK8sClient
	svc.scope.Cluster = &clusterv1.Cluster{}

	// The extra service must be added even though proxy protocol migration is pending.
	var capturedOpts hcloud.LoadBalancerAddServiceOpts
	mockClient.On("AddServiceToLoadBalancer", mock.Anything, hcloudLB, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedOpts = args.Get(2).(hcloud.LoadBalancerAddServiceOpts)
		}).
		Return(nil)

	result, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	mockClient.AssertExpectations(t) // fails here if AddServiceToLoadBalancer was never called
	require.NotNil(t, capturedOpts.ListenPort, "AddServiceToLoadBalancer should have been called for extra service")
	require.Equal(t, extraListenPort, *capturedOpts.ListenPort)
	require.Equal(t, extraDestPort, *capturedOpts.DestinationPort)
	require.NotZero(t, result.RequeueAfter, "should requeue while waiting for proxy protocol migration")
}
