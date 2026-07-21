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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		Status: infrav2.HetznerClusterStatus{
			// reconcileServices is always called after Reconcile has already populated this
			// from statusFromHCloudLB, so mirror that invariant here.
			ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{},
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

// TestReconcileServices_EnableProxyProtocol_UpdatesStatusInSameReconcile verifies that
// status.controlPlaneLoadBalancer.proxyProtocolEnabled is set as soon as the kube-API service is
// (re)created with proxy protocol, instead of waiting for the next reconcile to pick it up.
func TestReconcileServices_EnableProxyProtocol_UpdatesStatusInSameReconcile(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{}

	mockClient.On("AddServiceToLoadBalancer", mock.Anything, hcloudLB, mock.Anything).Return(nil)

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.True(t, hetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled)
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

func controlPlaneMachineForProxy(name string, annotated bool) *clusterv1.Machine {
	annotations := map[string]string{}
	if annotated {
		annotations[infrav2.ProxyProtocolForControlPlaneLoadBalancerAnnotation] = "true"
	}
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:         "test-cluster",
				clusterv1.MachineControlPlaneLabel: "",
			},
			Annotations: annotations,
		},
	}
}

// newProxyMigrationService builds a Service whose proxy-protocol readiness is decided from the
// given control-plane machines in the management cluster.
func newProxyMigrationService(t *testing.T, mockClient *mocks.Client, machines ...client.Object) *Service {
	t.Helper()
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Namespace = metav1.NamespaceDefault
	hetznerCluster.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)

	svc := newTestService(t, hetznerCluster, mockClient)
	svc.scope.Client = fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(machines...).Build()
	svc.scope.APIReader = svc.scope.Client
	svc.scope.Cluster = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: metav1.NamespaceDefault},
	}
	return svc
}

// TestReconcileServices_ProxyProtocolMigration_MachinesNotReady verifies that proxy protocol is
// NOT switched on when it is requested but a control-plane machine has not yet been annotated.
func TestReconcileServices_ProxyProtocolMigration_MachinesNotReady(t *testing.T) {
	mockClient := &mocks.Client{}
	svc := newProxyMigrationService(t, mockClient,
		controlPlaneMachineForProxy("cp-1", true),
		controlPlaneMachineForProxy("cp-2", false),
	)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, Proxyprotocol: false},
		},
	}

	res, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter, "should requeue while a control-plane machine is not annotated")
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_ProxyProtocolMigration_MachinesReady_SwitchesInPlace verifies that once
// every control-plane machine is annotated, proxy protocol is switched on in place via
// UpdateServiceOnLoadBalancer without deleting the kube-API service.
func TestReconcileServices_ProxyProtocolMigration_MachinesReady_SwitchesInPlace(t *testing.T) {
	mockClient := &mocks.Client{}
	svc := newProxyMigrationService(t, mockClient,
		controlPlaneMachineForProxy("cp-1", true),
		controlPlaneMachineForProxy("cp-2", true),
	)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, Proxyprotocol: false},
		},
	}

	mockClient.On("UpdateServiceOnLoadBalancer", mock.Anything, hcloudLB, mock.Anything, mock.Anything).Return(nil)

	res, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.True(t, svc.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled)
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_ProxyProtocolMigration_MachinesNotReady_StillReconcilesExtraServices
// verifies that while proxy protocol migration is waiting (a control-plane machine not yet
// annotated), the function still reconciles extraServices instead of returning early.
func TestReconcileServices_ProxyProtocolMigration_MachinesNotReady_StillReconcilesExtraServices(t *testing.T) {
	const extraListenPort = 8080
	const extraDestPort = 8081

	mockClient := &mocks.Client{}
	svc := newProxyMigrationService(t, mockClient,
		controlPlaneMachineForProxy("cp-1", true),
		controlPlaneMachineForProxy("cp-2", false),
	)
	svc.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices = []infrav2.LoadBalancerServiceSpec{
		{Protocol: "tcp", ListenPort: extraListenPort, DestinationPort: extraDestPort},
	}
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, Proxyprotocol: false}, // kube-API exists without proxy protocol
			// extraService is missing from the LB — should be added even while waiting for proxy protocol
		},
	}

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
