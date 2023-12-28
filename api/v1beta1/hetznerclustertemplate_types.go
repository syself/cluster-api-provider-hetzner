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

// HetznerClusterTemplateSpec defines the desired state of HetznerClusterTemplate.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="HetznerClusterTemplate.Spec is immutable."
type HetznerClusterTemplateSpec struct {
	Template HetznerClusterTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=hetznerclustertemplates,scope=Namespaced,categories=cluster-api,shortName=capihct
// +k8s:defaulter-gen=true

// HetznerClusterTemplate is the Schema for the hetznerclustertemplates API.
type HetznerClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HetznerClusterTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// HetznerClusterTemplateList contains a list of HetznerClusterTemplate.
type HetznerClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerClusterTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &HetznerClusterTemplate{}, &HetznerClusterTemplateList{})
}

// HetznerClusterTemplateResource contains spec for HetznerClusterSpec.
type HetznerClusterTemplateResource struct {
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`
	Spec       HetznerClusterSpec   `json:"spec"`
}
