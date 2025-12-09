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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// HCloudMachineFinalizer allows ReconcileHCloudMachine to clean up HCloud
	// resources associated with HCloudMachine before removing it from the
	// apiserver.
	HCloudMachineFinalizer = "infrastructure.cluster.x-k8s.io/hcloudmachine"
	// DeprecatedHCloudMachineFinalizer contains the old string.
	// The controller will automatically update to the new string.
	DeprecatedHCloudMachineFinalizer = "hcloudmachine.infrastructure.cluster.x-k8s.io"
)

// HCloudMachineSpec defines the desired state of HCloudMachine.
type HCloudMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Type is the HCloud Machine Type for this machine. It defines the desired server type of
	// server in Hetzner's Cloud API. You can use the hcloud CLI to get server names (`hcloud
	// server-type list`) or on https://www.hetzner.com/cloud
	//
	// The types follow this pattern: cxNV (shared, cheap), cpxNV (shared, performance), ccxNV
	// (dedicated), caxNV (ARM)
	//
	// N is a number, and V is the version of this machine type. Example: cpx32.
	//
	// The list of valid machine types gets changed by Hetzner from time to time. CAPH no longer
	// validates this string. It is up to you to use a valid type. Not all types are available in all
	// locations.
	Type HCloudMachineType `json:"type"`

	// ImageName is the reference to the Machine Image from which to create the machine instance.
	// It can reference an image uploaded to Hetzner API in two ways: either directly as the name of an image or as the label of an image.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Optional
	// +optional
	ImageName string `json:"imageName,omitempty"`

	// ImageURL gets used for installing custom node images. If that field is set, the controller
	// boots a new HCloud machine into rescue mode. Then the script provided by
	// --hcloud-image-url-command (which you need to provide to the controller binary) will be
	// copied into the rescue system and executed.
	//
	// The controller uses url.ParseRequestURI (Go function) to validate the URL.
	//
	// It is up to the script to provision the disk of the hcloud machine accordingly. The process
	// is considered successful if the last line in the output contains
	// IMAGE_URL_DONE. If the script terminates with a different last line, then
	// the process is considered to have failed.
	//
	// A Kubernetes event will be created in both (success, failure) cases containing the output
	// (stdout and stderr) of the script. If the script takes longer than 7 minutes, the
	// controller cancels the provisioning.
	//
	// Docs: https://syself.com/docs/caph/developers/image-url-command
	//
	// ImageURL is mutually exclusive to "ImageName".
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Optional
	// +optional
	ImageURL string `json:"imageURL,omitempty"`

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

	// SSHKeys specifies the ssh keys that were used for provisioning the server.
	SSHKeys []SSHKey `json:"sshKeys,omitempty"`

	// InstanceState is the state of the server for this machine.
	// +optional
	InstanceState *hcloud.ServerStatus `json:"instanceState,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions define the current service state of the HCloudMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// Deprecated groups status fields that were only used in the v1beta1 contract.
	// +optional
	Deprecated *HCloudMachineDeprecatedStatus `json:"deprecated,omitempty"`

	// BootState indicates the current state during provisioning.
	//
	// If Spec.ImageName is set the states will be:
	//   1. BootingToRealOS
	//   2. OperatingSystemRunning
	//
	// If Spec.ImageURL is set the states will be:
	//   1. Initializing
	//   2. EnablingRescue
	//   3. BootingToRescue
	//   4. RunningImageCommand
	//   5. BootingToRealOS
	//   6. OperatingSystemRunning

	// +optional
	BootState HCloudBootState `json:"bootState"`

	// BootStateSince is the timestamp of the last change to BootState. It is used to timeout
	// provisioning if a state takes too long.
	// +optional
	BootStateSince metav1.Time `json:"bootStateSince,omitzero"`

	// ExternalIDs contains temporary data during the provisioning process
	ExternalIDs HCloudMachineStatusExternalIDs `json:"externalIDs,omitempty"`
}

// HCloudMachineStatusExternalIDs holds temporary data during the provisioning process.
type HCloudMachineStatusExternalIDs struct {
	// ActionIDEnableRescueSystem is the hcloud API Action result of EnableRescueSystem.
	// +optional
	ActionIDEnableRescueSystem int64 `json:"actionIdEnableRescueSystem,omitzero"`
}

// HCloudMachineDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type HCloudMachineDeprecatedStatus struct {
	// v1beta1 groups all the status fields that were part of the legacy contract.
	// +optional
	V1Beta1 *HCloudMachineV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// HCloudMachineV1Beta1DeprecatedStatus groups the v1beta1 status fields that are
// preserved for compatibility.
type HCloudMachineV1Beta1DeprecatedStatus struct {
	// Conditions stores the legacy conditions that were observed in v1beta1.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// HCloudMachine is the Schema for the hcloudmachines API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hcloudmachines,scope=Namespaced,categories=cluster-api,shortName=hcma
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

// GetConditions returns the observations of the operational state of the HCloudMachine resource.
func (r *HCloudMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HCloudMachine to the predescribed clusterv1.Conditions.
func (r *HCloudMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

// SetBootState sets Status.BootStates and updates Status.BootStateSince.
func (r *HCloudMachine) SetBootState(bootState HCloudBootState) {
	if r.Status.BootState == bootState {
		return
	}
	r.Status.BootState = bootState
	r.Status.BootStateSince = metav1.Now()
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
