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
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/ptr"
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
// Allowed values are "short" and "wwn". Not set means "short".
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
// +kubebuilder:validation:MinProperties=1
type HetznerBareMetalMachineSpec struct {
	// providerID is set by the controller to either (new) `hrobot://<server-id>` or (old)
	// `hcloud://bm-NNNN` format. If the HetznerBareMetalMachineSpec has already a providerID, then
	// this will never change. If the providerID is empty, the controller sets it to the old format
	// by default (hcloud://bm-NNNN), except the Annotation
	// `capi.syself.com/use-hrobot-provider-id-for-baremetal` on the hetznerCluster is set to
	// `"true"`.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	ProviderID string `json:"providerID,omitempty"`

	// installImage provisions the machine with the Hetzner installimage tool.
	// Exactly one of installImage or customProvisioner must be set.
	// +optional
	InstallImage InstallImage `json:"installImage,omitempty,omitzero"`

	// customProvisioner provisions the machine with a custom command instead of installimage.
	// Exactly one of installImage or customProvisioner must be set.
	// +optional
	CustomProvisioner CustomProvisioner `json:"customProvisioner,omitempty,omitzero"`

	// hostSelector specifies matching criteria for labels on HetznerBareMetalHosts.
	// This is used to limit the set of HetznerBareMetalHost objects considered for
	// claiming for a HetznerBareMetalMachine. When it is not set, every HetznerBareMetalHost matches.
	// +optional
	HostSelector *HostSelector `json:"hostSelector,omitempty"`

	// sshSpec gives a reference on the secret where SSH details are specified as well as ports for SSH.
	// +optional
	SSHSpec SSHSpec `json:"sshSpec,omitempty,omitzero"`

	// skipCheckDisk skips the CheckDisk step during provisioning.
	// This is equivalent to setting the annotation capi.syself.com/ignore-check-disk on the HetznerBareMetalHost.
	// Not set means false.
	// +optional
	SkipCheckDisk *bool `json:"skipCheckDisk,omitempty"`
}

// HostSelector specifies matching criteria for labels on BareMetalHosts.
// This is used to limit the set of BareMetalHost objects considered for
// claiming for a Machine.
type HostSelector struct {
	// matchLabels defines the key/value pairs of labels that must exist on a chosen BareMetalHost.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// matchExpressions defines the label match expressions that must be true on a chosen BareMetalHost.
	// The limit is far above the number of expressions that a host selector needs.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=512
	MatchExpressions []HostSelectorRequirement `json:"matchExpressions,omitempty"`
}

// HostSelectorRequirement defines a requirement used for MatchExpressions to select host machines.
type HostSelectorRequirement struct {
	// key defines the key of the label that should be matched in the host object.
	// The limit is above the longest Kubernetes label key, which has 317 characters.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Key string `json:"key,omitempty"`

	// operator defines the selection operator.
	// +required
	Operator selection.Operator `json:"operator,omitempty"`

	// values define the values whose relation to the label value in the host machine is defined by the selection operator.
	// The limits are far above what a host selector needs. A Kubernetes label value has at most 63 characters.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=512
	// +kubebuilder:validation:items:MaxLength=512
	Values []string `json:"values,omitempty"`
}

// DefaultPortAfterInstallImage is the value of sshSpec.portAfterInstallImage when it is not set.
// It must match the +kubebuilder:default marker of the field.
const DefaultPortAfterInstallImage int32 = 22

// SSHSpec defines specs for SSH.
type SSHSpec struct {
	// secretRef gives reference to the secret where the SSH key is stored.
	// +required
	SecretRef SSHSecretRef `json:"secretRef,omitempty"`

	// noSSHAfterInstallImage disables SSH access to the machine after installimage
	// completed successfully. Not set means false.
	// +optional
	NoSSHAfterInstallImage *bool `json:"noSSHAfterInstallImage,omitempty"`

	// portAfterInstallImage specifies the port that has to be used to connect to the machine
	// by reaching the server via SSH after installing the image successfully.
	// +kubebuilder:default=22
	// +optional
	// +kubebuilder:validation:Maximum=65535
	PortAfterInstallImage *int32 `json:"portAfterInstallImage,omitempty"`

	// portAfterCloudInit is deprecated. Since PR Install Cloud-Init-Data via post-install.sh #1407 this field is not functional.
	//
	// Deprecated: This field is not used anymore.
	// +optional
	PortAfterCloudInit *int32 `json:"portAfterCloudInit,omitempty"`
}

// SSHSecretRef defines the secret containing all information of the SSH key used for the Hetzner robot.
type SSHSecretRef struct {
	// name is the name of the secret.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name,omitempty"`

	// key contains details about the keys used in the data of the secret.
	// +optional
	Key SSHSecretKeyRef `json:"key,omitempty,omitzero"`
}

// SSHSecretKeyRef defines the key name of the SSHSecret.
// +kubebuilder:validation:MinProperties=1
type SSHSecretKeyRef struct {
	// name is the key in the secret's data where the SSH key's name is stored.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name,omitempty"`

	// publicKey is the key in the secret's data where the SSH key's public key is stored.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	PublicKey string `json:"publicKey,omitempty"`

	// privateKey is the key in the secret's data where the SSH key's private key is stored.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	PrivateKey string `json:"privateKey,omitempty"`
}

// The values of the swraid fields when they are not set. They must match the +kubebuilder:default
// markers of these fields.
const (
	// DefaultSwraid is the value of installImage.swraid and customProvisioner.swraid when they are not set.
	DefaultSwraid int32 = 0

	// DefaultSwraidLevel is the value of installImage.swraidLevel when it is not set.
	DefaultSwraidLevel int32 = 1
)

// InstallImage defines the configuration for provisioning a machine with the Hetzner installimage tool.
type InstallImage struct {
	// image is the image to be provisioned. It defines the image for baremetal machine.
	// +optional
	Image Image `json:"image,omitempty,omitzero"`

	// postInstallScript (Bash) is used for configuring commands that should be executed after installimage.
	// It is passed along with the installimage command.
	// The limit is the Linux limit for one command argument, because CAPH sends the script to the server in one SSH command.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=131072
	PostInstallScript string `json:"postInstallScript,omitempty"`

	// partitions define the additional partitions to be created in installimage.
	// The limit is the number of entries in a default GPT partition table.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Partitions []Partition `json:"partitions,omitempty"`

	// logicalVolumeDefinitions defines the logical volume definitions to be created.
	// The limit is the same as for partitions.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=128
	LVMDefinitions []LVMDefinition `json:"logicalVolumeDefinitions,omitempty"`

	// btrfsDefinitions define the btrfs subvolume definitions to be created.
	// The limit is the same as for partitions.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=128
	BTRFSDefinitions []BTRFSDefinition `json:"btrfsDefinitions,omitempty"`

	// swraid defines the SWRAID in InstallImage. It enables or disables raids. Set 1 to enable.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Enum=0;1;
	Swraid *int32 `json:"swraid,omitempty"`

	// swraidLevel defines the SWRAIDLEVEL in InstallImage. Only relevant if the raid is enabled.
	// Pick one of 0,1,5,6,10. Ignored if swraid=0.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Enum=0;1;5;6;10;
	SwraidLevel *int32 `json:"swraidLevel,omitempty"`
}

// IsDefined returns true if the InstallImage is set.
func (i *InstallImage) IsDefined() bool {
	if i == nil {
		return false
	}
	return !reflect.DeepEqual(i, &InstallImage{})
}

// CustomProvisioner defines the configuration for provisioning a machine with a custom command
// instead of the Hetzner installimage tool.
type CustomProvisioner struct {
	// url is the location of the image that command provisions the machine from. CAPH passes it to
	// command in the rescue system.
	// The limit is 8 KiB, the default URL length limit of common HTTP servers.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=8192
	URL string `json:"url,omitempty"`

	// command is the basename of a command file below /shared on the controller pod. CAPH copies
	// that command into the rescue system and executes it there to provision the machine from url.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Command string `json:"command,omitempty"`

	// deviceStringType instructs CAPH to either use the short device name, or the WWN when calling
	// command. "short" or not set passes the short device name (e.g. "sda"); "wwn" passes the WWN
	// (e.g. "eui.00253885910c8cec").
	// +kubebuilder:validation:Enum=short;wwn
	// +optional
	DeviceStringType DeviceStringType `json:"deviceStringType,omitempty"`

	// swraid defines whether command sets up a RAID. Set 1 to enable. CAPH then uses a
	// HetznerBareMetalHost with spec.rootDeviceHints.raid.wwn and passes all these disks to command.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Enum=0;1;
	Swraid *int32 `json:"swraid,omitempty"`
}

// IsDefined returns true if the CustomProvisioner is set.
func (p *CustomProvisioner) IsDefined() bool {
	if p == nil {
		return false
	}
	return !reflect.DeepEqual(p, &CustomProvisioner{})
}

// Image defines the properties for the autosetup config.
type Image struct {
	// url defines the remote URL for downloading a tar, tar.gz, tar.bz, tar.bz2, tar.xz, tgz, tbz, txz image.
	// The limit is 8 KiB, the default URL length limit of common HTTP servers.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=8192
	URL string `json:"url,omitempty"`

	// name defines the archive name after download. This has to be a valid name for Installimage.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name,omitempty"`

	// path is the local path for a preinstalled image from upstream.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
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
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Mount string `json:"mount,omitempty"`

	// fileSystem can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap
	// or name of the LVM volume group (VG), if this PART is a VG.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	FileSystem string `json:"fileSystem,omitempty"`

	// size can use the keyword 'all' to assign all the remaining space of the drive to the last partition.
	// You can use M/G/T for unit specification in MiB/GiB/TiB.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	Size string `json:"size,omitempty"`
}

// BTRFSDefinition defines the btrfs subvolume definitions to be created.
type BTRFSDefinition struct {
	// volume defines the btrfs volume name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Volume string `json:"volume,omitempty"`

	// subvolume defines the subvolume name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	SubVolume string `json:"subvolume,omitempty"`

	// mount defines the mountpath.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Mount string `json:"mount,omitempty"`
}

// LVMDefinition defines the logical volume definitions to be created.
type LVMDefinition struct {
	// vg defines the vg name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	VG string `json:"vg,omitempty"`

	// name defines the volume name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name,omitempty"`

	// mount defines the mountpath.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Mount string `json:"mount,omitempty"`

	// filesystem defines the filesystem for this logical volume.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	FileSystem string `json:"filesystem,omitempty"`

	// size defines the size in M/G/T or MiB/GiB/TiB.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	Size string `json:"size,omitempty"`
}

// HetznerBareMetalMachineStatus defines the observed state of HetznerBareMetalMachine.
// +kubebuilder:validation:MinProperties=1
type HetznerBareMetalMachineStatus struct {
	// conditions represents the observations of a HetznerBareMetalMachine's current state.
	// Known condition types are Ready, HCloudTokenAvailable, HostAssociated, HostReady, ServerAvailable and Deleting.
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
	// +kubebuilder:validation:MaxItems=256
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// phase represents the current phase of HetznerBareMetalMachineStatus actuation.
	// One of Pending, Provisioning, Running, Deleting or Deleted.
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
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 is dropped.
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
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
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of HetznerBareMetalMachine.
	// +optional
	Spec HetznerBareMetalMachineSpec `json:"spec,omitempty,omitzero"`
	// status is the observed state of HetznerBareMetalMachine.
	// +optional
	Status HetznerBareMetalMachineStatus `json:"status,omitempty,omitzero"`
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
func (hbmm *HetznerBareMetalMachine) Swraid() int32 {
	if hbmm.Spec.InstallImage.IsDefined() {
		return ptr.Deref(hbmm.Spec.InstallImage.Swraid, DefaultSwraid)
	}
	if hbmm.Spec.CustomProvisioner.IsDefined() {
		return ptr.Deref(hbmm.Spec.CustomProvisioner.Swraid, DefaultSwraid)
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
