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
)

// HCloudRemediationTemplateSpec defines the desired state of HCloudRemediationTemplate.
type HCloudRemediationTemplateSpec struct {
	Template HCloudRemediationTemplateResource `json:"template"`
}

// HCloudRemediationTemplateResource describes the data needed to create a HCloudRemediation from a template.
type HCloudRemediationTemplateResource struct {
	// Spec is the specification of the desired behavior of the HCloudRemediation.
	Spec HCloudRemediationSpec `json:"spec"`
}

// HCloudRemediationTemplateStatus defines the observed state of HCloudRemediationTemplate.
type HCloudRemediationTemplateStatus struct {
	// HCloudRemediationStatus defines the observed state of HCloudRemediation
	Status HCloudRemediationStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hcloudremediationtemplates,scope=Namespaced,categories=cluster-api,shortName=hcrt
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=".spec.template.spec.strategy.type",description="Type of the remediation strategy"
// +kubebuilder:printcolumn:name="Retry limit",type=string,JSONPath=".spec.template.spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the node"
// +kubebuilder:printcolumn:name="Timeout",type=string,JSONPath=".spec.template.spec.strategy.timeout",description="Timeout for the remediation"

// HCloudRemediationTemplate is the Schema for the hcloudremediationtemplates API.
type HCloudRemediationTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec HCloudRemediationTemplateSpec `json:"spec,omitempty"`
	// +optional
	Status HCloudRemediationTemplateStatus `json:"status,omitempty"`
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
