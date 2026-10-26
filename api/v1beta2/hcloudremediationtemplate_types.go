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
)

// HCloudRemediationTemplateSpec defines the desired state of HCloudRemediationTemplate.
type HCloudRemediationTemplateSpec struct {
	// template defines the desired state of HCloudRemediationTemplate.
	// +required
	Template HCloudRemediationTemplateResource `json:"template,omitempty,omitzero"`
}

// HCloudRemediationTemplateResource describes the data needed to create a HCloudRemediation from a template.
// +kubebuilder:validation:MinProperties=1
type HCloudRemediationTemplateResource struct {
	// spec is the specification of the desired behavior of the HCloudRemediation. It is empty when
	// the template has no strategy. The HCloudRemediation created from it then has an empty spec as well.
	// +optional
	Spec HCloudRemediationSpec `json:"spec,omitempty,omitzero"`
}

// HCloudRemediationTemplateStatus defines the observed state of HCloudRemediationTemplate.
// +kubebuilder:validation:MinProperties=1
type HCloudRemediationTemplateStatus struct {
	// status defines the observed state of HCloudRemediation
	// +optional
	Status HCloudRemediationStatus `json:"status,omitempty,omitzero"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hcloudremediationtemplates,scope=Namespaced,categories=cluster-api,shortName=hcrt
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=".spec.template.spec.strategy.type",description="Type of the remediation strategy"
// +kubebuilder:printcolumn:name="Retry limit",type=integer,JSONPath=".spec.template.spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the node"
// +kubebuilder:printcolumn:name="Timeout",type=integer,JSONPath=".spec.template.spec.strategy.timeoutSeconds",description="Timeout for the remediation"

// HCloudRemediationTemplate is the Schema for the hcloudremediationtemplates API.
type HCloudRemediationTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of HCloudRemediationTemplate.
	// +optional
	Spec HCloudRemediationTemplateSpec `json:"spec,omitempty,omitzero"`
	// status is the observed state of HCloudRemediationTemplate.
	// +optional
	Status HCloudRemediationTemplateStatus `json:"status,omitempty,omitzero"`
}

//+kubebuilder:object:root=true

// HCloudRemediationTemplateList contains a list of HCloudRemediationTemplate.
type HCloudRemediationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudRemediationTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &HCloudRemediationTemplate{}, &HCloudRemediationTemplateList{})
}
