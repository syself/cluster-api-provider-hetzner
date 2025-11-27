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
	"errors"
	"fmt"
	"net/url"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// HetznerBareMetalMachineFinalizer allows Reconcilehetznerbaremetalmachine to clean up resources associated with hetznerbaremetalmachine before
	// removing it from the apiserver.
	HetznerBareMetalMachineFinalizer = "infrastructure.cluster.x-k8s.io/hetznerbaremetalmachine"

	// DeprecatedBareMetalMachineFinalizer contains the old string.
	// The controller will automatically update to the new string.
	DeprecatedBareMetalMachineFinalizer = "hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io"

	// BareMetalHostNamePrefix is a prefix for all hostNames of bare metal servers.
	BareMetalHostNamePrefix = "bm-"
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
	// ProviderID will be the hetznerbaremetalmachine which is set by the controller
	// in the `hcloud://bm-<server-id>` format.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// InstallImage is the configuration that is used for the autosetup configuration for installing an OS via InstallImage.
	InstallImage InstallImage `json:"installImage"`

	// HostSelector specifies matching criteria for labels on HetznerBareMetalHosts.
	// This is used to limit the set of HetznerBareMetalHost objects considered for
	// claiming for a HetznerBareMetalMachine.
	// +optional
	HostSelector HostSelector `json:"hostSelector,omitempty"`

	// SSHSpec gives a reference on the secret where SSH details are specified as well as ports for SSH.
	SSHSpec SSHSpec `json:"sshSpec,omitempty"`
}

// HostSelector specifies matching criteria for labels on BareMetalHosts.
// This is used to limit the set of BareMetalHost objects considered for
// claiming for a Machine.
type HostSelector struct {
	// MatchLabels defines the key/value pairs of labels that must exist on a chosen BareMetalHost.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchExpressions defines the label match expressions that must be true on a chosen BareMetalHost.
	// +optional
	MatchExpressions []HostSelectorRequirement `json:"matchExpressions,omitempty"`
}

// HostSelectorRequirement defines a requirement used for MatchExpressions to select host machines.
type HostSelectorRequirement struct {
	// Key defines the key of the label that should be matched in the host object.
	Key string `json:"key"`

	// Operator defines the selection operator.
	Operator selection.Operator `json:"operator"`

	// Values define the values whose relation to the label value in the host machine is defined by the selection operator.
	Values []string `json:"values"`
}

// SSHSpec defines specs for SSH.
type SSHSpec struct {
	// SecretRef gives reference to the secret where the SSH key is stored.
	SecretRef SSHSecretRef `json:"secretRef"`

	// PortAfterInstallImage specifies the port that has to be used to connect to the machine
	// by reaching the server via SSH after installing the image successfully.
	// +kubebuilder:default=22
	// +optional
	PortAfterInstallImage int `json:"portAfterInstallImage"`

	// PortAfterCloudInit is deprecated. Since PR Install Cloud-Init-Data via post-install.sh #1407 this field is not functional.
	// Deprecated: This field is not used anymore.
	// +optional
	PortAfterCloudInit int `json:"portAfterCloudInit"`
}

// SSHSecretRef defines the secret containing all information of the SSH key used for the Hetzner robot.
type SSHSecretRef struct {
	// Name is the name of the secret.
	Name string `json:"name"`

	// Key contains details about the keys used in the data of the secret.
	Key SSHSecretKeyRef `json:"key"`
}

// SSHSecretKeyRef defines the key name of the SSHSecret.
type SSHSecretKeyRef struct {
	// Name is the key in the secret's data where the SSH key's name is stored.
	Name string `json:"name"`

	// PublicKey is the key in the secret's data where the SSH key's public key is stored.
	PublicKey string `json:"publicKey"`

	// PrivateKey is the key in the secret's data where the SSH key's private key is stored.
	PrivateKey string `json:"privateKey"`
}

// InstallImage defines the configuration for InstallImage.
type InstallImage struct {
	// Image is the image to be provisioned. It defines the image for baremetal machine.
	Image Image `json:"image"`

	// PostInstallScript (Bash) is used for configuring commands that should be executed after installimage.
	// It is passed along with the installimage command.
	PostInstallScript string `json:"postInstallScript,omitempty"`

	// Partitions define the additional Partitions to be created in installimage.
	Partitions []Partition `json:"partitions"`

	// LVMDefinitions defines the logical volume definitions to be created.
	// +optional
	LVMDefinitions []LVMDefinition `json:"logicalVolumeDefinitions,omitempty"`

	// BTRFSDefinitions define the btrfs subvolume definitions to be created.
	// +optional
	BTRFSDefinitions []BTRFSDefinition `json:"btrfsDefinitions,omitempty"`

	// Swraid defines the SWRAID in InstallImage. It enables or disables raids. Set 1 to enable.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Enum=0;1;
	Swraid int `json:"swraid"`

	// SwraidLevel defines the SWRAIDLEVEL in InstallImage. Only relevant if the raid is enabled.
	// Pick one of 0,1,5,6,10. Ignored if Swraid=0.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Enum=0;1;5;6;10;
	SwraidLevel int `json:"swraidLevel,omitempty"`
}

// Image defines the properties for the autosetup config.
type Image struct {
	// URL defines the remote URL for downloading a tar, tar.gz, tar.bz, tar.bz2, tar.xz, tgz, tbz, txz image.
	URL string `json:"url,omitempty"`

	// UseCustomImageURLCommand makes the controller use the command provided by `--baremetal-image-url-command` instead of installimage.
	// Docs: https://syself.com/docs/caph/developers/image-url-command
	// +optional
	UseCustomImageURLCommand bool `json:"useCustomImageURLCommand"`

	// Name defines the archive name after download. This has to be a valid name for Installimage.
	Name string `json:"name,omitempty"`

	// Path is the local path for a preinstalled image from upstream.
	Path string `json:"path,omitempty"`
}

// GetDetails returns the path of the image and whether the image has to be downloaded.
func (image Image) GetDetails() (imagePath string, needsDownload bool, errorMessage string) {
	// If image is set, then the URL is also set and we have to download a remote file
	if image.UseCustomImageURLCommand {
		return "", false, "internal error: image.UseCustomImageURLCommand is active. Method GetDetails() should be used for the traditional way (without image-url-command)."
	}
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
	// Mount defines the mount path for this filesystem.
	// Keyword 'lvm' to use this PART as volume group (VG) for LVM.
	// Identifier 'btrfs.X' to use this PART as volume for
	// btrfs subvolumes. X can be replaced with a unique
	// alphanumeric keyword. NOTE: no support for btrfs multi-device volumes.
	Mount string `json:"mount"`

	// FileSystem can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap
	// or name of the LVM volume group (VG), if this PART is a VG.
	FileSystem string `json:"fileSystem"`

	// Size can use the keyword 'all' to assign all the remaining space of the drive to the last partition.
	// You can use M/G/T for unit specification in MiB/GiB/TiB.
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
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Addresses is a list of addresses assigned to the machine.
	// This field is copied from the infrastructure provider reference.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// Ready is the state of the hetznerbaremetalmachine.
	// +optional
	Ready bool `json:"ready"`

	// Phase represents the current phase of HetznerBareMetalMachineStatus actuation.
	// E.g. Pending, Running, Terminating, Failed, etc.
	// +optional
	Phase clusterv1.MachinePhase `json:"phase,omitempty"`

	// Conditions define the current service state of the HetznerBareMetalMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// HetznerBareMetalMachine is the Schema for the hetznerbaremetalmachines API.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hetznerbaremetalmachines,scope=Namespaced,categories=cluster-api,shortName=hbmm
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this HetznerBareMetalMachine belongs"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".metadata.annotations.infrastructure\\.cluster\\.x-k8s\\.io/HetznerBareMetalHost",description="HetznerBareMetalHost"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this HetznerBareMetalMachine"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="HetznerBareMetalMachine status such as Pending/Provisioning/Running etc"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HetznerBareMetalMachine"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
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
func (hbmm *HetznerBareMetalMachine) GetConditions() clusterv1.Conditions {
	return hbmm.Status.Conditions
}

// SetConditions sets the underlying service state of the HetznerBareMetalMachine to the predescribed clusterv1.Conditions.
func (hbmm *HetznerBareMetalMachine) SetConditions(conditions clusterv1.Conditions) {
	hbmm.Status.Conditions = conditions
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
