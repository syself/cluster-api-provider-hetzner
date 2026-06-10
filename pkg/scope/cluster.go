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

// Package scope defines cluster and machine scope as well as a repository for the Hetzner API.
package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// ClusterScopeParams defines the input parameters used to create a new scope.
type ClusterScopeParams struct {
	Client         client.Client
	APIReader      client.Reader
	Logger         logr.Logger
	HetznerSecret  *corev1.Secret
	HCloudClient   hcloudclient.Client
	Cluster        *clusterv1.Cluster
	HetznerCluster *infrav2.HetznerCluster
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.HetznerCluster == nil {
		return nil, errors.New("failed to generate new scope from nil HetznerCluster")
	}
	if params.HCloudClient == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudClient")
	}
	if params.APIReader == nil {
		return nil, errors.New("failed to generate new scope from nil APIReader")
	}

	emptyLogger := logr.Logger{}
	if params.Logger == emptyLogger {
		return nil, errors.New("failed to generate new scope from nil Logger")
	}

	helper, err := patch.NewHelper(params.HetznerCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ClusterScope{
		Logger:         params.Logger,
		Client:         params.Client,
		APIReader:      params.APIReader,
		Cluster:        params.Cluster,
		HetznerCluster: params.HetznerCluster,
		HCloudClient:   params.HCloudClient,
		patchHelper:    helper,
		hetznerSecret:  params.HetznerSecret,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	Client        client.Client
	APIReader     client.Reader
	patchHelper   *patch.Helper
	hetznerSecret *corev1.Secret

	HCloudClient hcloudclient.Client

	Cluster        *clusterv1.Cluster
	HetznerCluster *infrav2.HetznerCluster
}

// Name returns the HetznerCluster name.
func (s *ClusterScope) Name() string {
	return s.HetznerCluster.Name
}

// Namespace returns the namespace name.
func (s *ClusterScope) Namespace() string {
	return s.HetznerCluster.Namespace
}

// HetznerSecret returns the hetzner secret.
func (s *ClusterScope) HetznerSecret() *corev1.Secret {
	return s.hetznerSecret
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	// set summary for deprecated v1beta1 conditions.
	deprecatedv1beta1conditions.SetSummary(s.HetznerCluster)

	// set summary for conditions.
	readyCondition, err := conditions.NewSummaryCondition(
		s.HetznerCluster,
		clusterv1.ReadyCondition,
		infrav2.HetznerClusterSummaryOpts()...,
	)
	if err != nil {
		// Note, this could only happen if we hit edge cases in computing the summary, which should not
		// happen due to the fact that we are passing a non empty list of ForConditionTypes.
		s.Error(err, "Failed to set Ready condition")
		conditions.Set(s.HetznerCluster, metav1.Condition{
			Type:   clusterv1.ReadyCondition,
			Status: metav1.ConditionUnknown,
			Reason: clusterv1.InternalErrorReason,
		})

		patchErr := s.patchHelper.Patch(ctx, s.HetznerCluster, clusterpatchOpts()...)
		return errors.Join(err, patchErr)
	}

	conditions.Set(s.HetznerCluster, *readyCondition)

	return s.patchHelper.Patch(ctx, s.HetznerCluster, clusterpatchOpts()...)
}

// PatchObject persists the machine spec and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.HetznerCluster, clusterpatchOpts()...)
}

// GetSpecRegion returns a region.
func (s *ClusterScope) GetSpecRegion() []infrav2.Region {
	return s.HetznerCluster.Spec.ControlPlaneRegions
}

// SetStatusFailureDomain sets the region for the status.
func (s *ClusterScope) SetStatusFailureDomain(regions []infrav2.Region) {
	s.HetznerCluster.Status.FailureDomains = make([]clusterv1.FailureDomain, 0, len(regions))
	for _, region := range regions {
		s.HetznerCluster.Status.FailureDomains = append(s.HetznerCluster.Status.FailureDomains, clusterv1.FailureDomain{
			Name:         string(region),
			ControlPlane: ptr.To(true),
		})
	}
}

// ControlPlaneAPIEndpointPort returns the Port of the Kube-api server.
func (s *ClusterScope) ControlPlaneAPIEndpointPort() int32 {
	return int32(s.HetznerCluster.Spec.ControlPlaneLoadBalancer.Port) //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
}

// ClientConfig return a kubernetes client config for the workload cluster.
func (s *ClusterScope) ClientConfig(ctx context.Context) (clientcmd.ClientConfig, error) {
	return workloadClientConfigFromKubeconfigSecret(ctx, s.Logger, s.Client, s.APIReader, s.Cluster, s.HetznerCluster)
}

// ListMachines returns HCloudMachines.
func (s *ClusterScope) ListMachines(ctx context.Context) ([]*clusterv1.Machine, []*infrav1.HCloudMachine, error) {
	// get and index Machines by HCloudMachine name
	var machineListRaw clusterv1.MachineList
	machineByHCloudMachineName := make(map[string]*clusterv1.Machine)
	if err := s.Client.List(ctx, &machineListRaw, client.InNamespace(s.Namespace())); err != nil {
		return nil, nil, err
	}
	expectedGK := infrav1.GroupVersion.WithKind("HCloudMachine").GroupKind()
	for pos := range machineListRaw.Items {
		m := &machineListRaw.Items[pos]
		actualGK := schema.GroupKind{Group: m.Spec.InfrastructureRef.APIGroup, Kind: m.Spec.InfrastructureRef.Kind}
		if m.Spec.ClusterName != s.Cluster.Name ||
			actualGK.String() != expectedGK.String() {
			continue
		}
		machineByHCloudMachineName[m.Spec.InfrastructureRef.Name] = m
	}

	// match HCloudMachines to Machines
	var hcloudMachineListRaw infrav1.HCloudMachineList
	if err := s.Client.List(ctx, &hcloudMachineListRaw, client.InNamespace(s.Namespace())); err != nil {
		return nil, nil, err
	}

	machineList := make([]*clusterv1.Machine, 0, len(hcloudMachineListRaw.Items))
	hcloudMachineList := make([]*infrav1.HCloudMachine, 0, len(hcloudMachineListRaw.Items))

	for pos := range hcloudMachineListRaw.Items {
		hm := &hcloudMachineListRaw.Items[pos]
		m, ok := machineByHCloudMachineName[hm.Name]
		if !ok {
			continue
		}

		machineList = append(machineList, m)
		hcloudMachineList = append(hcloudMachineList, hm)
	}

	return machineList, hcloudMachineList, nil
}

// clusterpatchOpts returns the list of patch.Option for HetznerCluster.
func clusterpatchOpts() []patch.Option {
	return []patch.Option{
		// owned deprecated v1beta1 conditions.
		patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyV1Beta1Condition,
			infrav2.HCloudTokenAvailableV1Beta1Condition,
			infrav2.HetznerAPIReachableV1Beta1Condition,
			infrav2.NetworkReadyV1Beta1Condition,
			infrav2.LoadBalancerReadyV1Beta1Condition,
			infrav2.PlacementGroupsSyncedV1Beta1Condition,
			infrav2.ControlPlaneEndpointSetV1Beta1Condition,
			infrav2.TargetClusterReadyV1Beta1Condition,
			infrav2.TargetClusterSecretReadyV1Beta1Condition,
		}},
		// owned conditions.
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav2.HCloudTokenAvailableCondition,
			infrav2.HCloudRateLimitExceededCondition,
			infrav2.HetznerClusterDeletingCondition,
			infrav2.HetznerClusterNetworkReadyCondition,
			infrav2.HetznerClusterLoadBalancerReadyCondition,
			infrav2.HetznerClusterPlacementGroupsSyncedCondition,
			infrav2.HetznerClusterControlPlaneEndpointSetCondition,
			infrav2.HetznerClusterTargetClusterReadyCondition,
			infrav2.HetznerClusterTargetClusterSecretReadyCondition,
		}},
	}
}

// IsControlPlaneReady returns nil if the control plane is ready.
func IsControlPlaneReady(ctx context.Context, c clientcmd.ClientConfig) error {
	restConfig, err := c.ClientConfig()
	if err != nil {
		return err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	_, err = clientSet.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(ctx)
	return err
}
