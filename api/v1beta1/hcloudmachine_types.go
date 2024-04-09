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
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows ReconcileHCloudMachine to clean up HCloud
	// resources associated with HCloudMachine before removing it from the
	// apiserver.
	MachineFinalizer = "hcloudmachine.infrastructure.cluster.x-k8s.io"
)

// HCloudMachineSpec defines the desired state of HCloudMachine.
type HCloudMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Type is the HCloud Machine Type for this machine. It defines the desired server type of server in Hetzner's Cloud API. Example: cpx11.
	// +kubebuilder:validation:Enum=cpx11;cx21;cpx21;cx31;cpx31;cx41;cpx41;cx51;cpx51;ccx11;ccx12;ccx13;ccx21;ccx22;ccx23;ccx31;ccx32;ccx33;ccx41;ccx42;ccx43;ccx51;ccx52;ccx53;ccx62;ccx63;cax11;cax21;cax31;cax41
	Type HCloudMachineType `json:"type"`

	// ImageName is the reference to the Machine Image from which to create the machine instance.
	// It can reference an image uploaded to Hetzner API in two ways: either directly as the name of an image or as the label of an image.
	// +kubebuilder:validation:MinLength=1
	ImageName string `json:"imageName"`

	// SSHKeys define machine-specific SSH keys and override cluster-wide SSH keys.
	// +optional
	SSHKeys []SSHKey `json:"sshKeys,omitempty"`

	// PlacementGroupName defines the placement group of the machine in HCloud API that must reference an existing placement group.
	// +optional
	PlacementGroupName *string `json:"placementGroupName,omitempty"`

	// PublicNetwork specifies information for public networks. It defines the specs about
	// the primary IP address of the server. If both IPv4 and IPv6 are disabled, then the private network has to be enabled.
	// +optional
	PublicNetwork *PublicNetworkSpec `json:"publicNetwork,omitempty"`
}

// HCloudMachineStatus defines the observed state of HCloudMachine.
type HCloudMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses contain the server's associated addresses.
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// Region contains the name of the HCloud location the server is running.
	Region Region `json:"region,omitempty"`

	// InstanceState is the state of the server for this machine.
	// +optional
	InstanceState *hcloud.ServerStatus `json:"instanceState,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions define the current service state of the HCloudMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// HCloudMachine is the Schema for the hcloudmachines API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hcloudmachines,scope=Namespaced,categories=cluster-api,shortName=hcma
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this HCloudMachine belongs"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this HCloudMachine"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.instanceState",description="Phase of HCloudMachine"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of hcloudmachine"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +k8s:defaulter-gen=true
type HCloudMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudMachineSpec   `json:"spec,omitempty"`
	Status HCloudMachineStatus `json:"status,omitempty"`
}

// HCloudMachineSpec returns a DeepCopy.
func (r *HCloudMachine) HCloudMachineSpec() *HCloudMachineSpec {
	return r.Spec.DeepCopy()
}

// GetConditions returns the observations of the operational state of the HCloudMachine resource.
func (r *HCloudMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HCloudMachine to the predescribed clusterv1.Conditions.
func (r *HCloudMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// HCloudMachineList contains a list of HCloudMachine.
type HCloudMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &HCloudMachine{}, &HCloudMachineList{})
}
