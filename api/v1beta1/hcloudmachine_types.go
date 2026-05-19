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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
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
	Addresses []clusterv1beta1.MachineAddress `json:"addresses,omitempty"`

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
	FailureReason *string `json:"failureReason,omitempty"`

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
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in HCloudMachine's status with the V1Beta2 version.
	// +optional
	V1Beta2 *HCloudMachineV1Beta2Status `json:"v1beta2,omitempty"`

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

	// LastRemediatedAt records when the most recent successful remediation completed.
	// Used to prevent reboot loops across successive MHC incidents.
	// +optional
	LastRemediatedAt *metav1.Time `json:"lastRemediatedAt,omitempty"`
}

// HCloudMachineV1Beta2Status groups all the fields that will be added or modified in HCloudMachineStatus with the V1Beta2 version.
type HCloudMachineV1Beta2Status struct {
	// conditions represents the observations of a HCloudMachine's current state.
	// Known condition types are Ready, HCloudTokenAvailable, HCloudRateLimitExceeded, ServerCreated, ServerProvisioned and ServerAvailable.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// HCloudMachineStatusExternalIDs holds temporary data during the provisioning process.
type HCloudMachineStatusExternalIDs struct {
	// ActionIDEnableRescueSystem is the hcloud API Action result of EnableRescueSystem.
	// +optional
	ActionIDEnableRescueSystem int64 `json:"actionIdEnableRescueSystem,omitzero"`
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

// GetConditions returns the observations of the operational state of the HCloudMachine resource.
func (r *HCloudMachine) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HCloudMachine to the predescribed clusterv1beta1.Conditions.
func (r *HCloudMachine) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the observations of the operational state of the HCloudMachine resource.
func (r *HCloudMachine) GetV1Beta2Conditions() []metav1.Condition {
	if r.Status.V1Beta2 == nil {
		return nil
	}
	return r.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets the underlying v1beta2 service state of the HCloudMachine.
func (r *HCloudMachine) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if r.Status.V1Beta2 == nil {
		r.Status.V1Beta2 = &HCloudMachineV1Beta2Status{}
	}
	r.Status.V1Beta2.Conditions = conditions
}

// HCloudMachineV1Beta2SummaryOpts returns the v1beta2 summary options for an HCloudMachine.
// It is the single source of truth for which conditions contribute to the Ready summary,
// used both by MachineScope.Close() and by early-exit error paths that bypass the scope.
//
// The order of conditions in ForConditionTypes defines the priority for the Ready summary:
// when multiple conditions are unhealthy, the summary lists all of them in priority
// order (highest-priority first). The ordering reflects operational importance:
//  1. HCloudTokenAvailable    - invalid credentials block everything.
//  2. HCloudRateLimitExceeded - rate-limit issues (negative polarity).
//  3. ServerCreated           - server existence precedes later lifecycle stages; bootstrap readiness is folded in as a reason.
//  4. ServerProvisioned       - provisioning precedes availability.
//  5. ServerAvailable
func HCloudMachineV1Beta2SummaryOpts() []v1beta2conditions.SummaryOption {
	return []v1beta2conditions.SummaryOption{
		v1beta2conditions.ForConditionTypes{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
			HCloudMachineServerCreatedV1Beta2Condition,
			HCloudMachineServerProvisionedV1Beta2Condition,
			HCloudMachineServerAvailableV1Beta2Condition,
		},
		v1beta2conditions.NegativePolarityConditionTypes{
			HCloudRateLimitExceededV1Beta2Condition,
		},
		v1beta2conditions.IgnoreTypesIfMissing{
			HCloudTokenAvailableV1Beta2Condition,
			HCloudMachineServerCreatedV1Beta2Condition,
			HCloudMachineServerProvisionedV1Beta2Condition,
			HCloudMachineServerAvailableV1Beta2Condition,
			HCloudRateLimitExceededV1Beta2Condition,
		},
		// Customize the Ready summary: flag RateLimitExceeded=True as an issue
		// (negative-polarity), and report HCloudMachine-specific reasons instead
		// of CAPI's generic defaults (IssuesReported / UnknownReported / InfoReported).
		v1beta2conditions.CustomMergeStrategy{
			MergeStrategy: v1beta2conditions.DefaultMergeStrategy(
				// Register the rate-limit condition as negative-polarity (True = problem)
				// so it's bucketed as an issue and drives the Ready summary when firing.
				v1beta2conditions.GetPriorityFunc(v1beta2conditions.GetDefaultMergePriorityFunc(
					HCloudRateLimitExceededV1Beta2Condition,
				)),
				v1beta2conditions.ComputeReasonFunc(v1beta2conditions.GetDefaultComputeMergeReasonFunc(
					clusterv1beta1.NotReadyV1Beta2Reason,
					clusterv1beta1.ReadyUnknownV1Beta2Reason,
					clusterv1beta1.ReadyV1Beta2Reason,
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
