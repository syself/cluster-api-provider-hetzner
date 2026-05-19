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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
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
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in HCloudRemediation's status with the V1Beta2 version.
	// +optional
	V1Beta2 *HCloudRemediationV1Beta2Status `json:"v1beta2,omitempty"`
}

// HCloudRemediationV1Beta2Status groups all the fields that will be added or modified in HCloudRemediationStatus with the V1Beta2 version.
type HCloudRemediationV1Beta2Status struct {
	// conditions represents the observations of a HCloudRemediation's current state.
	// Known condition types are Ready, HCloudTokenAvailable and HCloudRateLimitExceeded.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
func (r *HCloudRemediation) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HCloudRemediation to the predescribed clusterv1beta1.Conditions.
func (r *HCloudRemediation) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the observations of the operational state of the HCloudRemediation resource.
func (r *HCloudRemediation) GetV1Beta2Conditions() []metav1.Condition {
	if r.Status.V1Beta2 == nil {
		return nil
	}
	return r.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets the underlying v1beta2 service state of the HCloudRemediation.
func (r *HCloudRemediation) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if r.Status.V1Beta2 == nil {
		r.Status.V1Beta2 = &HCloudRemediationV1Beta2Status{}
	}
	r.Status.V1Beta2.Conditions = conditions
}

//+kubebuilder:object:root=true

// HCloudRemediationList contains a list of HCloudRemediation.
type HCloudRemediationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudRemediation `json:"items"`
}

// HCloudRemediationV1Beta2SummaryOpts returns the v1beta2 summary options for HCloudRemediation.
// It is the single source of truth for which conditions contribute to the Ready summary,
// used both by HCloudRemediationScope.Close() and by early-exit error paths that bypass the scope.
//
// The order of conditions in ForConditionTypes defines the priority for the Ready summary:
// when multiple conditions are unhealthy, the summary lists all of them in priority order
// (highest-priority first). The ordering reflects operational importance:
//  1. HCloudTokenAvailable      - invalid credentials block everything.
//  2. HCloudRateLimitExceeded   - rate-limit issues (negative polarity).
//  3. RemediationSkipped        - remediation was skipped due to an irrecoverable
//     machine state; surfaced for visibility (negative polarity).
func HCloudRemediationV1Beta2SummaryOpts() []v1beta2conditions.SummaryOption {
	return []v1beta2conditions.SummaryOption{
		v1beta2conditions.ForConditionTypes{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			HCloudRemediationSkippedV1Beta2Condition,
		},
		v1beta2conditions.NegativePolarityConditionTypes{
			HCloudRateLimitExceededV1Beta2Condition,
			HCloudRemediationSkippedV1Beta2Condition,
		},
		v1beta2conditions.IgnoreTypesIfMissing{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			HCloudRemediationSkippedV1Beta2Condition,
		},
		v1beta2conditions.CustomMergeStrategy{
			MergeStrategy: v1beta2conditions.DefaultMergeStrategy(
				v1beta2conditions.GetPriorityFunc(v1beta2conditions.GetDefaultMergePriorityFunc(
					HCloudRateLimitExceededV1Beta2Condition,
					HCloudRemediationSkippedV1Beta2Condition,
				)),
				v1beta2conditions.ComputeReasonFunc(v1beta2conditions.GetDefaultComputeMergeReasonFunc(
					HCloudRemediationNotReadyV1Beta2Reason,
					HCloudRemediationReadyUnknownV1Beta2Reason,
					HCloudRemediationReadyV1Beta2Reason,
				)),
			),
		},
	}
}

func init() {
	objectTypes = append(objectTypes, &HCloudRemediation{}, &HCloudRemediationList{})
}
