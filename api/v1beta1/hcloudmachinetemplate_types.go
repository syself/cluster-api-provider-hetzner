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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// HCloudMachineTemplateSpec defines the desired state of HCloudMachineTemplate.
type HCloudMachineTemplateSpec struct {
	Template HCloudMachineTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hcloudmachinetemplates,scope=Namespaced,categories=cluster-api,shortName=capihcmt
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.template.spec.imageName",description="Image name"
// +kubebuilder:printcolumn:name="Placement group",type="string",JSONPath=".spec.template.spec.placementGroupName",description="Placement group name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.template.spec.type",description="Server type"
// +kubebuilder:storageversion
// +k8s:defaulter-gen=true

// HCloudMachineTemplate is the Schema for the hcloudmachinetemplates API.
type HCloudMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HCloudMachineTemplateSpec `json:"spec,omitempty"`
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
	SchemeBuilder.Register(&HCloudMachineTemplate{}, &HCloudMachineTemplateList{})
}
