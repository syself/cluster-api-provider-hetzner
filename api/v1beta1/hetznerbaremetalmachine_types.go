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
	"k8s.io/apimachinery/pkg/selection"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// BareMetalMachineFinalizer allows Reconcilehetznerbaremetalmachine to clean up resources associated with hetznerbaremetalmachine before
	// removing it from the apiserver.
	BareMetalMachineFinalizer = "hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io"
)

// HostSelector specifies matching criteria for labels on BareMetalHosts.
// This is used to limit the set of BareMetalHost objects considered for
// claiming for a Machine.
type HostSelector struct {
	// Key/value pairs of labels that must exist on a chosen BareMetalHost
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// Label match expressions that must be true on a chosen BareMetalHost
	// +optional
	MatchExpressions []HostSelectorRequirement `json:"matchExpressions,omitempty"`
}

type HostSelectorRequirement struct {
	Key      string             `json:"key"`
	Operator selection.Operator `json:"operator"`
	Values   []string           `json:"values"`
}

// HetznerBareMetalMachineSpec defines the desired state of HetznerBareMetalMachine.
type HetznerBareMetalMachineSpec struct {
	// ProviderID will be the hetznerbaremetalmachine in ProviderID format
	// (hetzner://<server-id>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Image is the image to be provisioned.
	Image string `json:"image"`

	// Partitions defines the additional Partitions to be created.
	Partitions []Partition `json:"partitions"`

	// LVMDefinitions defines the logical volume definitions to be created.
	// +optional
	LVMDefinitions []LVMDefinition `json:"logicalVolumeDefinitions,omitempty"`

	// BTRFSDefinitions defines the btrfs subvolume definitions to be created.
	// +optional
	BTRFSDefinitions []BTRFSDefinition `json:"btrfsDefinitions,omitempty"`

	// HostSelector specifies matching criteria for labels on HetznerBareMetalHosts.
	// This is used to limit the set of HetznerBareMetalHost objects considered for
	// claiming for a HetznerBareMetalMachine.
	// +optional
	HostSelector HostSelector `json:"hostSelector,omitempty"`

	// Type specifies the server type.
	// +optional
	Type string `json:"hostSelector,omitempty"`
}

// Partitions defines the additional Partitions to be created.
type Partition struct {

	// Mount defines the mount path for this filesystem.
	// or keyword 'lvm' to use this PART as volume group (VG) for LVM
	// identifier 'btrfs.X' to use this PART as volume for
	// btrfs subvolumes. X can be replaced with a unique
	// alphanumeric keyword. NOTE: no support btrfs multi-device volumes
	Mount string `json:"mount,omitempty"`

	// FileSystem can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap
	// or name of the LVM volume group (VG), if this PART is a VG.
	FileSystem string `json:"fileSystem,omitempty"`

	// Size can use the keyword 'all' to assign all the remaining space of the drive to the last partition.
	// can use M/G/T for unit specification in MiB/GiB/TiB
	Size string `json:"size,omitempty"`
}

// BTRFSDefinitions defines the btrfs subvolume definitions to be created.
type BTRFSDefinition struct {
	// Volume defines the btrfs volume name.
	Volume string `json:"volume,omitempty"`

	// SubVolume defines the subvolume name.
	SubVolume string `json:"subvolume,omitempty"`

	// Mount defines the mountpath.
	Mount string `json:"mount,omitempty"`
}

// LVMDefinitions defines the logical volume definitions to be created.
type LVMDefinition struct {
	// VG defines the vg name.
	VG string `json:"vg,omitempty"`

	// Name defines the volume name.
	Name string `json:"name,omitempty"`

	// Mount defines the mountpath.
	Mount string `json:"mount,omitempty"`

	// FileSystem defines the filesystem for this logical volume.
	FileSystem string `json:"filesystem,omitempty"`

	// Size defines the size in M/G/T or MiB/GiB/TiB.
	Size string `json:"size,omitempty"`
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
	// +optional
	Ready bool `json:"ready"`

	// Conditions defines current service state of the HetznerBareMetalMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
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

// GetConditions returns the observations of the operational state of the HetznerBareMetalMachine resource.
func (r *HetznerBareMetalMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HetznerBareMetalMachine to the predescribed clusterv1.Conditions.
func (r *HetznerBareMetalMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
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
