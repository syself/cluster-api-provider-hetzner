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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
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

	// ProxyProtocolForControlPlaneLoadBalancerAnnotation must be present with value "true" on ALL
	// control-plane nodes in the workload cluster before CAPH enables proxy protocol on the control
	// plane load balancer. The annotation is set by an external service (e.g. a node-configuration
	// daemonset) once the node is ready to receive PROXY-protocol connections. CAPH reads this
	// annotation — it never writes it.
	ProxyProtocolForControlPlaneLoadBalancerAnnotation = "capi.syself.com/proxy-protocol-for-controlplane-loadbalancer"
)

// HetznerClusterSpec defines the desired state of HetznerCluster.
type HetznerClusterSpec struct {
	// HCloudNetwork defines details about the private Network for Hetzner Cloud. If left empty, no private Network is configured.
	// +optional
	HCloudNetwork HCloudNetworkSpec `json:"hcloudNetwork"`

	// ControlPlaneRegion consists of a list of HCloud Regions (fsn, nbg, hel). Because HCloud Networks
	// have a very low latency we could assume in some use cases that a region is behaving like a zone.
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone
	// +listType=set
	ControlPlaneRegions []Region `json:"controlPlaneRegions"`

	// SSHKeys are cluster wide. Valid values are a valid SSH key name.
	SSHKeys HetznerSSHKeys `json:"sshKeys"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`

	// ControlPlaneLoadBalancer is an optional configuration for customizing control plane behavior.
	ControlPlaneLoadBalancer LoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=name
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

	// EnableProxyProtocolForControlPlaneLoadBalancer enables proxy protocol on the kube-apiserver
	// load balancer service. When true, CAPH checks whether all control-plane nodes in the
	// workload cluster carry the annotation
	// capi.syself.com/proxy-protocol-for-controlplane-loadbalancer: "true" (set by an external
	// service). Proxy protocol is activated on the LB only once every CP node has that annotation,
	// ensuring the backend is prepared before the LB starts sending PROXY-protocol headers.
	// +optional
	EnableProxyProtocolForControlPlaneLoadBalancer bool `json:"enableProxyProtocolForControlPlaneLoadBalancer,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
// +kubebuilder:validation:MinProperties=1
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Host string `json:"host,omitempty"`

	// Port is the port on which the API server is serving.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// HetznerClusterStatus defines the observed state of HetznerCluster.
type HetznerClusterStatus struct {
	// conditions represents the observations of a HetznerCluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// initialization provides observations of the HetznerCluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization HetznerClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	// +optional
	Network *NetworkStatus `json:"networkStatus,omitempty"`

	ControlPlaneLoadBalancer *LoadBalancerStatus `json:"controlPlaneLoadBalancer,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=name
	HCloudPlacementGroups []HCloudPlacementGroupStatus `json:"hcloudPlacementGroups,omitempty"`

	// failureDomains is a slice of failure domain objects synced from the infrastructure provider.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	FailureDomains []clusterv1.FailureDomain `json:"failureDomains,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	Deprecated *HetznerClusterDeprecatedStatus `json:"deprecated,omitempty"`
}

// HetznerClusterInitializationStatus provides observations of the HetznerCluster initialization process.
// +kubebuilder:validation:MinProperties=1
type HetznerClusterInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the HetznerCluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Cluster provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// HetznerClusterDeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HetznerClusterDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *HetznerClusterV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// HetznerClusterV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HetznerClusterV1Beta1DeprecatedStatus struct {
	// conditions defines current service state of the HetznerCluster.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 is dropped.
	Conditions []clusterv1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=hetznerclusters,scope=Namespaced,categories=cluster-api,shortName=hccl
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this HetznerCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Cluster infrastructure is ready for Nodes"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HetznerCluster"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint",description="API Endpoint",priority=1
// +kubebuilder:printcolumn:name="Regions",type="string",JSONPath=".spec.controlPlaneRegions",description="Control plane regions",priority=1
// +kubebuilder:printcolumn:name="Network enabled",type="boolean",JSONPath=".spec.hcloudNetwork.enabled",description="Indicates if private network is enabled.",priority=1
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason",priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1
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

// GetConditions returns the set of conditions for the HetznerCluster object.
func (r *HetznerCluster) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets the conditions for the HetznerCluster object.
func (r *HetznerCluster) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

// GetV1Beta1Conditions returns the deprecated v1beta1 conditions of the HetznerCluster object.
func (r *HetznerCluster) GetV1Beta1Conditions() clusterv1.Conditions {
	if r.Status.Deprecated == nil || r.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return r.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the deprecated v1beta1 conditions on the HetznerCluster object.
func (r *HetznerCluster) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if r.Status.Deprecated == nil {
		r.Status.Deprecated = &HetznerClusterDeprecatedStatus{}
	}
	if r.Status.Deprecated.V1Beta1 == nil {
		r.Status.Deprecated.V1Beta1 = &HetznerClusterV1Beta1DeprecatedStatus{}
	}
	r.Status.Deprecated.V1Beta1.Conditions = conditions
}

// HetznerClusterSummaryOpts returns the summary options for a HetznerCluster.
// It is the single source of truth for which conditions contribute to the Ready summary,
// used both by ClusterScope.Close() and by early-exit error paths that bypass the scope.
func HetznerClusterSummaryOpts() []conditions.SummaryOption {
	return []conditions.SummaryOption{
		// The summary is derived from all condition types listed in ForConditionTypes.
		// The order matters: it defines the priority in which conditions surface in the summary.
		conditions.ForConditionTypes{
			HCloudTokenAvailableCondition,
			HCloudRateLimitExceededCondition,
			HetznerClusterDeletingCondition,
			HetznerClusterNetworkReadyCondition,
			HetznerClusterLoadBalancerReadyCondition,
			HetznerClusterPlacementGroupsSyncedCondition,
			HetznerClusterControlPlaneEndpointSetCondition,
			HetznerClusterTargetClusterReadyCondition,
			HetznerClusterTargetClusterSecretReadyCondition,
		},
		// IgnoreTypesIfMissing lists conditions that may legitimately not be present on the object.
		// If any of these are missing, the summary treats them as if they are healthy.
		conditions.IgnoreTypesIfMissing{
			HetznerClusterNetworkReadyCondition,
			HetznerClusterLoadBalancerReadyCondition,
			HetznerClusterDeletingCondition,
			HCloudRateLimitExceededCondition,
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
		conditions.CustomMergeStrategy{
			MergeStrategy: conditions.DefaultMergeStrategy(
				conditions.GetPriorityFunc(conditions.GetDefaultMergePriorityFunc(
					// conditions with negative polarity
					HCloudRateLimitExceededCondition,
					HetznerClusterDeletingCondition,
				)),
				conditions.ComputeReasonFunc(conditions.GetDefaultComputeMergeReasonFunc(
					clusterv1.NotReadyReason,
					clusterv1.ReadyUnknownReason,
					clusterv1.ReadyReason,
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
