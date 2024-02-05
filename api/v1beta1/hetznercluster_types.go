/*
Copyright 2021 The Kubernetes Authors.

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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows ReconcileHetznerCluster to clean up HCloud
	// resources associated with HetznerCluster before removing it from the
	// apiserver.
	ClusterFinalizer = "hetznercluster.infrastructure.cluster.x-k8s.io"
	// AllowEmptyControlPlaneAddressAnnotation allows HetznerCluster Webhook
	// to skip some validation steps for external managed controle planes.
	AllowEmptyControlPlaneAddressAnnotation = "capi.syself.com/allow-empty-control-plane-address"
)

// HetznerClusterSpec defines the desired state of HetznerCluster.
type HetznerClusterSpec struct {
	// HCloudNetworkSpec defines the Network for Hetzner Cloud. If left empty no private Network is configured.
	// +optional
	HCloudNetwork HCloudNetworkSpec `json:"hcloudNetwork"`

	// ControlPlaneRegion consists of a list of HCloud Regions (fsn, nbg, hel). Because HCloud Networks
	// have a very low latency we could assume in some use-cases that a region is behaving like a zone
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone
	ControlPlaneRegions []Region `json:"controlPlaneRegions"`

	// SSHKeys are cluster wide. Valid values are a valid SSH key name.
	SSHKeys HetznerSSHKeys `json:"sshKeys"`
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint *clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// ControlPlaneLoadBalancer is optional configuration for customizing control plane behavior. Naming convention is from upstream cluster-api project.
	ControlPlaneLoadBalancer LoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`

	// +optional
	HCloudPlacementGroups []HCloudPlacementGroupSpec `json:"hcloudPlacementGroups,omitempty"`

	// HetznerSecretRef is a reference to a token to be used when reconciling this cluster.
	// This is generated in the security section under API TOKENS. Read & write is necessary.
	HetznerSecret HetznerSecretRef `json:"hetznerSecretRef"`
}

// HetznerClusterStatus defines the observed state of HetznerCluster.
type HetznerClusterStatus struct {
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// +optional
	Network *NetworkStatus `json:"networkStatus,omitempty"`

	ControlPlaneLoadBalancer *LoadBalancerStatus `json:"controlPlaneLoadBalancer,omitempty"`
	// +optional
	HCloudPlacementGroups []HCloudPlacementGroupStatus `json:"hcloudPlacementGroups,omitempty"`
	FailureDomains        clusterv1.FailureDomains     `json:"failureDomains,omitempty"`
	Conditions            clusterv1.Conditions         `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=hetznerclusters,scope=Namespaced,categories=cluster-api,shortName=capihc
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
func (r *HetznerCluster) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HetznerCluster to the predescribed clusterv1.Conditions.
func (r *HetznerCluster) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

// ClusterTagKey generates the key for resources associated with a cluster.
func (r *HetznerCluster) ClusterTagKey() string {
	return NameHetznerProviderOwned + r.Name
}

func init() {
	objectTypes = append(objectTypes, &HetznerCluster{}, &HetznerClusterList{})
}
