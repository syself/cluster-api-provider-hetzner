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

package v1beta2

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
	// A nil value means the count has not been computed yet, which a zero value
	// cannot express.
	// +optional
	RetryCount *int32 `json:"retryCount,omitempty"`

	// LastRemediated identifies when the host was last remediated.
	// A zero value serializes as null and is treated as absent.
	// +optional
	LastRemediated metav1.Time `json:"lastRemediated,omitempty"`

	// conditions represents the observations of a HCloudRemediation's current state.
	// Known condition types are Ready, HCloudTokenAvailable and HCloudRateLimitExceeded.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed when support for v1beta1 is dropped.
	// +optional
	Deprecated *HCloudRemediationDeprecatedStatus `json:"deprecated,omitempty"`
}

// HCloudRemediationDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type HCloudRemediationDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 is dropped.
	// +optional
	V1Beta1 *HCloudRemediationV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// HCloudRemediationV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 is dropped.
type HCloudRemediationV1Beta1DeprecatedStatus struct {
	// conditions defines the current service state of the HCloudRemediation using the deprecated v1beta1 condition type.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hcloudremediations,scope=Namespaced,categories=cluster-api,shortName=hcr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="Phase of the remediation"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Ready status of the remediation"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of the remediation"
// +kubebuilder:printcolumn:name="Timeout",type=string,JSONPath=".spec.strategy.timeoutSeconds",description="Timeout for the remediation",priority=1
// +kubebuilder:printcolumn:name="Last Remediated",type=string,JSONPath=".status.lastRemediated",description="Timestamp of the last remediation attempt",priority=1
// +kubebuilder:printcolumn:name="Retry count",type=string,JSONPath=".status.retryCount",description="How many times remediation controller has tried to remediate the node",priority=1
// +kubebuilder:printcolumn:name="Retry limit",type=string,JSONPath=".spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the node",priority=1
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason",priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1

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

// GetConditions returns the deprecated v1beta1 conditions of the HCloudRemediation resource.
func (r *HCloudRemediation) GetConditions() clusterv1beta1.Conditions {
	if r.Status.Deprecated == nil || r.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return r.Status.Deprecated.V1Beta1.Conditions
}

// SetConditions sets the deprecated v1beta1 conditions of the HCloudRemediation resource.
func (r *HCloudRemediation) SetConditions(conditions clusterv1beta1.Conditions) {
	if r.Status.Deprecated == nil {
		r.Status.Deprecated = &HCloudRemediationDeprecatedStatus{}
	}
	if r.Status.Deprecated.V1Beta1 == nil {
		r.Status.Deprecated.V1Beta1 = &HCloudRemediationV1Beta1DeprecatedStatus{}
	}
	r.Status.Deprecated.V1Beta1.Conditions = conditions
}

// GetV1Beta2Conditions returns the observations of the operational state of the HCloudRemediation resource.
func (r *HCloudRemediation) GetV1Beta2Conditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetV1Beta2Conditions sets the underlying service state of the HCloudRemediation.
func (r *HCloudRemediation) SetV1Beta2Conditions(conditions []metav1.Condition) {
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
		// ForConditionTypes lists every condition that contributes to Ready, in
		// priority order. When multiple conditions are unhealthy the summary
		// surfaces them in this order, so the most important issue is listed first.
		v1beta2conditions.ForConditionTypes{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			HCloudRemediationSkippedV1Beta2Condition,
		},
		// IgnoreTypesIfMissing tells the summary not to treat the absence of a
		// listed condition as Unknown. Some reconcile paths exit before every
		// condition has been set (for example, before the token is checked or
		// before remediation has been evaluated), and we don't want those early
		// exits to flip Ready to Unknown.
		v1beta2conditions.IgnoreTypesIfMissing{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			HCloudRemediationSkippedV1Beta2Condition,
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
					HCloudRemediationSkippedV1Beta2Condition,
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

func init() {
	objectTypes = append(objectTypes, &HCloudRemediation{}, &HCloudRemediationList{})
}
