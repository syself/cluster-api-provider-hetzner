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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
)

// HCloudMachineTemplateSpec defines the desired state of HCloudMachineTemplate.
type HCloudMachineTemplateSpec struct {
	Template HCloudMachineTemplateResource `json:"template"`
}

// HCloudMachineTemplateStatus defines the observed state of HCloudMachineTemplate.
type HCloudMachineTemplateStatus struct {
	// conditions represents the observations of an HCloudMachineTemplate's current state.
	// Known condition types are Ready, Available, HCloudTokenAvailable and HCloudRateLimitExceeded.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Capacity defines the resource capacity for this machine.
	// This value is used for autoscaling from zero operations as defined in:
	// https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty"`

	// OwnerType is the type of object that owns the HCloudMachineTemplate.
	// +optional
	OwnerType string `json:"ownerType,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	Deprecated *HCloudMachineTemplateDeprecatedStatus `json:"deprecated,omitempty"`
}

// HCloudMachineTemplateDeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HCloudMachineTemplateDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *HCloudMachineTemplateV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// HCloudMachineTemplateV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HCloudMachineTemplateV1Beta1DeprecatedStatus struct {
	// conditions defines current service state of the HCloudMachineTemplate.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 is dropped.
	Conditions []clusterv1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hcloudmachinetemplates,scope=Namespaced,categories=cluster-api,shortName=capihcmt
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.template.spec.imageName",description="Image name"
// +kubebuilder:printcolumn:name="Placement group",type="string",JSONPath=".spec.template.spec.placementGroupName",description="Placement group name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.template.spec.type",description="Server type"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +k8s:defaulter-gen=true

// HCloudMachineTemplate is the Schema for the hcloudmachinetemplates API.
type HCloudMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudMachineTemplateSpec   `json:"spec,omitempty"`
	Status HCloudMachineTemplateStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for the HCloudMachineTemplate object.
func (r *HCloudMachineTemplate) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets the conditions for the HCloudMachineTemplate object.
func (r *HCloudMachineTemplate) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

// GetV1Beta1Conditions returns the deprecated v1beta1 conditions of the HCloudMachineTemplate object.
func (r *HCloudMachineTemplate) GetV1Beta1Conditions() clusterv1.Conditions {
	if r.Status.Deprecated == nil || r.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return r.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the deprecated v1beta1 conditions on the HCloudMachineTemplate object.
func (r *HCloudMachineTemplate) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if r.Status.Deprecated == nil {
		r.Status.Deprecated = &HCloudMachineTemplateDeprecatedStatus{}
	}
	if r.Status.Deprecated.V1Beta1 == nil {
		r.Status.Deprecated.V1Beta1 = &HCloudMachineTemplateV1Beta1DeprecatedStatus{}
	}
	r.Status.Deprecated.V1Beta1.Conditions = conditions
}

// HCloudMachineTemplateSummaryOpts returns the summary options for an HCloudMachineTemplate.
// It is the single source of truth for which conditions contribute to the Ready summary, used both
// by SetHCloudMachineTemplateSummaryCondition and by early-exit error paths that bypass it.
//
// The order of conditions in ForConditionTypes defines the priority for the Ready summary:
// when multiple conditions are unhealthy, the summary lists all of them in priority
// order (highest-priority first).
//  1. HCloudTokenAvailable    - invalid credentials block everything.
//  2. HCloudRateLimitExceeded - rate-limit issues (negative polarity).
//  3. Available               - template availability and early-return visibility.
func HCloudMachineTemplateSummaryOpts() []conditions.SummaryOption {
	return []conditions.SummaryOption{
		// ForConditionTypes lists every condition that contributes to Ready, in priority order.
		conditions.ForConditionTypes{
			HCloudTokenAvailableCondition,
			HCloudRateLimitExceededCondition,
			HCloudMachineTemplateAvailableCondition,
		},
		// IgnoreTypesIfMissing tells the summary not to treat a listed condition as Unknown when it
		// is missing. A template owned by a ClusterClass returns before the HCloud token is read, so
		// it has no HCloudTokenAvailable condition. HCloudRateLimitExceeded exists only while the
		// template is rate limited.
		conditions.IgnoreTypesIfMissing{
			HCloudTokenAvailableCondition,
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

//+kubebuilder:object:root=true

// HCloudMachineTemplateList contains a list of HCloudMachineTemplate.
type HCloudMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudMachineTemplate `json:"items"`
}

// HCloudMachineTemplateResource describes the data needed to create am HCloudMachine from a template.
type HCloudMachineTemplateResource struct {
	// Standard object's metadata.
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// Spec is the specification of the desired behavior of the machine.
	Spec HCloudMachineSpec `json:"spec"`
}

func init() {
	objectTypes = append(objectTypes, &HCloudMachineTemplate{}, &HCloudMachineTemplateList{})
}
