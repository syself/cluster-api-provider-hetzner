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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// HCloudRemediationFinalizer allows HCloudRemediationReconciler to clean up resources associated with HCloudRemediation before
	// removing it from the apiserver.
	HCloudRemediationFinalizer = "hcloudremediation.infrastructure.cluster.x-k8s.io"
)

// HCloudRemediationSpec defines the desired state of HCloudRemediation.
type HCloudRemediationSpec struct {
	// Strategy field defines remediation strategy.
	Strategy *RemediationStrategy `json:"strategy,omitempty"`
}

// HCloudRemediationStatus defines the observed state of HCloudRemediation.
type HCloudRemediationStatus struct {
	// Phase represents the current phase of machine remediation.
	// E.g. Pending, Running, Done etc.
	// +optional
	Phase string `json:"phase,omitempty"`

	// RetryCount can be used as a counter during the remediation.
	// Field can hold number of reboots etc.
	// +optional
	RetryCount int `json:"retryCount,omitempty"`

	// LastRemediated identifies when the host was last remediated
	// +optional
	LastRemediated *metav1.Time `json:"lastRemediated,omitempty"`

	// Conditions defines current service state of the HCloudRemediation.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hcloudremediations,scope=Namespaced,categories=cluster-api,shortName=hcr
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Timeout",type=string,JSONPath=".spec.strategy.timeout",description="Timeout for the remediation"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="Phase of the remediation"
// +kubebuilder:printcolumn:name="Last Remediated",type=string,JSONPath=".status.lastRemediated",description="Timestamp of the last remediation attempt"
// +kubebuilder:printcolumn:name="Retry count",type=string,JSONPath=".status.retryCount",description="How many times remediation controller has tried to remediate the node"
// +kubebuilder:printcolumn:name="Retry limit",type=string,JSONPath=".spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the node"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"

// HCloudRemediation is the Schema for the hcloudremediations API.
type HCloudRemediation struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec HCloudRemediationSpec `json:"spec,omitempty"`
	// +optional
	Status HCloudRemediationStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the HCloudRemediation resource.
func (r *HCloudRemediation) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HCloudRemediation to the predescribed clusterv1.Conditions.
func (r *HCloudRemediation) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// HCloudRemediationList contains a list of HCloudRemediation.
type HCloudRemediationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudRemediation `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &HCloudRemediation{}, &HCloudRemediationList{})
}
