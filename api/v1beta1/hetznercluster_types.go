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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
)

const (
	// HetznerClusterFinalizer allows ReconcileHetznerCluster to clean up HCloud
	// resources associated with HetznerCluster before removing it from the
	// apiserver.
	HetznerClusterFinalizer = "infrastructure.cluster.x-k8s.io/hetznercluster"

	// DeprecatedHetznerClusterFinalizer contains the old string.
	// The controller will automatically update to the new string.
	DeprecatedHetznerClusterFinalizer = "hetznercluster.infrastructure.cluster.x-k8s.io"

	// AllowEmptyControlPlaneAddressAnnotation allows HetznerCluster Webhook
	// to skip some validation steps for externally managed control planes.
	AllowEmptyControlPlaneAddressAnnotation = "capi.syself.com/allow-empty-control-plane-address"
	// SkipNamespaceAnnotation marks a namespace so CAPH controllers skip reconciliation.
	SkipNamespaceAnnotation = "capi.syself.com/skip-namespace"
	// ConstantBareMetalHostnameAnnotation makes hostnames of bare metal servers constant.
	ConstantBareMetalHostnameAnnotation = "capi.syself.com/constant-bare-metal-hostname"

	// UseHrobotProviderIDForBaremetalAnnotation on a HetznerCluster defines which ProviderID
	// format to use for baremetal nodes. If "true" "hrobot://" will be used. If not set or empty,
	// then the old format ("hcloud://bm-") gets used.
	UseHrobotProviderIDForBaremetalAnnotation = "capi.syself.com/use-hrobot-provider-id-for-baremetal"
)

// HetznerClusterSpec defines the desired state of HetznerCluster.
type HetznerClusterSpec struct {
	// HCloudNetwork defines details about the private Network for Hetzner Cloud. If left empty, no private Network is configured.
	// +optional
	HCloudNetwork HCloudNetworkSpec `json:"hcloudNetwork"`

	// ControlPlaneRegion consists of a list of HCloud Regions (fsn, nbg, hel). Because HCloud Networks
	// have a very low latency we could assume in some use cases that a region is behaving like a zone.
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone
	ControlPlaneRegions []Region `json:"controlPlaneRegions"`

	// SSHKeys are cluster wide. Valid values are a valid SSH key name.
	SSHKeys HetznerSSHKeys `json:"sshKeys"`
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint *clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// ControlPlaneLoadBalancer is an optional configuration for customizing control plane behavior.
	ControlPlaneLoadBalancer LoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`

	// +optional
	HCloudPlacementGroups []HCloudPlacementGroupSpec `json:"hcloudPlacementGroups,omitempty"`

	// HetznerSecretRef is a reference to a token to be used when reconciling this cluster.
	// This is generated in the security section under API TOKENS. Read & write is necessary.
	HetznerSecret HetznerSecretRef `json:"hetznerSecretRef"`

	// SkipCreatingHetznerSecretInWorkloadCluster indicates whether the Hetzner secret should be
	// created in the workload cluster. By default the secret gets created, so that the ccm (running
	// in the wl-cluster) can use that secret. If you prefer to not reveal the secret in the
	// wl-cluster, you can set this to value to false, so that the secret is not created. Be sure to
	// run the ccm outside of the wl-cluster in that case, e.g. in the management cluster.
	// +optional
	SkipCreatingHetznerSecretInWorkloadCluster bool `json:"skipCreatingHetznerSecretInWorkloadCluster,omitempty"`
}

// HetznerClusterStatus defines the observed state of HetznerCluster.
type HetznerClusterStatus struct {
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// +optional
	Network *NetworkStatus `json:"networkStatus,omitempty"`

	ControlPlaneLoadBalancer *LoadBalancerStatus `json:"controlPlaneLoadBalancer,omitempty"`
	// +optional
	HCloudPlacementGroups []HCloudPlacementGroupStatus  `json:"hcloudPlacementGroups,omitempty"`
	FailureDomains        clusterv1beta1.FailureDomains `json:"failureDomains,omitempty"`
	Conditions            clusterv1beta1.Conditions     `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in HetznerCluster's status with the V1Beta2 version.
	// +optional
	V1Beta2 *HetznerClusterV1Beta2Status `json:"v1beta2,omitempty"`
}

// HetznerClusterV1Beta2Status groups all the fields that will be added or modified in HetznerCluster with the V1Beta2 version.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HetznerClusterV1Beta2Status struct {
	// conditions represents the observations of a HetznerCluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=hetznerclusters,scope=Namespaced,categories=cluster-api,shortName=hccl
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this HetznerCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for Nodes"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint",description="API Endpoint",priority=1
// +kubebuilder:printcolumn:name="Regions",type="string",JSONPath=".spec.controlPlaneRegions",description="Control plane regions"
// +kubebuilder:printcolumn:name="Network enabled",type="boolean",JSONPath=".spec.hcloudNetwork.enabled",description="Indicates if private network is enabled."
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +k8s:defaulter-gen=true

// HetznerCluster is the Schema for the hetznercluster API.
type HetznerCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HetznerClusterSpec   `json:"spec,omitempty"`
	Status HetznerClusterStatus `json:"status,omitempty"`
}

// HetznerClusterList contains a list of HetznerCluster
// +kubebuilder:object:root=true
// +k8s:defaulter-gen=true
type HetznerClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerCluster `json:"items"`
}

// GetConditions returns the observations of the operational state of the HetznerCluster resource.
func (r *HetznerCluster) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HetznerCluster to the predescribed clusterv1beta1.Conditions.
func (r *HetznerCluster) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the set of v1beta2 conditions for the HetznerCluster object.
func (r *HetznerCluster) GetV1Beta2Conditions() []metav1.Condition {
	if r.Status.V1Beta2 == nil {
		return nil
	}
	return r.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets v1beta2 conditions for the HetznerCluster object.
func (r *HetznerCluster) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if r.Status.V1Beta2 == nil {
		r.Status.V1Beta2 = &HetznerClusterV1Beta2Status{}
	}
	r.Status.V1Beta2.Conditions = conditions
}

// ClusterV1Beta2SummaryOpts returns the v1beta2 summary options for a HetznerCluster.
// It is the single source of truth for which conditions contribute to the Ready summary,
// used both by ClusterScope.Close() and by early-exit error paths that bypass the scope.
func ClusterV1Beta2SummaryOpts() []v1beta2conditions.SummaryOption {
	return []v1beta2conditions.SummaryOption{
		// The summary is derived from all condition types listed in ForConditionTypes.
		// The order matters: it defines the priority in which conditions surface in the summary.
		v1beta2conditions.ForConditionTypes{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			clusterv1beta1.DeletingV1Beta2Condition,
			NetworkReadyV1Beta2Condition,
			LoadBalancerReadyV1Beta2Condition,
			PlacementGroupsSyncedV1Beta2Condition,
			ControlPlaneEndpointSetV1Beta2Condition,
			TargetClusterReadyV1Beta2Condition,
			TargetClusterSecretReadyV1Beta2Condition,
		},
		// IgnoreTypesIfMissing lists conditions that may legitimately not be present on the object.
		// If any of these are missing, the summary treats them as if they are healthy.
		v1beta2conditions.IgnoreTypesIfMissing{
			NetworkReadyV1Beta2Condition,
			LoadBalancerReadyV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			clusterv1beta1.DeletingV1Beta2Condition,
		},
		// CustomMergeStrategy is used only to override the merge reasons, so
		// the Ready summary uses CAPI's standard Ready reasons (Ready /
		// NotReady / ReadyUnknown) instead of the generic merge defaults
		// (IssuesReported / UnknownReported / InfoReported).
		//
		// Negative polarity is passed directly into GetDefaultMergePriorityFunc
		// here. When a CustomMergeStrategy is provided, NewSummaryCondition
		// skips the path that wires up the NegativePolarityConditionTypes
		// SummaryOption into the default strategy, so the negative-polarity
		// types must be specified explicitly inside the strategy.
		v1beta2conditions.CustomMergeStrategy{
			MergeStrategy: v1beta2conditions.DefaultMergeStrategy(
				v1beta2conditions.GetPriorityFunc(v1beta2conditions.GetDefaultMergePriorityFunc(
					// conditions with negative polarity
					HCloudRateLimitExceededV1Beta2Condition,
					clusterv1beta1.DeletingV1Beta2Condition,
				)),
				v1beta2conditions.ComputeReasonFunc(v1beta2conditions.GetDefaultComputeMergeReasonFunc(
					clusterv1beta1.NotReadyV1Beta2Reason,
					clusterv1beta1.ReadyUnknownV1Beta2Reason,
					clusterv1beta1.ReadyV1Beta2Reason,
				)),
			),
		},
	}
}

// ClusterTagKey generates the key for resources associated with a cluster.
func (r *HetznerCluster) ClusterTagKey() string {
	return NameHetznerProviderOwned + r.Name
}

func init() {
	objectTypes = append(objectTypes, &HetznerCluster{}, &HetznerClusterList{})
}
