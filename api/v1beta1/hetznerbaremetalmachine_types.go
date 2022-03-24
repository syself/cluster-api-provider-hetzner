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

package v1beta1

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// BareMetalMachineFinalizer allows Reconcilehetznerbaremetalmachine to clean up resources associated with hetznerbaremetalmachine before
	// removing it from the apiserver.
	BareMetalMachineFinalizer = "hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io"
)

var errUnknownSuffix = errors.New("unknown suffix")

// ImageType defines the accepted image types.
type ImageType string

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
	// ProviderID will be the hetznerbaremetalmachine in ProviderID format
	// (hetzner://<server-id>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// InstallImage is the configuration which is used for the autosetup configuration for installing an OS via InstallImage.
	InstallImage InstallImage `json:"installImage"`

	// HostSelector specifies matching criteria for labels on HetznerBareMetalHosts.
	// This is used to limit the set of HetznerBareMetalHost objects considered for
	// claiming for a HetznerBareMetalMachine.
	// +optional
	HostSelector HostSelector `json:"hostSelector,omitempty"`

	// SSHSpec gives a reference on the secret where SSH details are specified as well as ports for ssh.
	SSHSpec SSHSpec `json:"sshSpec,omitempty"`
}

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

// HostSelectorRequirement defines a requirement used for MatchExpressions to select host machines.
type HostSelectorRequirement struct {
	Key      string             `json:"key"`
	Operator selection.Operator `json:"operator"`
	Values   []string           `json:"values"`
}

// SSHSpec defines specs for SSH.
type SSHSpec struct {
	SecretRef             SSHSecretRef `json:"secretRef"`
	PortAfterInstallImage int          `json:"portAfterInstallImage"`
	PortAfterCloudInit    int          `json:"portAfterCloudInit"`
}

// SSHSecretRef defines the secret containing all information of the SSH key used for Hetzner robot.
type SSHSecretRef struct {
	Name string          `json:"name"`
	Key  SSHSecretKeyRef `json:"key"`
}

// SSHSecretKeyRef defines the key name of the SSHSecret.
type SSHSecretKeyRef struct {
	Name       string `json:"name"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

// InstallImage defines the configuration for InstallImage.
type InstallImage struct {
	// Image is the image to be provisioned.
	Image Image `json:"image"`

	// PostInstallScript is used for configuring commands which should be executed after installimage.
	// It is passed along with the installimage command.
	PostInstallScript string `json:"postInstallScript,omitempty"`

	// Partitions defines the additional Partitions to be created.
	Partitions []Partition `json:"partitions"`

	// LVMDefinitions defines the logical volume definitions to be created.
	// +optional
	LVMDefinitions []LVMDefinition `json:"logicalVolumeDefinitions,omitempty"`

	// BTRFSDefinitions defines the btrfs subvolume definitions to be created.
	// +optional
	BTRFSDefinitions []BTRFSDefinition `json:"btrfsDefinitions,omitempty"`
}

// Image defines the properties for the autosetup config.
type Image struct {
	// URL defines the remote URL for downloading a tar, tar.gz, tar.bz, tar.bz2, tar.xz, tgz, tbz, txz image.
	URL string `json:"url,omitempty"`

	// Name defines the archive name after download. This has to be a valid name for Installimage.
	Name string `json:"name,omitempty"`

	// Path is the local path for a preinstalled image from upstream.
	Path string `json:"path,omitempty"`
}

// Partition defines the additional Partitions to be created.
type Partition struct {

	// Mount defines the mount path for this filesystem.
	// or keyword 'lvm' to use this PART as volume group (VG) for LVM
	// identifier 'btrfs.X' to use this PART as volume for
	// btrfs subvolumes. X can be replaced with a unique
	// alphanumeric keyword. NOTE: no support btrfs multi-device volumes
	Mount string `json:"mount"`

	// FileSystem can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap
	// or name of the LVM volume group (VG), if this PART is a VG.
	FileSystem string `json:"fileSystem"`

	// Size can use the keyword 'all' to assign all the remaining space of the drive to the last partition.
	// can use M/G/T for unit specification in MiB/GiB/TiB
	Size string `json:"size"`
}

// BTRFSDefinition defines the btrfs subvolume definitions to be created.
type BTRFSDefinition struct {
	// Volume defines the btrfs volume name.
	Volume string `json:"volume"`

	// SubVolume defines the subvolume name.
	SubVolume string `json:"subvolume"`

	// Mount defines the mountpath.
	Mount string `json:"mount"`
}

// LVMDefinition defines the logical volume definitions to be created.
type LVMDefinition struct {
	// VG defines the vg name.
	VG string `json:"vg"`

	// Name defines the volume name.
	Name string `json:"name"`

	// Mount defines the mountpath.
	Mount string `json:"mount"`

	// FileSystem defines the filesystem for this logical volume.
	FileSystem string `json:"filesystem"`

	// Size defines the size in M/G/T or MiB/GiB/TiB.
	Size string `json:"size"`
}

// HetznerBareMetalMachineStatus defines the observed state of HetznerBareMetalMachine.
type HetznerBareMetalMachineStatus struct {

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem.
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Addresses is a list of addresses assigned to the machine.
	// This field is copied from the infrastructure provider reference.
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

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

// SetFailure sets a failure reason and message.
func (r *HetznerBareMetalMachine) SetFailure(reason capierrors.MachineStatusError, message string) {
	r.Status.FailureReason = &reason
	r.Status.FailureMessage = &message
}

// GetImageSuffix tests whether the suffix is known and outputs it if yes. Otherwise it returns an error.
func GetImageSuffix(url string) (string, error) {
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

	return "", errors.Wrapf(errUnknownSuffix, "unknown suffix in URL %s", url)
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
