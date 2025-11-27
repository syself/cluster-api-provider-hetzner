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
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// OwnerType is the type of object that owns the HCloudMachineTemplate.
	// +optional
	OwnerType string `json:"ownerType,omitempty"`
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
func (r *HCloudMachineTemplate) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HCloudMachine to the predescribed clusterv1.Conditions.
func (r *HCloudMachineTemplate) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
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
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the desired behavior of the machine.
	Spec HCloudMachineSpec `json:"spec"`
}

func init() {
	objectTypes = append(objectTypes, &HCloudMachineTemplate{}, &HCloudMachineTemplateList{})
}
