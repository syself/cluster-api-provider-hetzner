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

package v1beta2

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// HetznerBareMetalMachineFinalizer allows Reconcilehetznerbaremetalmachine to clean up resources associated with hetznerbaremetalmachine before
	// removing it from the apiserver.
	HetznerBareMetalMachineFinalizer = "infrastructure.cluster.x-k8s.io/hetznerbaremetalmachine"

	// BareMetalHostNamePrefix is a prefix for all hostNames of bare metal servers.
	BareMetalHostNamePrefix = "bm-"
)

var errUnknownSuffix = errors.New("unknown suffix")

// ImageType defines the accepted image types.
type ImageType string

// DeviceStringType controls what CAPH passes as the device argument to customProvisioner.command.
// Allowed values are "" (same as "short"), "short", and "wwn".
type DeviceStringType string

const (
	// DeviceStringTypeShort passes the short device name (e.g. "sda") to customProvisioner.command.
	DeviceStringTypeShort DeviceStringType = "short"
	// DeviceStringTypeWWN passes the WWN (e.g. "eui.00253885910c8cec") to customProvisioner.command.
	DeviceStringTypeWWN DeviceStringType = "wwn"
)

const (
	// ImageTypeTar defines the image type for tar files.
	ImageTypeTar ImageType = "tar"
	// ImageTypeTarGz defines the image type for tar.gz files.
	ImageTypeTarGz ImageType = "tar.gz"
	// ImageTypeTarBz defines the image type for tar.bz files.
	ImageTypeTarBz ImageType = "tar.bz"
	// ImageTypeTarBz2 defines the image type for tar.bz2 files.
	ImageTypeTarBz2 ImageType = "tar.bz2"
	// ImageTypeTarXz defines the image type for tar.xz files.
	ImageTypeTarXz ImageType = "tar.xz"
	// ImageTypeTgz defines the image type for tgz files.
	ImageTypeTgz ImageType = "tgz"
	// ImageTypeTbz defines the image type for tbz files.
	ImageTypeTbz ImageType = "tbz"
	// ImageTypeTxz defines the image type for txz files.
	ImageTypeTxz ImageType = "txz"
)

// HetznerBareMetalMachineSpec defines the desired state of HetznerBareMetalMachine.
type HetznerBareMetalMachineSpec struct {
	// providerID is set by the controller to either (new) `hrobot://<server-id>` or (old)
	// `hcloud://bm-NNNN` format. If the HetznerBareMetalMachineSpec has already a providerID, then
	// this will never change. If the providerID is empty, the controller sets it to the old format
	// by default (hcloud://bm-NNNN), except the Annotation
	// `capi.syself.com/use-hrobot-provider-id-for-baremetal` on the hetznerCluster is set to
	// `"true"`.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// installImage provisions the machine with the Hetzner installimage tool.
	// Exactly one of installImage or customProvisioner must be set.
	// +optional
	InstallImage *InstallImage `json:"installImage,omitempty"`

	// customProvisioner provisions the machine with a custom command instead of installimage.
	// Exactly one of installImage or customProvisioner must be set.
	// +optional
	CustomProvisioner *CustomProvisioner `json:"customProvisioner,omitempty"`

	// hostSelector specifies matching criteria for labels on HetznerBareMetalHosts.
	// This is used to limit the set of HetznerBareMetalHost objects considered for
	// claiming for a HetznerBareMetalMachine.
	// +optional
	HostSelector HostSelector `json:"hostSelector,omitempty"`

	// sshSpec gives a reference on the secret where SSH details are specified as well as ports for SSH.
	SSHSpec SSHSpec `json:"sshSpec,omitempty"`

	// skipCheckDisk skips the CheckDisk step during provisioning.
	// This is equivalent to setting the annotation capi.syself.com/ignore-check-disk on the HetznerBareMetalHost.
	// +optional
	SkipCheckDisk bool `json:"skipCheckDisk,omitempty"`
}

// HostSelector specifies matching criteria for labels on BareMetalHosts.
// This is used to limit the set of BareMetalHost objects considered for
// claiming for a Machine.
type HostSelector struct {
	// matchLabels defines the key/value pairs of labels that must exist on a chosen BareMetalHost.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// matchExpressions defines the label match expressions that must be true on a chosen BareMetalHost.
	// +optional
	// +listType=atomic
	MatchExpressions []HostSelectorRequirement `json:"matchExpressions,omitempty"`
}

// HostSelectorRequirement defines a requirement used for MatchExpressions to select host machines.
type HostSelectorRequirement struct {
	// key defines the key of the label that should be matched in the host object.
	Key string `json:"key"`

	// operator defines the selection operator.
	Operator selection.Operator `json:"operator"`

	// values define the values whose relation to the label value in the host machine is defined by the selection operator.
	// +listType=atomic
	Values []string `json:"values"`
}

// SSHSpec defines specs for SSH.
type SSHSpec struct {
	// secretRef gives reference to the secret where the SSH key is stored.
	SecretRef SSHSecretRef `json:"secretRef"`

	// noSSHAfterInstallImage disables SSH access to the machine after installimage
	// completed successfully.
	// +optional
	NoSSHAfterInstallImage bool `json:"noSSHAfterInstallImage,omitempty"`

	// portAfterInstallImage specifies the port that has to be used to connect to the machine
	// by reaching the server via SSH after installing the image successfully.
	// +kubebuilder:default=22
	// +optional
	PortAfterInstallImage int `json:"portAfterInstallImage"`

	// portAfterCloudInit is deprecated. Since PR Install Cloud-Init-Data via post-install.sh #1407 this field is not functional.
	//
	// Deprecated: This field is not used anymore.
	// +optional
	PortAfterCloudInit int `json:"portAfterCloudInit"`
}

// SSHSecretRef defines the secret containing all information of the SSH key used for the Hetzner robot.
type SSHSecretRef struct {
	// name is the name of the secret.
	Name string `json:"name"`

	// key contains details about the keys used in the data of the secret.
	Key SSHSecretKeyRef `json:"key"`
}

// SSHSecretKeyRef defines the key name of the SSHSecret.
type SSHSecretKeyRef struct {
	// name is the key in the secret's data where the SSH key's name is stored.
	Name string `json:"name"`

	// publicKey is the key in the secret's data where the SSH key's public key is stored.
	PublicKey string `json:"publicKey"`

	// privateKey is the key in the secret's data where the SSH key's private key is stored.
	PrivateKey string `json:"privateKey"`
}

// InstallImage defines the configuration for provisioning a machine with the Hetzner installimage tool.
type InstallImage struct {
	// image is the image to be provisioned. It defines the image for baremetal machine.
	Image Image `json:"image"`

	// postInstallScript (Bash) is used for configuring commands that should be executed after installimage.
	// It is passed along with the installimage command.
	PostInstallScript string `json:"postInstallScript,omitempty"`

	// partitions define the additional partitions to be created in installimage.
	// +listType=atomic
	Partitions []Partition `json:"partitions"`

	// logicalVolumeDefinitions defines the logical volume definitions to be created.
	// +optional
	// +listType=atomic
	LVMDefinitions []LVMDefinition `json:"logicalVolumeDefinitions,omitempty"`

	// btrfsDefinitions define the btrfs subvolume definitions to be created.
	// +optional
	// +listType=atomic
	BTRFSDefinitions []BTRFSDefinition `json:"btrfsDefinitions,omitempty"`

	// swraid defines the SWRAID in InstallImage. It enables or disables raids. Set 1 to enable.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Enum=0;1;
	Swraid int `json:"swraid"`

	// swraidLevel defines the SWRAIDLEVEL in InstallImage. Only relevant if the raid is enabled.
	// Pick one of 0,1,5,6,10. Ignored if swraid=0.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Enum=0;1;5;6;10;
	SwraidLevel int `json:"swraidLevel,omitempty"`
}

// CustomProvisioner defines the configuration for provisioning a machine with a custom command
// instead of the Hetzner installimage tool.
type CustomProvisioner struct {
	// url is the location of the image that command provisions the machine from. CAPH passes it to
	// command in the rescue system.
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// command is the basename of a command file below /shared on the controller pod. CAPH copies
	// that command into the rescue system and executes it there to provision the machine from URL.
	//
	// Docs: https://syself.com/docs/caph/developers/image-url-command
	// +kubebuilder:validation:MinLength=1
	Command string `json:"command"`

	// deviceStringType instructs CAPH to either use the short device name, or the WWN when calling
	// command. "" and "short" both pass the short device name (e.g. "sda"); "wwn" passes the WWN
	// (e.g. "eui.00253885910c8cec").
	// +kubebuilder:validation:Enum="";short;wwn
	// +optional
	DeviceStringType DeviceStringType `json:"deviceStringType,omitempty"`

	// swraid defines whether command sets up a RAID. Set 1 to enable. CAPH then uses a
	// HetznerBareMetalHost with spec.rootDeviceHints.raid.wwn and passes all these disks to command.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Enum=0;1;
	Swraid int `json:"swraid,omitempty"`
}

// Image defines the properties for the autosetup config.
type Image struct {
	// url defines the remote URL for downloading a tar, tar.gz, tar.bz, tar.bz2, tar.xz, tgz, tbz, txz image.
	URL string `json:"url,omitempty"`

	// name defines the archive name after download. This has to be a valid name for Installimage.
	Name string `json:"name,omitempty"`

	// path is the local path for a preinstalled image from upstream.
	Path string `json:"path,omitempty"`
}

// GetDetails returns the path of the image and whether the image has to be downloaded.
func (image Image) GetDetails() (imagePath string, needsDownload bool, errorMessage string) {
	switch {
	case image.Name != "" && image.URL != "":
		suffix, err := GetImageSuffix(image.URL)
		if err != nil {
			errorMessage = "wrong image url suffix"
			return
		}
		imagePath = fmt.Sprintf("/root/%s.%s", image.Name, suffix)
		needsDownload = true
	case image.Path != "":
		// In the other case a local imagePath is specified
		imagePath = image.Path
	default:
		errorMessage = "invalid image - need to specify either name and url or path"
	}
	return imagePath, needsDownload, errorMessage
}

// String returns a string representation. The password gets redacted from the URL.
func (image Image) String() string {
	cleanURL := ""
	if image.URL != "" {
		u, err := url.Parse(image.URL)
		if err != nil {
			cleanURL = err.Error()
		} else {
			cleanURL = u.Redacted()
		}
	}
	if cleanURL == "" {
		cleanURL = image.Path
	}
	if image.Name == "" {
		return cleanURL
	}
	return fmt.Sprintf("%s (%s)", image.Name, cleanURL)
}

// Partition defines the additional Partitions to be created.
type Partition struct {
	// mount defines the mount path for this filesystem.
	// Keyword 'lvm' to use this PART as volume group (VG) for LVM.
	// Identifier 'btrfs.X' to use this PART as volume for
	// btrfs subvolumes. X can be replaced with a unique
	// alphanumeric keyword. NOTE: no support for btrfs multi-device volumes.
	Mount string `json:"mount"`

	// fileSystem can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap
	// or name of the LVM volume group (VG), if this PART is a VG.
	FileSystem string `json:"fileSystem"`

	// size can use the keyword 'all' to assign all the remaining space of the drive to the last partition.
	// You can use M/G/T for unit specification in MiB/GiB/TiB.
	Size string `json:"size"`
}

// BTRFSDefinition defines the btrfs subvolume definitions to be created.
type BTRFSDefinition struct {
	// volume defines the btrfs volume name.
	Volume string `json:"volume"`

	// subvolume defines the subvolume name.
	SubVolume string `json:"subvolume"`

	// mount defines the mountpath.
	Mount string `json:"mount"`
}

// LVMDefinition defines the logical volume definitions to be created.
type LVMDefinition struct {
	// vg defines the vg name.
	VG string `json:"vg"`

	// name defines the volume name.
	Name string `json:"name"`

	// mount defines the mountpath.
	Mount string `json:"mount"`

	// filesystem defines the filesystem for this logical volume.
	FileSystem string `json:"filesystem"`

	// size defines the size in M/G/T or MiB/GiB/TiB.
	Size string `json:"size"`
}

// HetznerBareMetalMachineStatus defines the observed state of HetznerBareMetalMachine.
type HetznerBareMetalMachineStatus struct {
	// conditions represents the observations of a HetznerBareMetalMachine's current state.
	// Known condition types are Ready, HCloudTokenAvailable, HostAssociated, HostReady and ServerAvailable.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// initialization provides observations of the HetznerBareMetalMachine initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization HetznerBareMetalMachineInitializationStatus `json:"initialization,omitempty,omitzero"`

	// addresses is a list of addresses assigned to the machine.
	// This field is copied from the infrastructure provider reference.
	// +optional
	// +listType=atomic
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// phase represents the current phase of HetznerBareMetalMachineStatus actuation.
	// E.g. Pending, Running, Terminating, Failed, etc.
	// +optional
	Phase clusterv1.MachinePhase `json:"phase,omitempty"`

	// lastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty,omitzero"`

	// lastRemediatedAt records when the most recent successful remediation completed.
	// Used to prevent reboot loops across successive MHC incidents.
	// +optional
	LastRemediatedAt metav1.Time `json:"lastRemediatedAt,omitempty,omitzero"`

	// deprecated groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	Deprecated *HetznerBareMetalMachineDeprecatedStatus `json:"deprecated,omitempty"`
}

// HetznerBareMetalMachineInitializationStatus provides observations of the HetznerBareMetalMachine initialization process.
// +kubebuilder:validation:MinProperties=1
type HetznerBareMetalMachineInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the HetznerBareMetalMachine's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// HetznerBareMetalMachineDeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HetznerBareMetalMachineDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *HetznerBareMetalMachineV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// HetznerBareMetalMachineV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type HetznerBareMetalMachineV1Beta1DeprecatedStatus struct {
	// conditions defines current service state of the HetznerBareMetalMachine.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 is dropped.
	Conditions []clusterv1.Condition `json:"conditions,omitempty"`
}

// HetznerBareMetalMachine is the Schema for the hetznerbaremetalmachines API.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hetznerbaremetalmachines,scope=Namespaced,categories=cluster-api,shortName=hbmm
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this HetznerBareMetalMachine belongs"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this HetznerBareMetalMachine"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="HetznerBareMetalMachine infrastructure is ready"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="HetznerBareMetalMachine status such as Pending/Provisioning/Running etc"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HetznerBareMetalMachine"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".metadata.annotations.infrastructure\\.cluster\\.x-k8s\\.io/HetznerBareMetalHost",description="HetznerBareMetalHost",priority=1
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason",priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1
type HetznerBareMetalMachine struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec HetznerBareMetalMachineSpec `json:"spec,omitempty"`
	// +optional
	Status HetznerBareMetalMachineStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for the HetznerBareMetalMachine resource.
func (hbmm *HetznerBareMetalMachine) GetConditions() []metav1.Condition {
	return hbmm.Status.Conditions
}

// SetConditions sets conditions for the HetznerBareMetalMachine object.
func (hbmm *HetznerBareMetalMachine) SetConditions(conditions []metav1.Condition) {
	hbmm.Status.Conditions = conditions
}

// GetV1Beta1Conditions returns the deprecated v1beta1 conditions of the HetznerBareMetalMachine object.
func (hbmm *HetznerBareMetalMachine) GetV1Beta1Conditions() clusterv1.Conditions {
	if hbmm.Status.Deprecated == nil || hbmm.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return hbmm.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the deprecated v1beta1 conditions on the HetznerBareMetalMachine object.
func (hbmm *HetznerBareMetalMachine) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if hbmm.Status.Deprecated == nil {
		hbmm.Status.Deprecated = &HetznerBareMetalMachineDeprecatedStatus{}
	}
	if hbmm.Status.Deprecated.V1Beta1 == nil {
		hbmm.Status.Deprecated.V1Beta1 = &HetznerBareMetalMachineV1Beta1DeprecatedStatus{}
	}
	hbmm.Status.Deprecated.V1Beta1.Conditions = conditions
}

// HetznerBareMetalMachineSummaryOpts returns the summary options for a HetznerBareMetalMachine.
//
// The order of conditions in ForConditionTypes defines the priority for the Ready summary:
// when multiple conditions are unhealthy, the summary lists all of them in priority
// order (highest-priority first). The ordering reflects operational importance:
//  1. HCloudTokenAvailable - invalid credentials block everything.
//  2. HostAssociated       - host association precedes host readiness; bootstrap readiness is folded in as a reason.
//  3. Deleting             - deletion progress (negative polarity), which should be surfaced before host readiness.
//  4. HostReady            - underlying HetznerBareMetalHost readiness.
//  5. ServerAvailable      - the HetznerBareMetalMachine is fully available; for control planes this is also gated on load balancer attachment.
func HetznerBareMetalMachineSummaryOpts() []conditions.SummaryOption {
	return []conditions.SummaryOption{
		// ForConditionTypes lists every condition that contributes to Ready, in
		// priority order. When multiple conditions are unhealthy the summary
		// surfaces them in this order, so the most important issue is listed first.
		conditions.ForConditionTypes{
			HCloudTokenAvailableCondition,
			HetznerBareMetalMachineHostAssociatedCondition,
			HetznerBareMetalMachineDeletingCondition,
			HetznerBareMetalMachineHostReadyCondition,
			HetznerBareMetalMachineServerAvailableCondition,
		},
		// IgnoreTypesIfMissing lists the conditions whose absence does not mean the HetznerBareMetalMachine
		// is not ready. Deleting is set only while the HetznerBareMetalMachine is being deleted. The other
		// conditions are steps of bringing the HetznerBareMetalMachine up, so a missing one counts as Unknown.
		conditions.IgnoreTypesIfMissing{
			HetznerBareMetalMachineDeletingCondition,
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
					HetznerBareMetalMachineDeletingCondition,
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

// GetImageSuffix tests whether the suffix is known and outputs it if yes. Otherwise it returns an error.
func GetImageSuffix(url string) (string, error) {
	if strings.HasPrefix(url, "oci://") {
		return "tar.gz", nil
	}
	for _, suffix := range []ImageType{
		ImageTypeTar,
		ImageTypeTarGz,
		ImageTypeTarBz,
		ImageTypeTarBz2,
		ImageTypeTarXz,
		ImageTypeTgz,
		ImageTypeTbz,
		ImageTypeTxz,
	} {
		if strings.HasSuffix(url, fmt.Sprintf(".%s", suffix)) {
			return string(suffix), nil
		}
	}

	return "", fmt.Errorf("unknown suffix in URL %s: %w", url, errUnknownSuffix)
}

// Swraid returns the swraid setting of installImage or customProvisioner, whichever is set.
func (hbmm *HetznerBareMetalMachine) Swraid() int {
	if hbmm.Spec.InstallImage != nil {
		return hbmm.Spec.InstallImage.Swraid
	}
	if hbmm.Spec.CustomProvisioner != nil {
		return hbmm.Spec.CustomProvisioner.Swraid
	}
	return 0
}

// HasHostAnnotation checks whether the annotation that references a host exists.
func (hbmm *HetznerBareMetalMachine) HasHostAnnotation() bool {
	annotations := hbmm.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[HostAnnotation]
	return ok
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
	objectTypes = append(objectTypes, &HetznerBareMetalMachine{}, &HetznerBareMetalMachineList{})
}
