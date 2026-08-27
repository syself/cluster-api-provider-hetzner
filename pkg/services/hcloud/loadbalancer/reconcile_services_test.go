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
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
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

// TestReconcileServices_HealthCheckSet_AddsKubeAPIServiceWithHealthCheck verifies that the health
// check from spec is carried into AddServiceToLoadBalancer when the kube-API service is created
// via reconcileServices instead of via createOptsFromSpec (e.g. taking over an existing LB).
func TestReconcileServices_HealthCheckSet_AddsKubeAPIServiceWithHealthCheck(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.HealthCheck = &infrav2.LoadBalancerHealthCheckSpec{
		Protocol: "http",
		Path:     ptr.To("/readyz"),
	}

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
	require.NotNil(t, capturedOpts.HealthCheck)
	require.Equal(t, hcloud.LoadBalancerServiceProtocolHTTP, capturedOpts.HealthCheck.Protocol)
	require.Equal(t, "/readyz", *capturedOpts.HealthCheck.HTTP.Path)
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_HealthCheckPathChange_UpdatesInPlaceWithoutGate verifies that a change
// which stays within http/https (here: a path change) is applied immediately via
// UpdateServiceOnLoadBalancer, without checking the control-plane rollout gate — only a switch
// away from tcp needs that gate, since only that switch can mark a not-yet-ready backend
// unhealthy.
func TestReconcileServices_HealthCheckPathChange_UpdatesInPlaceWithoutGate(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.HealthCheck = &infrav2.LoadBalancerHealthCheckSpec{
		Protocol: "http",
		Path:     ptr.To("/readyz"),
	}

	mockClient := &mocks.Client{}
	// scope.Client is intentionally left nil: reaching it would panic, so this also proves the
	// gate is never consulted for a change that stays within http/https.
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{
				ListenPort: testKubeAPIListenPort,
				HealthCheck: hcloud.LoadBalancerServiceHealthCheck{
					Protocol: hcloud.LoadBalancerServiceProtocolHTTP,
					Port:     testLBDestPort,
					HTTP:     &hcloud.LoadBalancerServiceHealthCheckHTTP{Path: "/oldpath"},
				},
			},
		},
	}

	var capturedOpts hcloud.LoadBalancerUpdateServiceOpts
	mockClient.On("UpdateServiceOnLoadBalancer", mock.Anything, hcloudLB, testKubeAPIListenPort, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedOpts = args.Get(3).(hcloud.LoadBalancerUpdateServiceOpts)
		}).
		Return(nil)

	res, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.NotNil(t, capturedOpts.HealthCheck)
	require.Equal(t, hcloud.LoadBalancerServiceProtocolHTTP, capturedOpts.HealthCheck.Protocol)
	require.Equal(t, "/readyz", *capturedOpts.HealthCheck.HTTP.Path)
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_HealthCheckMatchesLive_NoUpdateCall verifies that no update is issued
// when the live health check already matches the fields set in spec.
func TestReconcileServices_HealthCheckMatchesLive_NoUpdateCall(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Spec.ControlPlaneLoadBalancer.HealthCheck = &infrav2.LoadBalancerHealthCheckSpec{Protocol: "tcp"}

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{
				ListenPort:  testKubeAPIListenPort,
				HealthCheck: hcloud.LoadBalancerServiceHealthCheck{Protocol: hcloud.LoadBalancerServiceProtocolTCP, Port: testLBDestPort},
			},
		},
	}

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	// AssertExpectations fails if AddServiceToLoadBalancer/UpdateServiceOnLoadBalancer were called
	// without a matching .On(...) — none were set up here, so any call would fail the test.
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_HealthCheckUnset_LeavesLiveConfigAlone verifies that CAPH never touches
// the load balancer's health check when spec.healthCheck is omitted, even if the live service's
// health check doesn't match Hetzner's own tcp default (e.g. configured out-of-band).
func TestReconcileServices_HealthCheckUnset_LeavesLiveConfigAlone(t *testing.T) {
	hetznerCluster := newTestHetznerCluster()

	mockClient := &mocks.Client{}
	svc := newTestService(t, hetznerCluster, mockClient)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{
				ListenPort: testKubeAPIListenPort,
				HealthCheck: hcloud.LoadBalancerServiceHealthCheck{
					Protocol: hcloud.LoadBalancerServiceProtocolHTTP,
					HTTP:     &hcloud.LoadBalancerServiceHealthCheckHTTP{Path: "/custom"},
				},
			},
		},
	}

	_, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func controlPlaneMachineWithAnnotation(name, annotation string, annotated bool) *infrav2.HCloudMachine {
	annotations := map[string]string{}
	if annotated {
		annotations[annotation] = "true"
	}
	return &infrav2.HCloudMachine{
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

func controlPlaneMachineForProxy(name string, annotated bool) *infrav2.HCloudMachine {
	return controlPlaneMachineWithAnnotation(name, infrav2.ProxyProtocolForControlPlaneLoadBalancerAnnotation, annotated)
}

func controlPlaneMachineForHTTPHealthCheck(name string, annotated bool) *infrav2.HCloudMachine {
	return controlPlaneMachineWithAnnotation(name, infrav2.HTTPHealthCheckForControlPlaneLoadBalancerAnnotation, annotated)
}

// newMigrationTestService builds a Service backed by a fake management-cluster client seeded with
// the given control-plane infrastructure machines, for tests that gate a change on
// AllControlPlaneInfraMachinesAnnotatedFor{ProxyProtocol,HTTPHealthCheck}. configureSpec sets
// whatever part of the spec the test is migrating.
func newMigrationTestService(t *testing.T, mockClient *mocks.Client, configureSpec func(*infrav2.HetznerCluster), machines ...client.Object) *Service {
	t.Helper()
	hetznerCluster := newTestHetznerCluster()
	hetznerCluster.Namespace = metav1.NamespaceDefault
	configureSpec(hetznerCluster)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = infrav2.AddToScheme(scheme)

	svc := newTestService(t, hetznerCluster, mockClient)
	svc.scope.Client = fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(machines...).Build()
	svc.scope.APIReader = svc.scope.Client
	svc.scope.Cluster = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: metav1.NamespaceDefault},
	}
	return svc
}

// newProxyMigrationService builds a Service whose proxy-protocol readiness is decided from the
// given control-plane machines in the management cluster.
func newProxyMigrationService(t *testing.T, mockClient *mocks.Client, machines ...client.Object) *Service {
	t.Helper()
	return newMigrationTestService(t, mockClient, func(hc *infrav2.HetznerCluster) {
		hc.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true
	}, machines...)
}

// newHealthCheckMigrationService builds a Service whose http-health-check readiness is decided
// from the given control-plane machines in the management cluster.
func newHealthCheckMigrationService(t *testing.T, mockClient *mocks.Client, machines ...client.Object) *Service {
	t.Helper()
	return newMigrationTestService(t, mockClient, func(hc *infrav2.HetznerCluster) {
		hc.Spec.ControlPlaneLoadBalancer.HealthCheck = &infrav2.LoadBalancerHealthCheckSpec{
			Protocol: "http",
			Path:     ptr.To("/readyz"),
		}
	}, machines...)
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
	require.Equal(t, 2*time.Minute, res.RequeueAfter, "should requeue after 2 minutes while a control-plane machine is not annotated")

	cond := conditions.Get(svc.scope.HetznerCluster, infrav2.HetznerClusterLoadBalancerReadyCondition)
	require.NotNil(t, cond, "LoadBalancerReady condition should report the proxy protocol wait")
	require.Equal(t, metav1.ConditionFalse, cond.Status)
	require.Equal(t, infrav2.HetznerClusterLoadBalancerWaitingToActivateProxyProtocolReason, cond.Reason)

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

// TestReconcileServices_HealthCheckMigration_MachinesNotReady_Requeues verifies that switching
// the kube-API service's health check from tcp to http is NOT applied while a control-plane
// machine has not yet been annotated for it, mirroring the proxy-protocol migration gate.
func TestReconcileServices_HealthCheckMigration_MachinesNotReady_Requeues(t *testing.T) {
	mockClient := &mocks.Client{}
	svc := newHealthCheckMigrationService(t, mockClient,
		controlPlaneMachineForHTTPHealthCheck("cp-1", true),
		controlPlaneMachineForHTTPHealthCheck("cp-2", false),
	)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{
				ListenPort:  testKubeAPIListenPort,
				HealthCheck: hcloud.LoadBalancerServiceHealthCheck{Protocol: hcloud.LoadBalancerServiceProtocolTCP, Port: testLBDestPort},
			},
		},
	}

	res, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter, "should requeue while a control-plane machine is not annotated")
	// No UpdateServiceOnLoadBalancer expectation was set up, so AssertExpectations fails here if
	// the tcp check got switched to http anyway.
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_HealthCheckMigration_MachinesReady_SwitchesInPlace verifies that once
// every control-plane machine is annotated, the health check is switched from tcp to http in
// place via UpdateServiceOnLoadBalancer.
func TestReconcileServices_HealthCheckMigration_MachinesReady_SwitchesInPlace(t *testing.T) {
	mockClient := &mocks.Client{}
	svc := newHealthCheckMigrationService(t, mockClient,
		controlPlaneMachineForHTTPHealthCheck("cp-1", true),
		controlPlaneMachineForHTTPHealthCheck("cp-2", true),
	)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{
				ListenPort:  testKubeAPIListenPort,
				HealthCheck: hcloud.LoadBalancerServiceHealthCheck{Protocol: hcloud.LoadBalancerServiceProtocolTCP, Port: testLBDestPort},
			},
		},
	}

	var capturedOpts hcloud.LoadBalancerUpdateServiceOpts
	mockClient.On("UpdateServiceOnLoadBalancer", mock.Anything, hcloudLB, testKubeAPIListenPort, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedOpts = args.Get(3).(hcloud.LoadBalancerUpdateServiceOpts)
		}).
		Return(nil)

	res, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.NotNil(t, capturedOpts.HealthCheck)
	require.Equal(t, hcloud.LoadBalancerServiceProtocolHTTP, capturedOpts.HealthCheck.Protocol)
	require.Equal(t, "/readyz", *capturedOpts.HealthCheck.HTTP.Path)
	mockClient.AssertExpectations(t)
}

// TestReconcileServices_HealthCheckMigration_MachinesNotReady_StillReconcilesExtraServices
// verifies that while the health-check migration is waiting (a control-plane machine not yet
// annotated), the function still reconciles extraServices instead of returning early.
func TestReconcileServices_HealthCheckMigration_MachinesNotReady_StillReconcilesExtraServices(t *testing.T) {
	const extraListenPort = 8080
	const extraDestPort = 8081

	mockClient := &mocks.Client{}
	svc := newHealthCheckMigrationService(t, mockClient,
		controlPlaneMachineForHTTPHealthCheck("cp-1", true),
		controlPlaneMachineForHTTPHealthCheck("cp-2", false),
	)
	svc.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.ExtraServices = []infrav2.LoadBalancerServiceSpec{
		{Protocol: "tcp", ListenPort: extraListenPort, DestinationPort: extraDestPort},
	}
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{ListenPort: testKubeAPIListenPort, HealthCheck: hcloud.LoadBalancerServiceHealthCheck{Protocol: hcloud.LoadBalancerServiceProtocolTCP, Port: testLBDestPort}},
			// extraService is missing from the LB — should be added even while waiting for the health-check migration
		},
	}

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
	require.NotZero(t, result.RequeueAfter, "should requeue while waiting for the health-check migration")
}

// TestReconcileServices_HealthCheckMigration_MachinesNotReady_StillEnablesProxyProtocol verifies
// that a pending health-check migration (still waiting on its annotation) does not block enabling
// proxy protocol in the same reconcile: the two gates are independent, each keyed on its own
// annotation, so one being pending must not delay the other that is already satisfied.
func TestReconcileServices_HealthCheckMigration_MachinesNotReady_StillEnablesProxyProtocol(t *testing.T) {
	mockClient := &mocks.Client{}
	svc := newMigrationTestService(t, mockClient, func(hc *infrav2.HetznerCluster) {
		hc.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol = true
		hc.Spec.ControlPlaneLoadBalancer.HealthCheck = &infrav2.LoadBalancerHealthCheckSpec{
			Protocol: "http",
			Path:     ptr.To("/readyz"),
		}
	},
		// Annotated for the proxy-protocol migration, but not for the http-health-check one.
		controlPlaneMachineForProxy("cp-1", true),
		controlPlaneMachineForProxy("cp-2", true),
	)
	hcloudLB := &hcloud.LoadBalancer{
		Services: []hcloud.LoadBalancerService{
			{
				ListenPort:    testKubeAPIListenPort,
				Proxyprotocol: false,
				HealthCheck:   hcloud.LoadBalancerServiceHealthCheck{Protocol: hcloud.LoadBalancerServiceProtocolTCP, Port: testLBDestPort},
			},
		},
	}

	var updateCalls []hcloud.LoadBalancerUpdateServiceOpts
	mockClient.On("UpdateServiceOnLoadBalancer", mock.Anything, hcloudLB, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			updateCalls = append(updateCalls, args.Get(3).(hcloud.LoadBalancerUpdateServiceOpts))
		}).
		Return(nil)

	res, err := svc.reconcileServices(context.Background(), hcloudLB)
	require.NoError(t, err)
	require.Equal(t, 10*time.Second, res.RequeueAfter, "should requeue while the health-check migration waits, without blocking proxy protocol")
	require.True(t, svc.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ProxyProtocolEnabled, "proxy protocol should still be enabled in this reconcile")

	require.Len(t, updateCalls, 1, "only the proxy-protocol update should have been sent")
	require.NotNil(t, updateCalls[0].Proxyprotocol)
	require.True(t, *updateCalls[0].Proxyprotocol)
	require.Nil(t, updateCalls[0].HealthCheck, "the health check must not be applied while its migration gate is pending")
}
