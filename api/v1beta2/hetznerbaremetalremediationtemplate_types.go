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
	Template HetznerBareMetalRemediationTemplateResource `json:"template"`
}

// HetznerBareMetalRemediationTemplateResource describes the data needed to create a HetznerBareMetalRemediation from a template.
type HetznerBareMetalRemediationTemplateResource struct {
	// Spec is the specification of the desired behavior of the HetznerBareMetalRemediation.
	Spec HetznerBareMetalRemediationSpec `json:"spec"`
}

// HetznerBareMetalRemediationTemplateStatus defines the observed state of HetznerBareMetalRemediationTemplate.
type HetznerBareMetalRemediationTemplateStatus struct {
	// HetznerBareMetalRemediationStatus defines the observed state of HetznerBareMetalRemediation
	Status HetznerBareMetalRemediationStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hetznerbaremetalremediationtemplates,scope=Namespaced,categories=cluster-api,shortName=hbrt
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=".spec.template.spec.strategy.type",description="Type of the remediation strategy"
// +kubebuilder:printcolumn:name="Retry limit",type=string,JSONPath=".spec.template.spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the host"
// +kubebuilder:printcolumn:name="Timeout",type=string,JSONPath=".spec.template.spec.strategy.timeout",description="Timeout for the remediation"

// HetznerBareMetalRemediationTemplate is the Schema for the hetznerbaremetalremediationtemplates API.
type HetznerBareMetalRemediationTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec HetznerBareMetalRemediationTemplateSpec `json:"spec,omitempty"`
	// +optional
	Status HetznerBareMetalRemediationTemplateStatus `json:"status,omitempty"`
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
