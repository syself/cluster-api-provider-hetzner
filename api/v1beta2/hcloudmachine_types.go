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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// HCloudMachineFinalizer allows ReconcileHCloudMachine to clean up HCloud
	// resources associated with HCloudMachine before removing it from the
	// apiserver.
	HCloudMachineFinalizer = "infrastructure.cluster.x-k8s.io/hcloudmachine"
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
	// boots a new HCloud machine into rescue mode. Then the command referenced by
	// ImageURLCommand will be copied into the rescue system and executed.
	//
	// The controller uses url.ParseRequestURI (Go function) to validate the URL.
	//
	// It is up to the script to provision the disk of the hcloud machine accordingly. The process
	// is considered successful if the last line in the output contains
	// IMAGE_URL_DONE. If the script terminates with a different last line, then
	// the process is considered to have failed.
	//
	// A Kubernetes event will be created in both (success, failure) cases containing the output
	// (stdout and stderr) of the script. If the script takes longer than 20 minutes, the
	// controller cancels the provisioning.
	//
	// Docs: https://syself.com/docs/caph/developers/image-url-command
	//
	// ImageURL is mutually exclusive to "ImageName".
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Optional
	// +optional
	ImageURL string `json:"imageURL,omitempty"`

	// ImageURLCommand is the basename of a command file below /shared on the controller pod which
	// provisions a machine from ImageURL. CAPH copies that command into the rescue system and
	// executes it there.
	//
	// Docs: https://syself.com/docs/caph/developers/image-url-command
	//
	// ImageURLCommand must be set if ImageURL is set. ImageURLCommand must be empty if ImageURL is
	// empty.
	// +kubebuilder:validation:Optional
	// +optional
	ImageURLCommand string `json:"imageURLCommand,omitempty"`

	// SSHKeys define machine-specific SSH keys and override cluster-wide SSH keys.
	// +optional
	// +listType=map
	// +listMapKey=name
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
	// conditions represents the observations of an HCloudMachine's current state.
	// Known condition types are Ready, HCloudTokenAvailable, HCloudRateLimitExceeded, ServerCreated, ServerProvisioned and ServerAvailable.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// initialization provides observations of the HCloudMachine initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization HCloudMachineInitializationStatus `json:"initialization,omitempty,omitzero"`

	// Addresses contain the server's associated addresses.
	// +optional
	// +listType=atomic
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// Region contains the name of the HCloud location the server is running.
	// +optional
	Region Region `json:"region,omitempty"`

	// SSHKeys specifies the ssh keys that were used for provisioning the server.
	// +optional
	// +listType=map
	// +listMapKey=name
	SSHKeys []SSHKey `json:"sshKeys,omitempty"`

	// InstanceState is the state of the server for this machine.
	// +optional
	InstanceState InstanceState `json:"instanceState,omitempty"`

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
	// +optional
	ExternalIDs HCloudMachineStatusExternalIDs `json:"externalIDs,omitempty"`

	// LastRemediatedAt records when the most recent successful remediation completed.
	// Used to prevent reboot loops across successive MHC incidents.
	// +optional
	LastRemediatedAt metav1.Time `json:"lastRemediatedAt,omitempty,omitzero"`

	// deprecated groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	Deprecated *HCloudMachineDeprecatedStatus `json:"deprecated,omitempty"`
}

// HCloudMachineInitializationStatus provides observations of the HCloudMachine initialization process.
// +kubebuilder:validation:MinProperties=1
type HCloudMachineInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the HCloudMachine's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// HCloudMachineDeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HCloudMachineDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *HCloudMachineV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// HCloudMachineV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HCloudMachineV1Beta1DeprecatedStatus struct {
	// conditions defines current service state of the HCloudMachine.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 is dropped.
	Conditions []clusterv1.Condition `json:"conditions,omitempty"`
}

// InstanceState is the state of the HCloud server that backs an HCloudMachine. It is set from the
// Hetzner Cloud server status, with additional CAPH owned states for lifecycle phases that Hetzner does
// not report on its own, such as deletion driven by CAPH. The field may hold any status the Hetzner API
// reports; the constants below name the states CAPH handles explicitly.
type InstanceState string

const (
	// InstanceStateInitializing is set when the server is initializing.
	InstanceStateInitializing InstanceState = "initializing"
	// InstanceStateOff is set when the server is off.
	InstanceStateOff InstanceState = "off"
	// InstanceStateRunning is set when the server is running.
	InstanceStateRunning InstanceState = "running"
	// InstanceStateStarting is set when the server is starting.
	InstanceStateStarting InstanceState = "starting"
	// InstanceStateStopping is set when the server is stopping.
	InstanceStateStopping InstanceState = "stopping"
	// InstanceStateDeleting is set when CAPH is deleting the server.
	InstanceStateDeleting InstanceState = "deleting"
	// InstanceStateUnknown is set when the server state is unknown.
	InstanceStateUnknown InstanceState = "unknown"
)

// HCloudMachineStatusExternalIDs holds temporary data during the provisioning process.
type HCloudMachineStatusExternalIDs struct {
	// ActionIDEnableRescueSystem is the hcloud API Action result of EnableRescueSystem.
	// +optional
	ActionIDEnableRescueSystem int64 `json:"actionIdEnableRescueSystem,omitzero"`

	// ActionIDCreateServer is the hcloud API Action result of CreateServer.
	// +optional
	ActionIDCreateServer int64 `json:"actionIdCreateServer,omitzero"`
}

// HCloudMachine is the Schema for the hcloudmachines API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hcloudmachines,scope=Namespaced,categories=cluster-api,shortName=hcma
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this HCloudMachine belongs"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this HCloudMachine"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="HCloudMachine infrastructure is ready"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.instanceState",description="Phase of HCloudMachine"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of hcloudmachine"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason",priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1
// +k8s:defaulter-gen=true
type HCloudMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudMachineSpec   `json:"spec,omitempty"`
	Status HCloudMachineStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for the HCloudMachine object.
func (r *HCloudMachine) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets the conditions for the HCloudMachine object.
func (r *HCloudMachine) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

// GetV1Beta1Conditions returns the deprecated v1beta1 conditions of the HCloudMachine object.
func (r *HCloudMachine) GetV1Beta1Conditions() clusterv1.Conditions {
	if r.Status.Deprecated == nil || r.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return r.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the deprecated v1beta1 conditions on the HCloudMachine object.
func (r *HCloudMachine) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if r.Status.Deprecated == nil {
		r.Status.Deprecated = &HCloudMachineDeprecatedStatus{}
	}
	if r.Status.Deprecated.V1Beta1 == nil {
		r.Status.Deprecated.V1Beta1 = &HCloudMachineV1Beta1DeprecatedStatus{}
	}
	r.Status.Deprecated.V1Beta1.Conditions = conditions
}

// HCloudMachineSummaryOpts returns the summary options for an HCloudMachine.
// It is the single source of truth for which conditions contribute to the Ready summary,
// used both by MachineScope.Close() and by early-exit error paths that bypass the scope.
//
// The conditions are listed in priority order (highest first). The ordering reflects
// operational importance:
//  1. HCloudTokenAvailable    - invalid credentials block everything.
//  2. HCloudRateLimitExceeded - rate-limit issues (negative polarity).
//  3. SSHPrivateKeyAvailable  - the rescue SSH key (imageURL flow) is a precondition for creating and provisioning the server.
//  4. ServerCreated           - server existence precedes later lifecycle stages; bootstrap readiness is folded in as a reason.
//  5. ServerProvisioned       - provisioning precedes availability.
//  6. ServerAvailable
func HCloudMachineSummaryOpts() []conditions.SummaryOption {
	return []conditions.SummaryOption{
		// ForConditionTypes lists every condition that contributes to Ready, in
		// priority order. When multiple conditions are unhealthy the summary
		// surfaces them in this order, so the most important issue is listed first.
		conditions.ForConditionTypes{
			HCloudTokenAvailableCondition,
			HCloudRateLimitExceededCondition,
			HCloudMachineSSHPrivateKeyAvailableCondition,
			HCloudMachineServerCreatedCondition,
			HCloudMachineServerProvisionedCondition,
			HCloudMachineServerAvailableCondition,
		},
		// IgnoreTypesIfMissing tells the summary not to treat the absence of a
		// listed condition as Unknown. Some reconcile paths exit before every
		// condition has been set (for example, before the token is checked or
		// before the server is created), and we don't want those early exits to
		// flip Ready to Unknown.
		conditions.IgnoreTypesIfMissing{
			HCloudTokenAvailableCondition,
			HCloudMachineSSHPrivateKeyAvailableCondition,
			HCloudMachineServerCreatedCondition,
			HCloudMachineServerProvisionedCondition,
			HCloudMachineServerAvailableCondition,
			HCloudRateLimitExceededCondition,
		},
		// CustomMergeStrategy is used only to override the merge reasons, so
		// the Ready summary uses CAPI's standard Ready reasons (Ready /
		// NotReady / ReadyUnknown) instead of the generic merge defaults
		// (IssuesReported / UnknownReported / InfoReported).
		//
		// Negative polarity is passed directly into GetDefaultMergePriorityFunc
		// here. When a CustomMergeStrategy is provided, NewSummaryCondition
		// skips the path that wires up the NegativePolarityConditionTypes
		// SummaryOption into the default strategy, so the negative-polarity
		// types must be specified explicitly inside the strategy.
		conditions.CustomMergeStrategy{
			MergeStrategy: conditions.DefaultMergeStrategy(
				conditions.GetPriorityFunc(conditions.GetDefaultMergePriorityFunc(
					// conditions with negative polarity
					HCloudRateLimitExceededCondition,
				)),
				conditions.ComputeReasonFunc(conditions.GetDefaultComputeMergeReasonFunc(
					clusterv1.NotReadyReason,
					clusterv1.ReadyUnknownReason,
					clusterv1.ReadyReason,
				)),
			),
		},
	}
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
