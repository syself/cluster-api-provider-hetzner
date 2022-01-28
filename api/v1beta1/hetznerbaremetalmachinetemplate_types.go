/*
Copyright 2021.

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

// HetznerBareMetalMachineTemplateSpec defines the desired state of HetznerBareMetalMachineTemplate
type HetznerBareMetalMachineTemplateSpec struct {
	Template HetznerBareMetalMachineTemplateResource `json:"template"`

	// When set to True, HetznerBaremetalMachine controller will
	// pick the same pool of BMHs' that were released during the upgrade operation.
	// +kubebuilder:default=false
	// +optional
	NodeReuse bool `json:"nodeReuse"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HetznerBareMetalMachineTemplate"
// +kubebuilder:resource:path=hetznerbaremetalmachinetemplates,scope=Namespaced,categories=cluster-api,shortName=hbmt;hbmmtemplate;hetznerbaremetalmachinetemplates;hetznerbaremetalmachinetemplate
// +kubebuilder:storageversion
// HetznerBareMetalMachineTemplate is the Schema for the hetznerbaremetalmachinetemplates API
type HetznerBareMetalMachineTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec HetznerBareMetalMachineTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// HetznerBareMetalMachineTemplateList contains a list of HetznerBareMetalMachineTemplate
type HetznerBareMetalMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerBareMetalMachineTemplate `json:"items"`
}

// HetznerBareMetalMachineTemplateResource describes the data needed to create a HetznerBareMetalMachine from a template
type HetznerBareMetalMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec HetznerBareMetalMachineSpec `json:"spec"`
}

func init() {
	SchemeBuilder.Register(&HetznerBareMetalMachineTemplate{}, &HetznerBareMetalMachineTemplateList{})
}
