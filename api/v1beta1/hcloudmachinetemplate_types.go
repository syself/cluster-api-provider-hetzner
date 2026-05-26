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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
)

// HCloudMachineTemplateSpec defines the desired state of HCloudMachineTemplate.
type HCloudMachineTemplateSpec struct {
	Template HCloudMachineTemplateResource `json:"template"`
}

// HCloudMachineTemplateStatus defines the observed state of HCloudMachineTemplate.
type HCloudMachineTemplateStatus struct {
	// Capacity defines the resource capacity for this machine.
	// This value is used for autoscaling from zero operations as defined in:
	// https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty"`

	// Conditions defines current service state of the HCloudMachineTemplate.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in HCloudMachineTemplate's status with the V1Beta2 version.
	// +optional
	V1Beta2 *HCloudMachineTemplateV1Beta2Status `json:"v1beta2,omitempty"`

	// OwnerType is the type of object that owns the HCloudMachineTemplate.
	// +optional
	OwnerType string `json:"ownerType,omitempty"`
}

// HCloudMachineTemplateV1Beta2Status groups all the fields that will be added or modified in HCloudMachineTemplateStatus with the V1Beta2 version.
type HCloudMachineTemplateV1Beta2Status struct {
	// conditions represents the observations of a HCloudMachineTemplate's current state.
	// Known condition types are Ready, Available, HCloudTokenAvailable and HCloudRateLimitExceeded.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hcloudmachinetemplates,scope=Namespaced,categories=cluster-api,shortName=capihcmt
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.template.spec.imageName",description="Image name"
// +kubebuilder:printcolumn:name="Placement group",type="string",JSONPath=".spec.template.spec.placementGroupName",description="Placement group name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.template.spec.type",description="Server type"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:storageversion
// +k8s:defaulter-gen=true

// HCloudMachineTemplate is the Schema for the hcloudmachinetemplates API.
type HCloudMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudMachineTemplateSpec   `json:"spec,omitempty"`
	Status HCloudMachineTemplateStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the HCloudMachine resource.
func (r *HCloudMachineTemplate) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HCloudMachine to the predescribed clusterv1beta1.Conditions.
func (r *HCloudMachineTemplate) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the observations of the operational state of the HCloudMachineTemplate resource.
func (r *HCloudMachineTemplate) GetV1Beta2Conditions() []metav1.Condition {
	if r.Status.V1Beta2 == nil {
		return nil
	}
	return r.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets the underlying v1beta2 service state of the HCloudMachineTemplate.
func (r *HCloudMachineTemplate) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if r.Status.V1Beta2 == nil {
		r.Status.V1Beta2 = &HCloudMachineTemplateV1Beta2Status{}
	}
	r.Status.V1Beta2.Conditions = conditions
}

// HCloudMachineTemplateV1Beta2SummaryOpts returns the v1beta2 summary options for an HCloudMachineTemplate.
// It is the single source of truth for which conditions contribute to the Ready summary.
//
// The order of conditions in ForConditionTypes defines the priority for the Ready summary:
// when multiple conditions are unhealthy, the summary lists all of them in priority
// order (highest-priority first).
//  1. HCloudTokenAvailable    - invalid credentials block everything.
//  2. HCloudRateLimitExceeded - rate-limit issues (negative polarity).
//  3. Available               - template availability and early-return visibility.
func HCloudMachineTemplateV1Beta2SummaryOpts() []v1beta2conditions.SummaryOption {
	return []v1beta2conditions.SummaryOption{
		v1beta2conditions.ForConditionTypes{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			HCloudMachineTemplateAvailableV1Beta2Condition,
		},
		// HCloudTokenAvailable and HCloudRateLimitExceeded are optional inputs:
		// token availability is only known after token validation, and rate-limit
		// state is only present while rate limited.
		v1beta2conditions.IgnoreTypesIfMissing{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
		},
		v1beta2conditions.CustomMergeStrategy{
			MergeStrategy: v1beta2conditions.DefaultMergeStrategy(
				v1beta2conditions.GetPriorityFunc(v1beta2conditions.GetDefaultMergePriorityFunc(
					// negative polarity condition.
					HCloudRateLimitExceededV1Beta2Condition,
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
	ObjectMeta clusterv1beta1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the desired behavior of the machine.
	Spec HCloudMachineSpec `json:"spec"`
}

func init() {
	objectTypes = append(objectTypes, &HCloudMachineTemplate{}, &HCloudMachineTemplateList{})
}
