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

// HetznerBareMetalRemediationTemplateSpec defines the desired state of HetznerBareMetalRemediationTemplate.
type HetznerBareMetalRemediationTemplateSpec struct {
	// template defines the desired state of HetznerBareMetalRemediationTemplate.
	// +required
	Template HetznerBareMetalRemediationTemplateResource `json:"template,omitempty,omitzero"`
}

// HetznerBareMetalRemediationTemplateResource describes the data needed to create a HetznerBareMetalRemediation from a template.
// +kubebuilder:validation:MinProperties=1
type HetznerBareMetalRemediationTemplateResource struct {
	// spec is the specification of the desired behavior of the HetznerBareMetalRemediation. It is empty when
	// the template has no strategy. The HetznerBareMetalRemediation created from it then has an empty spec as well.
	// +optional
	Spec HetznerBareMetalRemediationSpec `json:"spec,omitempty,omitzero"`
}

// HetznerBareMetalRemediationTemplateStatus defines the observed state of HetznerBareMetalRemediationTemplate.
// +kubebuilder:validation:MinProperties=1
type HetznerBareMetalRemediationTemplateStatus struct {
	// status defines the observed state of HetznerBareMetalRemediation
	// +optional
	Status HetznerBareMetalRemediationStatus `json:"status,omitempty,omitzero"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hetznerbaremetalremediationtemplates,scope=Namespaced,categories=cluster-api,shortName=hbrt
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=".spec.template.spec.strategy.type",description="Type of the remediation strategy"
// +kubebuilder:printcolumn:name="Retry limit",type=integer,JSONPath=".spec.template.spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the host"
// +kubebuilder:printcolumn:name="Timeout",type=integer,JSONPath=".spec.template.spec.strategy.timeoutSeconds",description="Timeout for the remediation"

// HetznerBareMetalRemediationTemplate is the Schema for the hetznerbaremetalremediationtemplates API.
type HetznerBareMetalRemediationTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of HetznerBareMetalRemediationTemplate.
	// +optional
	Spec HetznerBareMetalRemediationTemplateSpec `json:"spec,omitempty,omitzero"`
	// status is the observed state of HetznerBareMetalRemediationTemplate.
	// +optional
	Status HetznerBareMetalRemediationTemplateStatus `json:"status,omitempty,omitzero"`
}

//+kubebuilder:object:root=true

// HetznerBareMetalRemediationTemplateList contains a list of HetznerBareMetalRemediationTemplate.
type HetznerBareMetalRemediationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerBareMetalRemediationTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &HetznerBareMetalRemediationTemplate{}, &HetznerBareMetalRemediationTemplateList{})
}
