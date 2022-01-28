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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows Reconcilehetznerbaremetalmachine to clean up resources associated with hetznerbaremetalmachine before
	// removing it from the apiserver.
	BareMetalMachineFinalizer = "hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io"
)

// HetznerBareMetalMachineSpec defines the desired state of HetznerBareMetalMachine.
type HetznerBareMetalMachineSpec struct {
	// ProviderID will be the hetznerbaremetalmachine in ProviderID format
	// (hetzner://<server-id>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Image is the image to be provisioned.
	Image string `json:"image"`

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the hetznerbaremetalmachine's
	// namespace if not specified.
	// +optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// Type specifies the server type.
	// +optional
	Type string `json:"hostSelector,omitempty"`

	// When set to true we delete all data when we provision the node.
	// +optional
	// +kubebuilder:default:=false
	// +kubebuilder:validation:Optional
	CleanUpData bool `json:"cleanUpData,omitempty"`
}

// HetznerBareMetalMachineStatus defines the observed state of HetznerBareMetalMachine.
type HetznerBareMetalMachineStatus struct {

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the hetznerbaremetalmachine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the hetznerbaremetalmachine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of
	// hetznerbaremetalmachines can be added as events to the hetznerbaremetalmachine object
	// and/or logged in the controller's output.
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the hetznerbaremetalmachine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the hetznerbaremetalmachine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of
	// hetznerbaremetalmachines can be added as events to the hetznerbaremetalmachine object
	// and/or logged in the controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Addresses is a list of addresses assigned to the machine.
	// This field is copied from the infrastructure provider reference.
	// +optional
	Addresses capi.MachineAddresses `json:"addresses,omitempty"`

	// Region contains the server location.
	Region Region `json:"region,omitempty"`

	// Phase represents the current phase of machine actuation.
	// E.g. Pending, Running, Terminating, Failed etc.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Ready is the state of the hetznerbaremetalmachine.
	// TODO : Document the variable :
	// mhrivnak: " it would be good to document what this means, how to interpret
	// it, under what circumstances the value changes, etc."
	// +optional
	Ready bool `json:"ready"`

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the hetznerbaremetalmachine's
	// namespace if not specified.
	// +optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hetznerbaremetalmachines,scope=Namespaced,categories=cluster-api,shortName=hbm;hbmachine;hbmachines;hetznerbaremetalm;hetznerbaremetalmachine
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of hetznerbaremetalmachine"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="hetznerbaremetalmachine is Ready"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this M3Machine belongs"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="hetznerbaremetalmachine current phase"
// HetznerBareMetalMachine is the Schema for the hetznerbaremetalmachines API.
type HetznerBareMetalMachine struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec HetznerBareMetalMachineSpec `json:"spec,omitempty"`
	// +optional
	Status HetznerBareMetalMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HetznerBareMetalMachineList contains a list of HetznerBareMetalMachine.
type HetznerBareMetalMachineList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerBareMetalMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HetznerBareMetalMachine{}, &HetznerBareMetalMachineList{})
}
